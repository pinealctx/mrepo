package git

import (
	"context"
	"fmt"
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
)

type RepoStatus struct {
	Name       string
	Path       string
	Branch     string
	Remote     string
	Ahead      int
	Behind     int
	Worktree   WorktreeStatus
	Error      error
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

func ScanGitRepos(ctx context.Context, rootDir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "find", rootDir, "-maxdepth", "2", "-name", ".git", "-type", "d")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("scan for git repos: %w", err)
	}

	var repos []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		repoPath := filepath.Dir(line)
		relPath, err := filepath.Rel(rootDir, repoPath)
		if err != nil {
			continue
		}
		if relPath == "." {
			continue
		}
		repos = append(repos, filepath.ToSlash(relPath))
	}

	return repos, nil
}
