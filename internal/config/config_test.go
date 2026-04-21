package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoNameFromPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"services/backend", "backend"},
		{"nexus-ai", "nexus-ai"},
		{"services/backend.git", "backend"},
		{"my.project", "my"},
		{"a/b/c/repo", "repo"},
	}
	for _, tt := range tests {
		got := RepoNameFromPath(tt.input)
		if got != tt.want {
			t.Errorf("RepoNameFromPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{".repos.toml", FormatTOML},
		{".repos.yaml", FormatYAML},
		{".repos.yml", FormatYAML},
		{".repos.json", FormatTOML}, // default
	}
	for _, tt := range tests {
		got := DetectFormat(tt.path)
		if got != tt.want {
			t.Errorf("DetectFormat(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestConfigRoundTrip(t *testing.T) {
	cfg := New()
	cfg.AddRepo("backend", "services/backend", "https://github.com/org/backend.git", "main", "Go server")
	cfg.AddRepo("frontend", "web/frontend", "git@github.com:org/frontend.git", "dev", "")

	dir := t.TempDir()

	t.Run("toml", func(t *testing.T) {
		path := filepath.Join(dir, ".repos.toml")
		if err := cfg.Save(path); err != nil {
			t.Fatalf("Save: %v", err)
		}
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		assertConfigEqual(t, cfg, loaded)
	})

	t.Run("yaml", func(t *testing.T) {
		path := filepath.Join(dir, ".repos.yaml")
		if err := cfg.Save(path); err != nil {
			t.Fatalf("Save: %v", err)
		}
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		assertConfigEqual(t, cfg, loaded)
	})
}

func assertConfigEqual(t *testing.T, a, b *Config) {
	t.Helper()
	if len(a.Repos) != len(b.Repos) {
		t.Fatalf("repo count: %d vs %d", len(a.Repos), len(b.Repos))
	}
	for name, ra := range a.Repos {
		rb, ok := b.Repos[name]
		if !ok {
			t.Errorf("repo %q missing in loaded config", name)
			continue
		}
		if ra.Path != rb.Path {
			t.Errorf("repo %q path: %q vs %q", name, ra.Path, rb.Path)
		}
		if ra.Remote != rb.Remote {
			t.Errorf("repo %q remote: %q vs %q", name, ra.Remote, rb.Remote)
		}
		if ra.Branch != rb.Branch {
			t.Errorf("repo %q branch: %q vs %q", name, ra.Branch, rb.Branch)
		}
	}
}

func TestAddRepoDuplicate(t *testing.T) {
	cfg := New()
	if err := cfg.AddRepo("test", "path", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddRepo("test", "other", "", "", ""); err == nil {
		t.Fatal("expected error for duplicate repo")
	}
}

func TestRemoveRepo(t *testing.T) {
	cfg := New()
	cfg.AddRepo("a", "path-a", "", "", "")
	cfg.AddRepo("b", "path-b", "", "", "")

	// Add group referencing repo a.
	cfg.Groups = map[string]*Group{
		"all": {Repos: []string{"a", "b"}},
	}

	removed, err := cfg.RemoveRepo("a")
	if err != nil {
		t.Fatal(err)
	}
	if removed.Path != "path-a" {
		t.Errorf("removed path = %q, want %q", removed.Path, "path-a")
	}
	if _, exists := cfg.Repos["a"]; exists {
		t.Error("repo a should be removed from Repos")
	}
	// Group should have a removed.
	if len(cfg.Groups["all"].Repos) != 1 || cfg.Groups["all"].Repos[0] != "b" {
		t.Errorf("group all repos = %v, want [b]", cfg.Groups["all"].Repos)
	}
}

func TestRemoveRepoCleansEmptyGroup(t *testing.T) {
	cfg := New()
	cfg.AddRepo("a", "path-a", "", "", "")
	cfg.Groups = map[string]*Group{
		"only-a": {Repos: []string{"a"}},
	}

	cfg.RemoveRepo("a")
	if _, exists := cfg.Groups["only-a"]; exists {
		t.Error("empty group should be deleted")
	}
}

func TestFindConfigFile(t *testing.T) {
	dir := t.TempDir()

	// No config file.
	_, err := FindConfigFile(dir)
	if err == nil {
		t.Fatal("expected error when no config file exists")
	}

	// Create TOML config.
	tomlPath := filepath.Join(dir, ".repos.toml")
	os.WriteFile(tomlPath, []byte("version = 1\n"), 0o644)

	found, err := FindConfigFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if found != tomlPath {
		t.Errorf("FindConfigFile = %q, want %q", found, tomlPath)
	}

	// TOML takes priority over YAML.
	yamlPath := filepath.Join(dir, ".repos.yaml")
	os.WriteFile(yamlPath, []byte("version: 1\n"), 0o644)

	found, err = FindConfigFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if found != tomlPath {
		t.Errorf("TOML should take priority, got %q", found)
	}
}

func TestSortedRepoNames(t *testing.T) {
	cfg := New()
	cfg.AddRepo("charlie", "c", "", "", "")
	cfg.AddRepo("alpha", "a", "", "", "")
	cfg.AddRepo("bravo", "b", "", "", "")

	names := cfg.SortedRepoNames()
	want := []string{"alpha", "bravo", "charlie"}
	if len(names) != len(want) {
		t.Fatalf("got %d names, want %d", len(names), len(want))
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, want[i])
		}
	}
}
