package tui

import (
	"strings"
	"testing"

	"github.com/pinealctx/mrepo/internal/git"
)

func TestSummarizeResultsCountsFetchFailures(t *testing.T) {
	results := map[string]string{
		"repo-a": "up to date",
		"repo-b": "FAIL: fetch failed",
	}

	got := summarizeResults("fetch", results)
	want := "fetch: 1 ok, 1 failed"
	if got != want {
		t.Fatalf("summarizeResults() = %q, want %q", got, want)
	}
}

func TestModelHandlesFetchMsg(t *testing.T) {
	m := model{
		operating: true,
		loading:   true,
		details:   map[string]*git.RepoStatus{},
	}

	msg := fetchMsg{
		results: map[string]string{"repo-a": "FAIL: fetch failed"},
		details: map[string]*git.RepoStatus{
			"repo-a": {Name: "repo-a", Worktree: git.StatusClean},
		},
	}

	updated, cmd := m.Update(msg)
	if cmd != nil {
		t.Fatal("fetchMsg should not schedule a follow-up command")
	}

	got, ok := updated.(model)
	if !ok {
		t.Fatalf("expected updated model, got %T", updated)
	}
	if got.operating {
		t.Fatal("expected operating=false after fetch result")
	}
	if got.loading {
		t.Fatal("expected loading=false after fetch result")
	}
	if got.statusText != "fetch: 0 ok, 1 failed" {
		t.Fatalf("unexpected status text: %q", got.statusText)
	}
	if _, ok := got.details["repo-a"]; !ok {
		t.Fatal("expected refreshed repo details to be applied")
	}
}

func TestDisplayNameRoot(t *testing.T) {
	m := model{
		repos: map[string]string{
			".": ".",
		},
	}

	if got := m.displayName("."); got != "<root>" {
		t.Fatalf("displayName(.) = %q, want %q", got, "<root>")
	}
}

func TestRenderDiffPanelDoesNotWrapLongLines(t *testing.T) {
	m := model{
		diffContent: &git.FileDiff{
			Path:    "very/long/path.go",
			Content: strings.Repeat("x", 200),
		},
	}

	rendered := m.renderDiffPanel(5, 20)
	if lines := strings.Split(rendered, "\n"); len(lines) != 2 {
		t.Fatalf("rendered %d lines, want 2", len(lines))
	}
}
