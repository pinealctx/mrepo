package git

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

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
		case x == 'U' || y == 'U':
			s.Worktree = StatusConflicted
			return s
		case x == '?' && y == '?':
			if s.Worktree < StatusUntracked {
				s.Worktree = StatusUntracked
			}
		case x == 'M' || x == 'A' || x == 'D' || x == 'R':
			if s.Worktree < StatusStaged {
				s.Worktree = StatusStaged
			}
		case y == 'M' || y == 'A' || y == 'D' || y == 'R':
			if s.Worktree < StatusDirty {
				s.Worktree = StatusDirty
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

// OperationResult holds the outcome of a single pull or fetch operation.
type OperationResult struct {
	Name   string
	Path   string
	Output string
	Error  error
}

func Pull(ctx context.Context, name, repoPath string) *OperationResult {
	out, err := gitCmd(ctx, repoPath, "pull", "--ff-only")
	r := &OperationResult{Name: name, Path: repoPath, Output: out}
	if err != nil {
		r.Error = fmt.Errorf("pull: %s", out)
	}
	return r
}

func Fetch(ctx context.Context, name, repoPath string) *OperationResult {
	out, err := gitCmd(ctx, repoPath, "fetch", "--all")
	r := &OperationResult{Name: name, Path: repoPath, Output: out}
	if err != nil {
		r.Error = fmt.Errorf("fetch: %s", out)
	}
	return r
}

// CloneSpec describes a repo to clone.
type CloneSpec struct {
	Path   string
	Remote string
	Branch string
	Depth  int // 0 = full clone, >0 = shallow clone
}

// CloneResult holds the outcome of a single clone operation.
type CloneResult struct {
	Name   string
	Path   string
	Output string
	Error  error
}

// validateCloneTarget checks that path is within rootDir and remote looks like a URL.
func validateCloneTarget(rootDir, relPath, remote string) error {
	if strings.HasPrefix(relPath, "-") {
		return fmt.Errorf("invalid path %q: must not start with '-'", relPath)
	}
	absPath, err := filepath.Abs(filepath.Join(rootDir, relPath))
	if err != nil {
		return fmt.Errorf("invalid path %q: %w", relPath, err)
	}
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return fmt.Errorf("invalid root %q: %w", rootDir, err)
	}
	if !strings.HasPrefix(absPath, rootAbs+string(filepath.Separator)) && absPath != rootAbs {
		return fmt.Errorf("path %q escapes root directory", relPath)
	}
	if remote != "" && !strings.Contains(remote, "://") && !strings.Contains(remote, "@") {
		return fmt.Errorf("invalid remote %q: must be a URL or SSH address", remote)
	}
	return nil
}

// Clone clones a single repo. If the target directory already exists, it skips.
func Clone(ctx context.Context, name string, spec CloneSpec) *CloneResult {
	targetPath := spec.Path
	r := &CloneResult{Name: name, Path: targetPath}

	if _, err := os.Stat(targetPath); err == nil {
		r.Output = "already exists, skipped"
		return r
	}

	args := []string{"clone"}
	if spec.Branch != "" {
		args = append(args, "--branch", spec.Branch)
	}
	if spec.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(spec.Depth))
	}
	args = append(args, "--", spec.Remote, targetPath)

	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.CombinedOutput()
	r.Output = strings.TrimSpace(string(out))
	if err != nil {
		r.Error = fmt.Errorf("clone: %s", r.Output)
	}
	return r
}

// parallelDo runs f for each entry in items using bounded parallelism.
// It collects results into a pre-allocated slice in deterministic order.
func parallelDo[T any](ctx context.Context, items map[string]string, parallel int, f func(ctx context.Context, name string) *T) []*T {
	if parallel <= 0 {
		parallel = 4
	}

	results := make([]*T, len(items))
	var idx atomic.Int64

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(parallel)

	for name := range items {
		name := name
		i := int(idx.Add(1)) - 1

		eg.Go(func() error {
			r := f(egCtx, name)
			results[i] = r
			return nil
		})
	}

	_ = eg.Wait()
	return results
}

func GetStatuses(ctx context.Context, rootDir string, repos map[string]string, parallel int) []*RepoStatus {
	return parallelDo(ctx, repos, parallel, func(ctx context.Context, name string) *RepoStatus {
		absPath := filepath.Join(rootDir, repos[name])
		return GetStatus(ctx, name, absPath)
	})
}

