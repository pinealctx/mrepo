package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestStatusString(t *testing.T) {
	tests := []struct {
		status WorktreeStatus
		err    error
		want   string
	}{
		{StatusClean, nil, "clean"},
		{StatusDirty, nil, "dirty"},
		{StatusStaged, nil, "staged"},
		{StatusUntracked, nil, "untracked"},
		{StatusConflicted, nil, "conflicted"},
		{StatusMissing, nil, "missing"},
		{StatusClean, fmt.Errorf("boom"), "error"},
	}
	for _, tt := range tests {
		s := &RepoStatus{Worktree: tt.status, Error: tt.err}
		got := s.StatusString()
		if got != tt.want {
			t.Errorf("StatusString() = %q, want %q", got, tt.want)
		}
	}
}

func TestIsClean(t *testing.T) {
	if (&RepoStatus{Worktree: StatusClean}).IsClean() != true {
		t.Error("clean status should be clean")
	}
	if (&RepoStatus{Worktree: StatusDirty}).IsClean() != false {
		t.Error("dirty status should not be clean")
	}
	if (&RepoStatus{Worktree: StatusClean, Error: fmt.Errorf("err")}).IsClean() != false {
		t.Error("clean with error should not be clean")
	}
}

func TestGetStatusMissing(t *testing.T) {
	s := GetStatus(context.Background(), "test", "/nonexistent/path/that/does/not/exist")
	if s.Worktree != StatusMissing {
		t.Errorf("Worktree = %v, want StatusMissing", s.Worktree)
	}
	if s.Error != nil {
		t.Errorf("Error should be nil for missing repo, got %v", s.Error)
	}
}

func TestGetStatusFromRealRepo(t *testing.T) {
	// Use the current project's git repo as a test fixture.
	repoPath, _ := filepath.Abs(".")
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		t.Skip("not in a git repo")
	}

	s := GetStatus(context.Background(), "self", repoPath)
	if s.Error != nil {
		t.Fatalf("unexpected error: %v", s.Error)
	}
	if s.Branch == "" {
		t.Error("Branch should not be empty")
	}
}

func TestGetRepoInfo(t *testing.T) {
	repoPath, _ := filepath.Abs(".")
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		t.Skip("not in a git repo")
	}

	info := GetRepoInfo(context.Background(), repoPath)
	if info.Branch == "" {
		t.Error("Branch should not be empty")
	}
	// Remote may be empty in some test environments, just verify no panic.
}

func TestScanGitRepos(t *testing.T) {
	root := t.TempDir()

	// Create fake git repos.
	for _, name := range []string{"repo-a", "repo-b", "skip-this"} {
		gitDir := filepath.Join(root, name, ".git")
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a nested repo (depth 2).
	gitDir := filepath.Join(root, "sub", "repo-c", ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a node_modules dir with a fake git repo inside (should be skipped).
	gitDir = filepath.Join(root, "node_modules", "pkg", ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	repos, err := ScanGitRepos(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	found := make(map[string]bool)
	for _, r := range repos {
		found[r] = true
	}

	if !found["repo-a"] {
		t.Error("expected repo-a to be found")
	}
	if !found["repo-b"] {
		t.Error("expected repo-b to be found")
	}
	if !found["skip-this"] {
		t.Error("expected skip-this to be found")
	}
	if !found["sub/repo-c"] {
		t.Error("expected sub/repo-c to be found")
	}
	if found["node_modules/pkg"] {
		t.Error("node_modules repos should be skipped")
	}

	if len(repos) != 4 {
		t.Errorf("found %d repos, want 4: %v", len(repos), repos)
	}
}

func TestScanGitReposDepthLimit(t *testing.T) {
	root := t.TempDir()

	// Depth 3: should be excluded.
	gitDir := filepath.Join(root, "a", "b", "c", ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	repos, err := ScanGitRepos(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range repos {
		if r == "a/b/c" {
			t.Error("depth 3 repo should not be found")
		}
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d: %v", len(repos), repos)
	}
}

func TestValidateCloneTarget(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		relPath string
		remote  string
		wantErr bool
	}{
		{"services/backend", "https://github.com/org/repo.git", false},
		{"../../etc", "https://github.com/org/repo.git", true},
		{"-flag", "https://github.com/org/repo.git", true},
		{"ok", "not-a-url", true},
		{"ok", "git@github.com:org/repo.git", false},
	}

	for _, tt := range tests {
		err := validateCloneTarget(dir, tt.relPath, tt.remote)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateCloneTarget(%q, %q) err = %v, wantErr %v", tt.relPath, tt.remote, err, tt.wantErr)
		}
	}
}
