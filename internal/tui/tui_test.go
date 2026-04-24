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

func TestExpandTabs(t *testing.T) {
	got := expandTabs("\tfoo\tbar")
	want := "    foo    bar"
	if got != want {
		t.Fatalf("expandTabs() = %q, want %q", got, want)
	}
}

func TestEnterOnFileMovesFocusToDiff(t *testing.T) {
	m := model{
		focus:    focusFiles,
		selected: "repo-a",
		repos:    map[string]string{"repo-a": "repo-a"},
		fileTree: []fileTreeNode{
			{Path: "cmd", IsDir: true},
			{Path: "cmd/main.go", Status: "M"},
		},
		fileCursor: 1,
	}

	updated, cmd := m.updateFilesNav("enter")
	if cmd == nil {
		t.Fatal("enter on file should load diff")
	}
	got := updated.(model)
	if got.focus != focusDiff {
		t.Fatalf("focus = %v, want %v", got.focus, focusDiff)
	}
}

func TestEnterOnRepoMovesFocusToBranches(t *testing.T) {
	m := model{
		focus: focusRepos,
		items: []string{"repo-a"},
	}

	updated, cmd := m.updateReposNav("enter")
	if cmd != nil {
		t.Fatal("enter on repo should not schedule a command")
	}
	got := updated.(model)
	if got.focus != focusBranches {
		t.Fatalf("focus = %v, want %v", got.focus, focusBranches)
	}
}

func TestLeftFromDiffMovesFocusToFiles(t *testing.T) {
	m := model{focus: focusDiff}

	got := m.moveFocusLeft().(model)
	if got.focus != focusFiles {
		t.Fatalf("focus = %v, want %v", got.focus, focusFiles)
	}
}

func TestRightMovesFocusAcrossPanels(t *testing.T) {
	m := model{focus: focusRepos, diffContent: &git.FileDiff{Path: "a", Content: "b"}}

	got := m.moveFocusRight().(model)
	if got.focus != focusBranches {
		t.Fatalf("focus = %v, want %v", got.focus, focusBranches)
	}

	got = got.moveFocusRight().(model)
	if got.focus != focusFiles {
		t.Fatalf("focus = %v, want %v", got.focus, focusFiles)
	}

	got = got.moveFocusRight().(model)
	if got.focus != focusDiff {
		t.Fatalf("focus = %v, want %v", got.focus, focusDiff)
	}
}

func TestDiffHorizontalScroll(t *testing.T) {
	m := model{focus: focusDiff}

	updated, cmd := m.updateDiffNav("right")
	if cmd != nil {
		t.Fatal("diff horizontal scroll should not schedule a command")
	}
	got := updated.(model)
	if got.diffXOffset != 4 {
		t.Fatalf("diffXOffset = %d, want 4", got.diffXOffset)
	}

	updated, _ = got.updateDiffNav("left")
	got = updated.(model)
	if got.diffXOffset != 0 {
		t.Fatalf("diffXOffset = %d, want 0", got.diffXOffset)
	}
}

func TestEscFromDiffMovesFocusToFiles(t *testing.T) {
	m := model{focus: focusDiff}

	updated, cmd := m.updateDiffNav("esc")
	if cmd != nil {
		t.Fatal("esc from diff should not schedule a command")
	}
	got := updated.(model)
	if got.focus != focusFiles {
		t.Fatalf("focus = %v, want %v", got.focus, focusFiles)
	}
}

func TestCompactVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "release version unchanged",
			in:   "v0.1.0",
			want: "v0.1.0",
		},
		{
			name: "pseudo version shortened",
			in:   "v0.0.3-0.20260424054146-e2ad6f288055",
			want: "v0.0.3-e2ad6f2",
		},
		{
			name: "pseudo version keeps metadata",
			in:   "v0.0.3-0.20260424054146-e2ad6f288055+dirty",
			want: "v0.0.3-e2ad6f2+dirty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compactVersion(tt.in); got != tt.want {
				t.Fatalf("compactVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
