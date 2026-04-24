package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pinealctx/mrepo/internal/config"
)

func TestAddScannedReposRejectsDuplicateRepoName(t *testing.T) {
	prevRoot := rootDir
	t.Cleanup(func() {
		rootDir = prevRoot
	})

	rootDir = t.TempDir()
	cfg := config.New()
	if err := cfg.AddRepo("api", "services/api", "", "", ""); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(rootDir, ".repos.toml")
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(rootDir, "libs", "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = filepath.Join(rootDir, "libs", "api")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}
	cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/org/libs-api.git")
	cmd.Dir = filepath.Join(rootDir, "libs", "api")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add failed: %v: %s", err, out)
	}

	err := addScannedRepos(context.Background(), cfgPath, cfg, []string{"libs/api"})
	if err == nil {
		t.Fatal("expected duplicate repo name error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate name error, got %v", err)
	}
	if len(cfg.Repos) != 1 {
		t.Fatalf("expected config to keep original repo only, got %d repos", len(cfg.Repos))
	}

	loaded, loadErr := config.Load(cfgPath)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(loaded.Repos) != 1 {
		t.Fatalf("expected saved config to keep original repo only, got %d repos", len(loaded.Repos))
	}
	if _, ok := loaded.Repos["api"]; !ok {
		t.Fatal("expected original repo to remain in saved config")
	}
}

func TestAddScannedReposRejectsEscapingPath(t *testing.T) {
	prevRoot := rootDir
	t.Cleanup(func() {
		rootDir = prevRoot
	})

	rootDir = t.TempDir()
	cfg := config.New()
	cfgPath := filepath.Join(rootDir, ".repos.toml")

	err := addScannedRepos(context.Background(), cfgPath, cfg, []string{"../escape"})
	if err == nil {
		t.Fatal("expected path validation error")
	}
	if !strings.Contains(err.Error(), "escapes root directory") {
		t.Fatalf("expected path traversal error, got %v", err)
	}
}
