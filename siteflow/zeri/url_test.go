package zeri

import "testing"

func TestIsZeriURLRequiresDomainBoundary(t *testing.T) {
	if !IsZeriURL("https://zeri-m.top/comic/1") {
		t.Fatal("expected zeri-m.top to be accepted")
	}
	if IsZeriURL("https://zeri-m.top.evil.example/comic/1") || IsZeriURL("https://notzeri.example/comic/1") {
		t.Fatal("lookalike domain was accepted")
	}
}
