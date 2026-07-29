package hitomi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"comic_downloader_go_playwright_stealth/siteflow/assets"
	"comic_downloader_go_playwright_stealth/siteflow/zeri"
)

func TestGalleryIDFromURL(t *testing.T) {
	id, ok := GalleryIDFromURL("https://hitomi.la/doujinshi/example-3929784.html")
	if !ok || id != 3929784 {
		t.Fatalf("GalleryIDFromURL() = %d, %t", id, ok)
	}
}

func TestLiveMetadataWhenConfigured(t *testing.T) {
	rawURL := strings.TrimSpace(os.Getenv("COMIC_HITOMI_TEST_URL"))
	if rawURL == "" {
		t.Skip("COMIC_HITOMI_TEST_URL is not set")
	}
	result, err := ExecuteWithProgress(context.Background(), rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageCount == 0 || len(result.CollectedImages) != result.PageCount {
		t.Fatalf("invalid live result: %#v", result)
	}
	download, err := assets.DownloadImagesContext(context.Background(), assets.CollectionSummary{
		Site: "hitomi", BaseURL: result.FinalURL, Title: "live-thumbnail-test",
	}, result.CollectedImages[:1], t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(download.Files) != 1 || download.Bytes == 0 {
		t.Fatalf("invalid live download: %#v", download)
	}
	if err := zeri.CreateJPGThumbnail(download.Files[0], t.TempDir()+"/thumb.jpg", 256); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteWithProgress(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/galleries/3929784.js", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `var galleryinfo = {"id":"3929784","title":"Example","files":[{"name":"1.jpg","hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]};`)
	})
	mux.HandleFunc("/gg.js", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `var o = 0; function m(g) { o = 0; break; } var cfg = {b: ''};`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient()
	client.APIBaseURL = server.URL
	client.HTTPClient = server.Client()
	info, err := client.GetGalleryInfo(context.Background(), 3929784)
	if err != nil {
		t.Fatal(err)
	}
	imageURL, err := client.ImageURL(context.Background(), info.Files[0].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(imageURL, ".gold-usergeneratedcontent.net/") || !strings.HasSuffix(imageURL, ".webp") {
		t.Fatalf("unexpected image URL %q", imageURL)
	}
}
