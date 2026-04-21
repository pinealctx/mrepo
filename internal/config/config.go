package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigName = ".repos"
	FormatTOML        = "toml"
	FormatYAML        = "yaml"
)

type Repo struct {
	Path        string `toml:"path" yaml:"path"`
	Remote      string `toml:"remote,omitempty" yaml:"remote,omitempty"`
	Branch      string `toml:"branch,omitempty" yaml:"branch,omitempty"`
	Description string `toml:"description,omitempty" yaml:"description,omitempty"`
}

type Group struct {
	Repos []string `toml:"repos" yaml:"repos"`
}

type Config struct {
	Version int               `toml:"version" yaml:"version"`
	Repos   map[string]*Repo  `toml:"repos" yaml:"repos"`
	Groups  map[string]*Group `toml:"groups" yaml:"groups"`
}

func New() *Config {
	return &Config{
		Version: 1,
		Repos:   make(map[string]*Repo),
		Groups:  make(map[string]*Group),
	}
}

func FindConfigFile(dir string) (string, error) {
	for _, ext := range []string{FormatTOML, "yml", FormatYAML} {
		p := filepath.Join(dir, DefaultConfigName+"."+ext)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no config file found (looked for .repos.toml, .repos.yml, .repos.yaml)")
}

func DetectFormat(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".toml":
		return FormatTOML
	case ".yaml", ".yml":
		return FormatYAML
	default:
		return FormatTOML
	}
}

func ConfigPath(dir, format string) string {
	if format == FormatYAML {
		return filepath.Join(dir, DefaultConfigName+".yaml")
	}
	return filepath.Join(dir, DefaultConfigName+".toml")
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := New()
	format := DetectFormat(path)

	switch format {
	case FormatYAML:
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse yaml: %w", err)
		}
	default:
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse toml: %w", err)
		}
	}

	// Validate repo entries.
	for name, repo := range cfg.Repos {
		if repo.Path == "" {
			return nil, fmt.Errorf("repo %q has empty path", name)
		}
		if strings.HasPrefix(repo.Path, "-") {
			return nil, fmt.Errorf("repo %q has invalid path %q: must not start with '-'", name, repo.Path)
		}
		if repo.Remote != "" && !strings.Contains(repo.Remote, "://") && !strings.Contains(repo.Remote, "@") {
			return nil, fmt.Errorf("repo %q has invalid remote %q: must be a URL or SSH address", name, repo.Remote)
		}
	}

	return cfg, nil
}

func (c *Config) Save(path string) error {
	var data []byte
	var err error

	format := DetectFormat(path)
	switch format {
	case FormatYAML:
		data, err = yaml.Marshal(c)
	default:
		data, err = toml.Marshal(c)
	}
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

func (c *Config) AddRepo(name, repoPath, remote, branch, desc string) error {
	if _, exists := c.Repos[name]; exists {
		return fmt.Errorf("repo %q already exists", name)
	}
	c.Repos[name] = &Repo{
		Path:        filepath.ToSlash(repoPath),
		Remote:      remote,
		Branch:      branch,
		Description: desc,
	}
	return nil
}

func (c *Config) RemoveRepo(name string) (*Repo, error) {
	repo, exists := c.Repos[name]
	if !exists {
		return nil, fmt.Errorf("repo %q not found", name)
	}
	delete(c.Repos, name)

	for gName, group := range c.Groups {
		filtered := make([]string, 0, len(group.Repos))
		for _, r := range group.Repos {
			if r != name {
				filtered = append(filtered, r)
			}
		}
		group.Repos = filtered
		if len(group.Repos) == 0 {
			delete(c.Groups, gName)
		}
	}

	return repo, nil
}

func (c *Config) SortedRepoNames() []string {
	names := make([]string, 0, len(c.Repos))
	for name := range c.Repos {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c *Config) RepoNamesForGroup(groupName string) ([]string, error) {
	group, exists := c.Groups[groupName]
	if !exists {
		return nil, fmt.Errorf("group %q not found", groupName)
	}
	return group.Repos, nil
}

func RepoNameFromPath(p string) string {
	return strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
}

func EnsureConfig(rootDir, format string) (string, error) {
	path, err := FindConfigFile(rootDir)
	if err == nil {
		return path, nil
	}

	path = ConfigPath(rootDir, format)
	cfg := New()
	if err := cfg.Save(path); err != nil {
		return "", err
	}
	return path, nil
}