func PullAll(ctx context.Context, rootDir string, repos map[string]string, parallel int) []*OperationResult {
	return parallelDo(ctx, repos, parallel, func(ctx context.Context, name string) *OperationResult {
		absPath := filepath.Join(rootDir, repos[name])
		return Pull(ctx, name, absPath)
	})
}

func FetchAll(ctx context.Context, rootDir string, repos map[string]string, parallel int) []*OperationResult {
	return parallelDo(ctx, repos, parallel, func(ctx context.Context, name string) *OperationResult {
		absPath := filepath.Join(rootDir, repos[name])
		return Fetch(ctx, name, absPath)
	})
}

func CloneAll(ctx context.Context, rootDir string, specs map[string]CloneSpec, parallel int) []*CloneResult {
	return parallelDo(ctx, toNameMap(specs), parallel, func(ctx context.Context, name string) *CloneResult {
		spec := specs[name]
		absTarget := filepath.Join(rootDir, spec.Path)
		s := CloneSpec{Path: absTarget, Remote: spec.Remote, Branch: spec.Branch, Depth: spec.Depth}
		return Clone(ctx, name, s)
	})
}

func toNameMap(specs map[string]CloneSpec) map[string]string {
	m := make(map[string]string, len(specs))
	for name := range specs {
		m[name] = specs[name].Path
	}
	return m
}

// BranchInfo holds metadata about a single local branch.
type BranchInfo struct {
	Name    string // branch name, e.g. "main", "feature/foo"
	Current bool   // true if HEAD points to this branch
	Remote  string // upstream short ref, e.g. "origin/main", empty if no upstream
	Ahead   int    // commits ahead of upstream
	Behind  int    // commits behind upstream
}

// GetBranches returns all local branches for a repo with upstream tracking info.
// The current branch is always first.
func GetBranches(ctx context.Context, repoPath string) ([]BranchInfo, error) {
	// Use | as separator since %x00 (NUL) is not expanded on Windows (mingw).
	const sep = "|"
	out, err := gitCmd(ctx, repoPath,
		"for-each-ref",
		"--format=%(refname:short)"+sep+"%(upstream:short)"+sep+"%(upstream:track)"+sep+"%(HEAD)",
		"refs/heads/")
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}
	if out == "" {
		return nil, nil
	}

	var branches []BranchInfo
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, sep, 4)
		if len(parts) < 4 {
			continue
		}
		name := parts[0]
		upstream := parts[1]
		track := parts[2]
		head := parts[3]

		ahead, behind := parseTrackStatus(track)

		bi := BranchInfo{
			Name:    name,
			Current: head == "*",
			Remote:  upstream,
			Ahead:   ahead,
			Behind:  behind,
		}
		branches = append(branches, bi)
	}

	// Sort: current branch first, then alphabetical.
	sort.Slice(branches, func(i, j int) bool {
		if branches[i].Current != branches[j].Current {
			return branches[i].Current
		}
		return branches[i].Name < branches[j].Name
	})

	return branches, nil
}

// parseTrackStatus parses "[ahead N]", "[behind N]", or "[ahead N, behind M]".
var trackRe = regexp.MustCompile(`ahead (\d+)|behind (\d+)`)

func parseTrackStatus(s string) (ahead, behind int) {
	for _, m := range trackRe.FindAllStringSubmatch(s, -1) {
		if m[1] != "" {
			ahead, _ = strconv.Atoi(m[1])
		}
		if m[2] != "" {
			behind, _ = strconv.Atoi(m[2])
		}
	}
	return
}

// RemoteBranchInfo holds metadata about a remote-tracking branch.
type RemoteBranchInfo struct {
	Name string // e.g. "origin/main"
}

// GetRemoteBranches returns all remote-tracking branches for a repo.
func GetRemoteBranches(ctx context.Context, repoPath string) ([]RemoteBranchInfo, error) {
	out, err := gitCmd(ctx, repoPath,
		"for-each-ref",
		"--format=%(refname:short)",
		"refs/remotes/")
	if err != nil {
		return nil, fmt.Errorf("list remote branches: %w", err)
	}
	if out == "" {
		return nil, nil
	}

	var branches []RemoteBranchInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "->") {
			continue
		}
		branches = append(branches, RemoteBranchInfo{Name: line})
	}

	sort.Slice(branches, func(i, j int) bool {
		return branches[i].Name < branches[j].Name
	})

	return branches, nil
}

