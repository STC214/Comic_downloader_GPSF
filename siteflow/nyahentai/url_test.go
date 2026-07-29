package nyahentai

import "testing"

func TestIsNyahentaiURLRequiresDomainBoundary(t *testing.T) {
	if !IsNyahentaiURL("https://nyahentai.one/g/1") {
		t.Fatal("expected nyahentai.one to be accepted")
	}
	if IsNyahentaiURL("https://nyahentai.one.evil.example/g/1") {
		t.Fatal("lookalike domain was accepted")
	}
}
