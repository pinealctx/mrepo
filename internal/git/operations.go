package git

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

type WorktreeStatus int

const (
	StatusClean WorktreeStatus = iota
	StatusDirty
	StatusStaged
	StatusUntracked
	StatusConflicted
	StatusMissing
)

type RepoStatus struct {
	Name     string
	Path     string
	Branch   string
	Remote   string
	Ahead    int
	Behind   int
	Worktree WorktreeStatus
	Error    error
}

func (s *RepoStatus) IsClean() bool {
	return s.Worktree == StatusClean && s.Error == nil
}

func (s *RepoStatus) StatusString() string {
	if s.Error != nil {
		return "error"
	}
	switch s.Worktree {
	case StatusClean:
		return "clean"
	case StatusDirty:
		return "dirty"
	case StatusStaged:
		return "staged"
	case StatusUntracked:
		return "untracked"
	case StatusConflicted:
		return "conflicted"
	case StatusMissing:
		return "missing"
	default:
		return "unknown"
	}
}

func gitCmd(ctx context.Context, repoPath string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func GetStatus(ctx context.Context, name, repoPath string) *RepoStatus {
	s := &RepoStatus{Name: name, Path: repoPath}

	// Check if directory exists.
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		s.Worktree = StatusMissing
		return s
	}

	branch, err := gitCmd(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		s.Error = fmt.Errorf("get branch: %w", err)
		return s
	}
	s.Branch = branch

	remote, _ := gitCmd(ctx, repoPath, "rev-parse", "--abbrev-ref", "@{upstream}")
	s.Remote = remote

	if s.Remote != "" {
		counts, err := gitCmd(ctx, repoPath, "rev-list", "--left-right", "--count", s.Remote+"...HEAD")
		if err == nil {
			parts := strings.Split(counts, "\t")
			if len(parts) == 2 {
				s.Behind, _ = strconv.Atoi(parts[0])
				s.Ahead, _ = strconv.Atoi(parts[1])
			}
		}
	}

	porcelain, err := gitCmd(ctx, repoPath, "status", "--porcelain=v1")
	if err != nil {
		s.Error = fmt.Errorf("get status: %w", err)
		return s
	}

	if porcelain == "" {
		s.Worktree = StatusClean
		return s
	}

	s.Worktree = StatusClean
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 2 {
			continue
		}
		x := line[0]
		y := line[1]
		switch {
		case x == '?' && y == '?':
			if s.Worktree < StatusUntracked {
				s.Worktree = StatusUntracked
			}
		case x == 'U' || y == 'U':
			s.Worktree = StatusConflicted
			return s
		case y == 'M' || y == 'A' || y == 'D' || y == 'R':
			if s.Worktree < StatusDirty {
				s.Worktree = StatusDirty
			}
		case x == 'M' || x == 'A' || x == 'D' || x == 'R':
			if s.Worktree < StatusStaged {
				s.Worktree = StatusStaged
			}
		}
	}

	return s
}

// RepoInfo holds metadata detected from an existing Git repository on disk.
type RepoInfo struct {
	Remote string
	Branch string
}

