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
	if err := cfg.AddRepo("backend", "services/backend", "https://github.com/org/backend.git", "main", "Go server"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddRepo("frontend", "web/frontend", "git@github.com:org/frontend.git", "dev", ""); err != nil {
		t.Fatal(err)
	}

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
	if err := cfg.AddRepo("a", "path-a", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddRepo("b", "path-b", "", "", ""); err != nil {
		t.Fatal(err)
	}

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
	if err := cfg.AddRepo("a", "path-a", "", "", ""); err != nil {
		t.Fatal(err)
	}
	cfg.Groups = map[string]*Group{
		"only-a": {Repos: []string{"a"}},
	}

	if _, err := cfg.RemoveRepo("a"); err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(tomlPath, []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := FindConfigFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if found != tomlPath {
		t.Errorf("FindConfigFile = %q, want %q", found, tomlPath)
	}

	// TOML takes priority over YAML.
	yamlPath := filepath.Join(dir, ".repos.yaml")
	if err := os.WriteFile(yamlPath, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

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
	if err := cfg.AddRepo("charlie", "c", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddRepo("alpha", "a", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddRepo("bravo", "b", "", "", ""); err != nil {
		t.Fatal(err)
	}

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

func TestSortedRepoNamesRootFirst(t *testing.T) {
	cfg := New()
	if err := cfg.AddRepo("bravo", "b", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddRepo(".", ".", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddRepo("alpha", "a", "", "", ""); err != nil {
		t.Fatal(err)
	}

	names := cfg.SortedRepoNames()
	if names[0] != "." {
		t.Errorf("first name = %q, want %q", names[0], ".")
	}
}

func TestAddGroup(t *testing.T) {
	cfg := New()
	if err := cfg.AddGroup("services"); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Groups["services"]; !ok {
		t.Error("group not created")
	}
	if err := cfg.AddGroup("services"); err == nil {
		t.Error("expected error for duplicate group")
	}
}

func TestDeleteGroup(t *testing.T) {
	cfg := New()
	if err := cfg.AddGroup("services"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.DeleteGroup("services"); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Groups["services"]; ok {
		t.Error("group should be deleted")
	}
	if err := cfg.DeleteGroup("services"); err == nil {
		t.Error("expected error for missing group")
	}
}

func TestAddRepoToGroup(t *testing.T) {
	cfg := New()
	if err := cfg.AddRepo("backend", "services/backend", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddGroup("services"); err != nil {
		t.Fatal(err)
	}

	if err := cfg.AddRepoToGroup("services", "backend"); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Groups["services"].Repos) != 1 || cfg.Groups["services"].Repos[0] != "backend" {
		t.Errorf("group repos = %v, want [backend]", cfg.Groups["services"].Repos)
	}

	// Duplicate.
	if err := cfg.AddRepoToGroup("services", "backend"); err == nil {
		t.Error("expected error for duplicate repo in group")
	}

	// Nonexistent repo.
	if err := cfg.AddRepoToGroup("services", "missing"); err == nil {
		t.Error("expected error for nonexistent repo")
	}

	// Nonexistent group.
	if err := cfg.AddRepoToGroup("missing", "backend"); err == nil {
		t.Error("expected error for nonexistent group")
	}
}

func TestRemoveRepoFromGroup(t *testing.T) {
	cfg := New()
	if err := cfg.AddRepo("a", "path-a", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddRepo("b", "path-b", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddGroup("all"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddRepoToGroup("all", "a"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddRepoToGroup("all", "b"); err != nil {
		t.Fatal(err)
	}

	if err := cfg.RemoveRepoFromGroup("all", "a"); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Groups["all"].Repos) != 1 || cfg.Groups["all"].Repos[0] != "b" {
		t.Errorf("group repos = %v, want [b]", cfg.Groups["all"].Repos)
	}

	// Not in group.
	if err := cfg.RemoveRepoFromGroup("all", "a"); err == nil {
		t.Error("expected error for repo not in group")
	}
}

func TestRemoveRepoFromGroupDeletesEmpty(t *testing.T) {
	cfg := New()
	if err := cfg.AddRepo("a", "path-a", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddGroup("only-a"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddRepoToGroup("only-a", "a"); err != nil {
		t.Fatal(err)
	}

	if err := cfg.RemoveRepoFromGroup("only-a", "a"); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Groups["only-a"]; ok {
		t.Error("empty group should be deleted")
	}
}

func TestSortedGroupNames(t *testing.T) {
	cfg := New()
	if err := cfg.AddGroup("charlie"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddGroup("alpha"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AddGroup("bravo"); err != nil {
		t.Fatal(err)
	}

	names := cfg.SortedGroupNames()
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
