package zeri

import (
	"fmt"
	"net/url"
	"strings"

	"comic_downloader_go_playwright_stealth/browser"
)

// ExecutionResult describes the resolved Zeri summary/reader flow.
type ExecutionResult struct {
	Summary                    SummaryPage  `json:"summary"`
	Reader                     ReaderPage   `json:"reader"`
	PaginationPages            []ReaderPage `json:"paginationPages,omitempty"`
	CollectedImages            []string     `json:"collectedImages,omitempty"`
	ActivationClicks           int          `json:"activationClicks"`
	PaginationActivationClicks int          `json:"paginationActivationClicks,omitempty"`
	FinalURL                   string       `json:"finalURL"`
	FinalTitle                 string       `json:"finalTitle"`
}

// IsZeriURL reports whether the URL belongs to the Zeri family we currently support.
func IsZeriURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return strings.Contains(host, "zeri-m.top") || strings.Contains(host, "zeri")
}

// Execute resolves the summary page, navigates to the reader page, and activates reader zoom.
func Execute(session browser.BrowserPageActions, summaryURL string) (ExecutionResult, error) {
	return ExecuteWithProgress(session, summaryURL, nil)
}

// ExecuteWithProgress resolves the summary page, navigates to the reader page, activates reader zoom,
// and reports overall progress as the flow advances.
func ExecuteWithProgress(session browser.BrowserPageActions, summaryURL string, progress DownloadProgressFunc) (ExecutionResult, error) {
	if session == nil {
		return ExecutionResult{}, fmt.Errorf("browser session is nil")
	}
	report := func(current, total int, fraction float64, phase, message string) {
		if progress == nil {
			return
		}
		progress(DownloadProgress{
			Current:  current,
			Total:    total,
			Fraction: clamp01(fraction),
			Phase:    phase,
			Message:  message,
		})
	}

	report(0, 0, 0.02, "启动", "摘要")
	summaryHTML, err := session.Content()
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read summary html: %w", err)
	}
	summary, err := ParseSummaryPage(summaryURL, summaryHTML)
	if err != nil {
		return ExecutionResult{}, err
	}
	if summary.ReaderURL == "" {
		return ExecutionResult{}, fmt.Errorf("reader url not found in summary page")
	}
	downloadWeight := computeDownloadWeight(summary.PageCount)
	parseWeight := 1.0 - downloadWeight
	if parseWeight < 0.2 {
		parseWeight = 0.2
	}
	report(0, 0, 0.10*parseWeight, "解析", "摘要")
	if err := session.Goto(summary.ReaderURL); err != nil {
		return ExecutionResult{}, fmt.Errorf("goto reader url %q: %w", summary.ReaderURL, err)
	}

	readerHTML, err := session.Content()
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read reader html: %w", err)
	}
	reader, err := ParseReaderPage(summaryURL, summary.ReaderURL, readerHTML)
	if err != nil {
		return ExecutionResult{}, err
	}
	report(0, 0, 0.30*parseWeight, "解析", "正文")

	activationClicks := 0
	if reader.HasZoom100 {
		if err := session.ClickText("100%"); err != nil {
			return ExecutionResult{}, fmt.Errorf("click 100%%: %w", err)
		}
		activationClicks = 1
		activatedHTML, err := session.Content()
		if err == nil {
			if activatedReader, parseErr := ParseReaderPage(summaryURL, summary.ReaderURL, activatedHTML); parseErr == nil {
				reader = activatedReader
			}
		}
	}
	report(0, 0, 0.45*parseWeight, "激活", "100%")

	paginationPages, collectedImages, paginationClicks, err := walkReaderPagination(session, summaryURL, reader, func(current, total int) {
		if progress == nil || total <= 0 {
			return
		}
		spanStart := 0.45 * parseWeight
		span := parseWeight - spanStart
		if span < 0 {
			span = 0
		}
		fraction := spanStart
		if total > 0 {
			fraction = spanStart + span*float64(current)/float64(total)
		}
		report(current, total, fraction, "解析", fmt.Sprintf("%d/%d", current, total))
	})
	if err != nil {
		return ExecutionResult{}, err
	}
	collectedImages = mergeUniqueStrings(append(reader.FilteredImageURLs, collectedImages...))
	report(len(paginationPages), len(paginationPages), parseWeight, "解析", "完成")

	return ExecutionResult{
		Summary:                    summary,
		Reader:                     reader,
		PaginationPages:            paginationPages,
		CollectedImages:            collectedImages,
		ActivationClicks:           activationClicks,
		PaginationActivationClicks: paginationClicks,
		FinalURL:                   reader.URL,
		FinalTitle:                 reader.Title,
	}, nil
}

func walkReaderPagination(session browser.BrowserPageActions, summaryURL string, first ReaderPage, progress func(current, total int)) ([]ReaderPage, []string, int, error) {
	paginationURLs := mergeUniqueStrings(append([]string{first.URL}, first.PaginationURLs...))
	paginationPages := make([]ReaderPage, 0, len(paginationURLs))
	collectedImages := make([]string, 0)
	activationClicks := 0
	total := len(paginationURLs)
	for _, pageURL := range paginationURLs {
		if strings.TrimSpace(pageURL) == "" {
			continue
		}
		if pageURL != first.URL {
			if err := session.Goto(pageURL); err != nil {
				return nil, nil, 0, fmt.Errorf("goto pagination url %q: %w", pageURL, err)
			}
		}
		html, err := session.Content()
		if err != nil {
			return nil, nil, 0, fmt.Errorf("read pagination html %q: %w", pageURL, err)
		}
		page, err := ParseReaderPage(summaryURL, pageURL, html)
		if err != nil {
			return nil, nil, 0, err
		}
		paginationPages = append(paginationPages, page)
		collectedImages = append(collectedImages, page.FilteredImageURLs...)
		if progress != nil {
			progress(len(paginationPages), total)
		}
	}
	return paginationPages, mergeUniqueStrings(collectedImages), activationClicks, nil
}

func computeDownloadWeight(targetCount int) float64 {
	if targetCount <= 0 {
		return 0.35
	}
	weight := 0.25 + float64(targetCount)/float64(targetCount+8)*0.50
	if weight < 0.25 {
		weight = 0.25
	}
	if weight > 0.75 {
		weight = 0.75
	}
	return weight
}

// DownloadWeightForCount returns the progress fraction reserved for the download phase.
func DownloadWeightForCount(targetCount int) float64 {
	return computeDownloadWeight(targetCount)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func mergeUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	merged := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		merged = append(merged, value)
	}
	return merged
}
