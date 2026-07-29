package runtime

import "testing"

func TestRelativizePathPreservesEmptyPath(t *testing.T) {
	if got := RelativizePath(`C:\runtime`, ""); got != "" {
		t.Fatalf("RelativizePath(base, empty) = %q, want empty", got)
	}
}

func TestResolvePathPreservesEmptyPath(t *testing.T) {
	if got := ResolvePath(`C:\runtime`, ""); got != "" {
		t.Fatalf("ResolvePath(base, empty) = %q, want empty", got)
	}
}
