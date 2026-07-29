package hentaiaz

import "testing"

func TestIsHentaiazURLRequiresDomainBoundary(t *testing.T) {
	if !IsHentaiazURL("https://x.hentaiaz.com/example") {
		t.Fatal("expected hentaiaz.com subdomain to be accepted")
	}
	if IsHentaiazURL("https://hentaiaz.com.evil.example/example") {
		t.Fatal("lookalike domain was accepted")
	}
}