// Checkout switches to the given branch. If create is true, creates the branch first.
func Checkout(ctx context.Context, repoPath, branch string, create bool) error {
	args := []string{"checkout"}
	if create {
		args = append(args, "-b")
	}
	args = append(args, branch)
	_, err := gitCmd(ctx, repoPath, args...)
	if err != nil {
		return fmt.Errorf("checkout %s: %w", branch, err)
	}
	return nil
}

// DiffFile represents a single changed file in the working tree.
type DiffFile struct {
	Status string // "M", "A", "D", "R", "?"
	Path   string // relative file path
}

// GetDiffFiles returns all changed files (staged, unstaged, and untracked).
func GetDiffFiles(ctx context.Context, repoPath string) ([]DiffFile, error) {
	out, err := gitCmd(ctx, repoPath, "status", "--porcelain=v1")
	if err != nil {
		return nil, fmt.Errorf("get diff files: %w", err)
	}
	if out == "" {
		return nil, nil
	}

	var files []DiffFile
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		x := line[0]
		y := line[1]
		path := line[3:]

		// Determine display status from XY codes.
		status := fileStatusFromXY(x, y)

		// Handle renames: "old -> new"
		if strings.Contains(path, " -> ") {
			parts := strings.SplitN(path, " -> ", 2)
			path = parts[1]
		}

		files = append(files, DiffFile{Status: status, Path: path})
	}

	return files, nil
}

func fileStatusFromXY(x, y byte) string {
	switch {
	case x == '?' && y == '?':
		return "?"
	case x == 'D' || y == 'D':
		return "D"
	case y == 'M' || x == 'M':
		return "M"
	case x == 'A' || y == 'A':
		return "A"
	case x == 'R' || y == 'R':
		return "R"
	default:
		return string(x) + string(y)
	}
}

// FileDiff holds the unified diff content for a single file.
type FileDiff struct {
	Path    string
	Content string
	Error   error
}

// GetFileDiff returns the unified diff for a single file.
// For untracked files, compares against /dev/null.
func GetFileDiff(ctx context.Context, repoPath, filePath string, isUntracked bool) *FileDiff {
	if isUntracked {
		return getFileDiffUntracked(ctx, repoPath, filePath)
	}

	out, err := gitCmd(ctx, repoPath, "diff", "HEAD", "--", filePath)
	if err != nil {
		// File may be newly added with no HEAD comparison.
		return getFileDiffUntracked(ctx, repoPath, filePath)
	}
	return &FileDiff{Path: filePath, Content: out}
}

func getFileDiffUntracked(ctx context.Context, repoPath, filePath string) *FileDiff {
	cmd := exec.CommandContext(ctx, "git", "diff", "--no-index", "/dev/null", filePath)
	cmd.Dir = repoPath
	out, _ := cmd.CombinedOutput()
	content := strings.TrimSpace(string(out))
	if content == "" {
		return &FileDiff{Path: filePath, Error: fmt.Errorf("no diff available")}
	}
	return &FileDiff{Path: filePath, Content: content}
}

func ScanGitRepos(ctx context.Context, rootDir string) ([]string, error) {
	var repos []string

	// Check if root directory itself is a git repo.
	if _, err := os.Stat(filepath.Join(rootDir, ".git")); err == nil {
		repos = append(repos, ".")
	}

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Skip the root directory itself.
		if path == rootDir {
			return nil
		}

		base := filepath.Base(path)

		// Detect Git repo BEFORE skipping .git directories.
		if d.IsDir() && base == ".git" {
			repoPath := filepath.Dir(path)
			relPath, relErr := filepath.Rel(rootDir, repoPath)
			if relErr != nil || relPath == "." {
				return filepath.SkipDir
			}
			repos = append(repos, filepath.ToSlash(relPath))
			return filepath.SkipDir
		}

		// Skip common non-project directories.
		if d.IsDir() && (base == "node_modules" || base == "vendor" || base == ".idea" || base == ".vscode") {
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