// GetRepoInfo detects the remote URL (origin) and current branch of a repo.
func GetRepoInfo(ctx context.Context, repoPath string) RepoInfo {
	var info RepoInfo
	info.Remote, _ = gitCmd(ctx, repoPath, "remote", "get-url", "origin")
	info.Branch, _ = gitCmd(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	return info
}

// Log runs git log and returns oneline output.
func Log(ctx context.Context, repoPath string, max int) (string, error) {
	n := fmt.Sprintf("-%d", max)
	return gitCmd(ctx, repoPath, "log", "--oneline", n)
}

type PullResult struct {
	Name   string
	Path   string
	Output string
	Error  error
}

func Pull(ctx context.Context, name, repoPath string) *PullResult {
	out, err := gitCmd(ctx, repoPath, "pull", "--ff-only")
	r := &PullResult{Name: name, Path: repoPath, Output: out}
	if err != nil {
		r.Error = fmt.Errorf("%s", out)
	}
	return r
}

func Fetch(ctx context.Context, name, repoPath string) *PullResult {
	out, err := gitCmd(ctx, repoPath, "fetch", "--all")
	r := &PullResult{Name: name, Path: repoPath, Output: out}
	if err != nil {
		r.Error = fmt.Errorf("%s", out)
	}
	return r
}

// CloneSpec describes a repo to clone.
type CloneSpec struct {
	Path   string
	Remote string
	Branch string
}

// CloneResult holds the outcome of a single clone operation.
type CloneResult struct {
	Name   string
	Path   string
	Output string
	Error  error
}

// Clone clones a single repo. If the target directory already exists, it skips.
func Clone(ctx context.Context, name string, spec CloneSpec) *CloneResult {
	targetPath := spec.Path
	r := &CloneResult{Name: name, Path: targetPath}

	if _, err := os.Stat(targetPath); err == nil {
		r.Output = "already exists, skipped"
		return r
	}

	args := []string{"clone", spec.Remote, targetPath}
	if spec.Branch != "" {
		args = []string{"clone", "--branch", spec.Branch, spec.Remote, targetPath}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.CombinedOutput()
	r.Output = strings.TrimSpace(string(out))
	if err != nil {
		r.Error = fmt.Errorf("%s", r.Output)
	}
	return r
}

func GetStatuses(ctx context.Context, rootDir string, repos map[string]string, parallel int) []*RepoStatus {
	if parallel <= 0 {
		parallel = 4
	}

	results := make([]*RepoStatus, len(repos))
	var mu sync.Mutex
	idx := 0

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(parallel)

	for name, relPath := range repos {
		name, relPath := name, relPath
		i := idx
		idx++

		eg.Go(func() error {
			absPath := filepath.Join(rootDir, relPath)
			s := GetStatus(egCtx, name, absPath)

			mu.Lock()
			results[i] = s
			mu.Unlock()
			return nil
		})
	}

	_ = eg.Wait()
	return results
}

func PullAll(ctx context.Context, rootDir string, repos map[string]string, parallel int) []*PullResult {
	if parallel <= 0 {
		parallel = 4
	}

	results := make([]*PullResult, len(repos))
	var mu sync.Mutex
	idx := 0

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(parallel)

	for name, relPath := range repos {
		name, relPath := name, relPath
		i := idx
		idx++

		eg.Go(func() error {
			absPath := filepath.Join(rootDir, relPath)
			r := Pull(egCtx, name, absPath)

			mu.Lock()
			results[i] = r
			mu.Unlock()
			return nil
		})
	}

	_ = eg.Wait()
	return results
}

func FetchAll(ctx context.Context, rootDir string, repos map[string]string, parallel int) []*PullResult {
	if parallel <= 0 {
		parallel = 4
	}

	results := make([]*PullResult, len(repos))
	var mu sync.Mutex
	idx := 0

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(parallel)

	for name, relPath := range repos {
		name, relPath := name, relPath
		i := idx
		idx++

		eg.Go(func() error {
			absPath := filepath.Join(rootDir, relPath)
			r := Fetch(egCtx, name, absPath)

			mu.Lock()
			results[i] = r
			mu.Unlock()
			return nil
		})
	}

	_ = eg.Wait()
	return results
}

func CloneAll(ctx context.Context, rootDir string, specs map[string]CloneSpec, parallel int) []*CloneResult {
	if parallel <= 0 {
		parallel = 4
	}

	results := make([]*CloneResult, len(specs))
	var mu sync.Mutex
	idx := 0

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(parallel)

	for name, spec := range specs {
		name, spec := name, spec
		i := idx
		idx++

		eg.Go(func() error {
			absTarget := filepath.Join(rootDir, spec.Path)
			s := CloneSpec{Path: absTarget, Remote: spec.Remote, Branch: spec.Branch}
			r := Clone(egCtx, name, s)

			mu.Lock()
			results[i] = r
			mu.Unlock()
			return nil
		})
	}

	_ = eg.Wait()
	return results
}

func ScanGitRepos(ctx context.Context, rootDir string) ([]string, error) {
	var repos []string

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Skip the root directory itself.
		if path == rootDir {
			return nil
		}

		// Skip common non-project directories.
		base := filepath.Base(path)
		if d.IsDir() && (base == ".git" || base == "node_modules" || base == "vendor" || base == ".idea" || base == ".vscode") {
			return filepath.SkipDir
		}

		// Found a Git repo.
		if d.IsDir() && base == ".git" {
			repoPath := filepath.Dir(path)
			relPath, relErr := filepath.Rel(rootDir, repoPath)
			if relErr != nil || relPath == "." {
				return nil
			}
			repos = append(repos, filepath.ToSlash(relPath))
			return filepath.SkipDir
		}

		// Limit depth to 2 levels.
		rel, _ := filepath.Rel(rootDir, path)
		if rel != "" && strings.Count(rel, string(filepath.Separator)) >= 2 {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan for git repos: %w", err)
	}

	return repos, nil
}
