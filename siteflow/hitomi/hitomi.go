package hitomi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"comic_downloader_go_playwright_stealth/siteflow/zeri"
)

const (
	defaultAPIBaseURL = "https://ltn.gold-usergeneratedcontent.net"
	defaultCDNDomain  = "gold-usergeneratedcontent.net"
	maxTextBytes      = int64(8 << 20)
)

var (
	galleryPathIDRe = regexp.MustCompile(`(?i)/(?:galleries|reader)/(\d+)(?:\.html)?`)
	trailingIDRe    = regexp.MustCompile(`(?i)[-/](\d{4,})(?:\.html)?/?$`)
	anyIDRe         = regexp.MustCompile(`\d{4,}`)
	hashPathRe      = regexp.MustCompile(`/[0-9a-f]{61}([0-9a-f]{2})([0-9a-f])`)
)

type GalleryFile struct {
	Width   int    `json:"width"`
	Hash    string `json:"hash"`
	HasWebP int    `json:"haswebp"`
	Name    string `json:"name"`
	Height  int    `json:"height"`
}

type GalleryInfo struct {
	ID            int           `json:"id"`
	Title         string        `json:"title"`
	JapaneseTitle string        `json:"japanese_title"`
	Files         []GalleryFile `json:"files"`
}

func (g *GalleryInfo) UnmarshalJSON(data []byte) error {
	type alias GalleryInfo
	var raw struct {
		alias
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*g = GalleryInfo(raw.alias)
	if len(raw.ID) == 0 || string(raw.ID) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw.ID, &g.ID); err == nil {
		return nil
	}
	var text string
	if err := json.Unmarshal(raw.ID, &text); err != nil {
		return err
	}
	id, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return fmt.Errorf("parse gallery id: %w", err)
	}
	g.ID = id
	return nil
}

type ExecutionResult struct {
	FinalURL        string
	FinalTitle      string
	PageCount       int
	CollectedImages []string
}

type Client struct {
	HTTPClient *http.Client
	APIBaseURL string

	mu          sync.Mutex
	lastGGFetch time.Time
	mDefault    int
	mMap        map[int]int
	bPrefix     string
}

func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
		APIBaseURL: defaultAPIBaseURL,
		mMap:       map[int]int{},
	}
}

func IsHitomiURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "hitomi.la" || strings.HasSuffix(host, ".hitomi.la")
}

func GalleryIDFromURL(raw string) (int, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	target := raw
	if err == nil {
		target = parsed.Path
	}
	for _, re := range []*regexp.Regexp{galleryPathIDRe, trailingIDRe} {
		if match := re.FindStringSubmatch(target); len(match) > 1 {
			id, err := strconv.Atoi(match[1])
			return id, err == nil && id > 0
		}
	}
	if match := anyIDRe.FindString(target); match != "" {
		id, err := strconv.Atoi(match)
		return id, err == nil && id > 0
	}
	return 0, false
}

func ExecuteWithProgress(ctx context.Context, rawURL string, progress zeri.DownloadProgressFunc) (ExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if progress != nil {
		progress(zeri.DownloadProgress{Fraction: 0.05, Phase: "解析", Message: "读取图库信息"})
	}
	client := NewClient()
	id, ok := GalleryIDFromURL(rawURL)
	if !ok {
		return ExecutionResult{}, fmt.Errorf("hitomi gallery id not found in url %q", rawURL)
	}
	info, err := client.GetGalleryInfo(ctx, id)
	if err != nil {
		return ExecutionResult{}, err
	}
	imageURLs := make([]string, 0, len(info.Files))
	for _, file := range info.Files {
		imageURL, err := client.ImageURL(ctx, file.Hash)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("build hitomi image url %q: %w", file.Name, err)
		}
		imageURLs = append(imageURLs, imageURL)
	}
	if len(imageURLs) == 0 {
		return ExecutionResult{}, fmt.Errorf("hitomi gallery %d has no images", id)
	}
	title := strings.TrimSpace(info.Title)
	if title == "" {
		title = strings.TrimSpace(info.JapaneseTitle)
	}
	if title == "" {
		title = fmt.Sprintf("hitomi-%d", id)
	}
	if progress != nil {
		progress(zeri.DownloadProgress{
			Current: len(imageURLs), Total: len(imageURLs), Fraction: 1,
			Phase: "解析", Message: "完成",
		})
	}
	return ExecutionResult{
		FinalURL:        fmt.Sprintf("https://hitomi.la/galleries/%d.html", id),
		FinalTitle:      title,
		PageCount:       len(imageURLs),
		CollectedImages: imageURLs,
	}, nil
}

