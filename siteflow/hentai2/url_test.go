package hentai2

import "testing"

func TestIsHentai2URLRequiresDomainBoundary(t *testing.T) {
	if !IsHentai2URL("https://www3.hentai2.net/example") {
		t.Fatal("expected hentai2.net subdomain to be accepted")
	}
	if IsHentai2URL("https://hentai2.net.evil.example/example") {
		t.Fatal("lookalike domain was accepted")
	}
}
