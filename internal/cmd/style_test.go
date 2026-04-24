package cmd

import "testing"

func TestDisplayRepoNameRoot(t *testing.T) {
	prevRoot := rootDir
	t.Cleanup(func() {
		rootDir = prevRoot
	})

	rootDir = "E:/Projects/github/pinealctx/mrepo"

	if got := displayRepoName("."); got != "<root>" {
		t.Fatalf("displayRepoName(.) = %q, want %q", got, "<root>")
	}
	if got := displayRepoName(""); got != "<root>" {
		t.Fatalf("displayRepoName(\"\") = %q, want %q", got, "<root>")
	}
}
