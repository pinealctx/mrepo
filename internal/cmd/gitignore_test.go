package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignore(t *testing.T) {
	root := t.TempDir()

	// Create .git dir so it's detected as a git repo.
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Add a repo path.
	if err := ensureGitignore(root, "services/backend"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "services/backend/") {
		t.Errorf("expected .gitignore to contain 'services/backend/', got:\n%s", content)
	}

	// Add again — should be idempotent.
	if err := ensureGitignore(root, "services/backend"); err != nil {
		t.Fatal(err)
	}

	data2, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	count := strings.Count(string(data2), "services/backend/")
	if count != 1 {
		t.Errorf("expected 1 occurrence, got %d", count)
	}

	// Add another repo.
	if err := ensureGitignore(root, "web/app"); err != nil {
		t.Fatal(err)
	}

	data3, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.Contains(string(data3), "web/app/") {
		t.Error("expected .gitignore to contain 'web/app/'")
	}
	// Should still have mrepo header only once.
	if strings.Count(string(data3), "mrepo") != 1 {
		t.Error("mrepo header should appear only once")
	}
}

func TestEnsureGitignoreSkipsRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// "." should be skipped.
	if err := ensureGitignore(root, "."); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, ".gitignore")); err == nil {
		t.Error(".gitignore should not be created for root repo itself")
	}
}

func TestEnsureGitignoreNoGitDir(t *testing.T) {
	root := t.TempDir()

	// No .git dir — should be a no-op.
	if err := ensureGitignore(root, "some-repo"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, ".gitignore")); err == nil {
		t.Error(".gitignore should not be created without .git dir")
	}
}

func TestRemoveFromGitignore(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Setup: add two repos.
	if err := ensureGitignore(root, "services/backend"); err != nil {
		t.Fatal(err)
	}
	if err := ensureGitignore(root, "web/app"); err != nil {
		t.Fatal(err)
	}

	// Remove one.
	if err := removeFromGitignore(root, "services/backend"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	content := string(data)

	if strings.Contains(content, "services/backend/") {
		t.Errorf("services/backend/ should be removed, got:\n%s", content)
	}
	if !strings.Contains(content, "web/app/") {
		t.Errorf("web/app/ should still be present, got:\n%s", content)
	}
}