func (c *Client) GetGalleryInfo(ctx context.Context, id int) (GalleryInfo, error) {
	if id <= 0 {
		return GalleryInfo{}, fmt.Errorf("hitomi gallery id is empty")
	}
	body, err := c.getText(ctx, fmt.Sprintf("%s/galleries/%d.js", c.apiBaseURL(), id))
	if err != nil {
		return GalleryInfo{}, fmt.Errorf("get hitomi galleryinfo %d: %w", id, err)
	}
	text := strings.TrimSpace(body)
	text = strings.TrimPrefix(text, "var galleryinfo = ")
	text = strings.TrimSpace(strings.TrimSuffix(text, ";"))
	var info GalleryInfo
	if err := json.Unmarshal([]byte(text), &info); err != nil {
		return GalleryInfo{}, fmt.Errorf("parse hitomi galleryinfo %d: %w", id, err)
	}
	if len(info.Files) == 0 {
		return GalleryInfo{}, fmt.Errorf("hitomi gallery %d has no files", id)
	}
	return info, nil
}

func (c *Client) ImageURL(ctx context.Context, hash string) (string, error) {
	hash = strings.TrimSpace(hash)
	if len(hash) < 3 {
		return "", fmt.Errorf("invalid image hash")
	}
	if err := c.refreshGG(ctx); err != nil {
		return "", err
	}
	suffix := hash[len(hash)-3:]
	shard, err := strconv.ParseInt(suffix[2:]+suffix[:2], 16, 64)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	path := c.bPrefix + strconv.FormatInt(shard, 10) + "/" + hash
	c.mu.Unlock()
	raw := "https://a." + defaultCDNDomain + "/" + path + ".webp"
	match := hashPathRe.FindStringSubmatch(raw)
	if len(match) < 3 {
		return raw, nil
	}
	g, err := strconv.ParseInt(match[2]+match[1], 16, 32)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	m, ok := c.mMap[int(g)]
	if !ok {
		m = c.mDefault
	}
	c.mu.Unlock()
	return regexp.MustCompile(`//..?\.`+regexp.QuoteMeta(defaultCDNDomain)+`/`).
		ReplaceAllString(raw, "//w"+strconv.Itoa(1+m)+"."+defaultCDNDomain+"/"), nil
}

func (c *Client) refreshGG(ctx context.Context) error {
	c.mu.Lock()
	if !c.lastGGFetch.IsZero() && time.Since(c.lastGGFetch) < time.Minute {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	body, err := c.getText(ctx, c.apiBaseURL()+"/gg.js")
	if err != nil {
		return fmt.Errorf("get hitomi gg.js: %w", err)
	}
	mDefault := firstInt(regexp.MustCompile(`var o = (\d+)`), body)
	o := firstInt(regexp.MustCompile(`o = (\d+); break;`), body)
	mMap := map[int]int{}
	for _, match := range regexp.MustCompile(`case (\d+):`).FindAllStringSubmatch(body, -1) {
		if value, err := strconv.Atoi(match[1]); err == nil {
			mMap[value] = o
		}
	}
	bPrefix := ""
	if match := regexp.MustCompile(`b: '([^']*)'`).FindStringSubmatch(body); len(match) > 1 {
		bPrefix = match[1]
	}
	c.mu.Lock()
	c.mDefault, c.mMap, c.bPrefix, c.lastGGFetch = mDefault, mMap, bPrefix, time.Now()
	c.mu.Unlock()
	return nil
}

func firstInt(re *regexp.Regexp, text string) int {
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0
	}
	value, _ := strconv.Atoi(match[1])
	return value
}

func (c *Client) getText(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://hitomi.la/")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %s", resp.Status)
	}
	limited := &io.LimitedReader{R: resp.Body, N: maxTextBytes + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if int64(len(data)) > maxTextBytes {
		return "", fmt.Errorf("response too large")
	}
	return string(data), nil
}

func (c *Client) apiBaseURL() string {
	if strings.TrimSpace(c.APIBaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	}
	return defaultAPIBaseURL
}
