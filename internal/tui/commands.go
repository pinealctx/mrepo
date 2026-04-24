package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/pinealctx/mrepo/internal/config"
	"github.com/pinealctx/mrepo/internal/git"
)

func refreshStatus(rootDir string, repos map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		statuses := git.GetStatuses(ctx, rootDir, repos, runtime.NumCPU())
		details := make(map[string]*git.RepoStatus, len(statuses))
		for _, s := range statuses {
			details[s.Name] = s
		}
		return statusMsg{details: details}
	}
}

func pullAll(rootDir string, repos map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		existing := filterExisting(rootDir, repos)
		results := git.PullAll(ctx, rootDir, existing, runtime.NumCPU())
		out := make(map[string]string, len(results))
		for _, r := range results {
			if r.Error != nil {
				out[r.Name] = fmt.Sprintf("FAIL: %s", r.Error)
			} else {
				out[r.Name] = r.Output
			}
		}
		return pullMsg{results: out}
	}
}

func fetchAllRepos(rootDir string, repos map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		existing := filterExisting(rootDir, repos)
		fetchResults := git.FetchAll(ctx, rootDir, existing, runtime.NumCPU())
		statuses := git.GetStatuses(ctx, rootDir, repos, runtime.NumCPU())
		details := make(map[string]*git.RepoStatus, len(statuses))
		for _, s := range statuses {
			details[s.Name] = s
		}
		out := make(map[string]string, len(fetchResults))
		for _, r := range fetchResults {
			if r.Error != nil {
				out[r.Name] = fmt.Sprintf("FAIL: %s", r.Error)
			} else if r.Output == "" {
				out[r.Name] = "up to date"
			} else {
				out[r.Name] = r.Output
			}
		}
		return fetchMsg{results: out, details: details}
	}
}

func cloneMissing(rootDir string, cfg *config.Config, repos map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		specs := make(map[string]git.CloneSpec)
		for name, path := range repos {
			repo := cfg.Repos[name]
			if repo == nil || repo.Remote == "" || !isMissing(rootDir, path) {
				continue
			}
			specs[name] = git.CloneSpec{Path: repo.Path, Remote: repo.Remote, Branch: repo.Branch}
		}
		if len(specs) == 0 {
			return cloneMsg{results: map[string]string{}}
		}
		results := git.CloneAll(ctx, rootDir, specs, runtime.NumCPU())
		out := make(map[string]string, len(results))
		for _, r := range results {
			if r.Error != nil {
				out[r.Name] = fmt.Sprintf("FAIL: %s", r.Error)
			} else {
				out[r.Name] = r.Output
			}
		}
		return cloneMsg{results: out}
	}
}

func syncAll(rootDir string, cfg *config.Config, repos map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		out := make(map[string]string)
		cloneSpecs := make(map[string]git.CloneSpec)
		for name, path := range repos {
			repo := cfg.Repos[name]
			if repo == nil || repo.Remote == "" || !isMissing(rootDir, path) {
				continue
			}
			cloneSpecs[name] = git.CloneSpec{Path: repo.Path, Remote: repo.Remote, Branch: repo.Branch}
		}
		if len(cloneSpecs) > 0 {
			for _, r := range git.CloneAll(ctx, rootDir, cloneSpecs, runtime.NumCPU()) {
				if r.Error != nil {
					out[r.Name] = fmt.Sprintf("CLONE FAIL: %s", r.Error)
				} else {
					out[r.Name] = "cloned"
				}
			}
		}
		existing := filterExisting(rootDir, repos)
		if len(existing) > 0 {
			for _, r := range git.PullAll(ctx, rootDir, existing, runtime.NumCPU()) {
				if r.Error != nil {
					out[r.Name] = fmt.Sprintf("PULL FAIL: %s", r.Error)
				} else if _, has := out[r.Name]; !has {
					out[r.Name] = "pulled"
				}
			}
		}
		return syncMsg{results: out}
	}
}

// loadDetailForRepo loads branches, remote branches, and diff files for a repo
// as a single synchronous command to avoid multiple rapid re-renders (flickering).
func loadDetailForRepo(rootDir, relPath string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		absPath := filepath.Join(rootDir, relPath)
		branches, err := git.GetBranches(ctx, absPath)
		if err != nil {
			return detailMsg{err: err}
		}
		files, _ := git.GetDiffFiles(ctx, absPath)
		return detailMsg{branches: branches, files: files}
	}
}

func loadFileDiffForRepo(rootDir, relPath, filePath string, isUntracked bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		absPath := filepath.Join(rootDir, relPath)
		return fileDiffMsg{diff: git.GetFileDiff(ctx, absPath, filePath, isUntracked)}
	}
}

func doCheckout(rootDir, relPath, branch string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		absPath := filepath.Join(rootDir, relPath)
		return checkoutResultMsg{err: git.Checkout(ctx, absPath, branch, false)}
	}
}

// --- Helpers ---

func isMissing(rootDir, relPath string) bool {
	_, err := os.Stat(filepath.Join(rootDir, relPath))
	return os.IsNotExist(err)
}

func filterExisting(rootDir string, repos map[string]string) map[string]string {
	existing := make(map[string]string)
	for name, path := range repos {
		if !isMissing(rootDir, path) {
			existing[name] = path
		}
	}
	return existing
}

func buildFileTree(files []git.DiffFile) []fileTreeNode {
	if len(files) == 0 {
		return nil
	}
	addedDirs := make(map[string]bool)
	var nodes []fileTreeNode
	sorted := make([]git.DiffFile, len(files))
	copy(sorted, files)
	slices.SortFunc(sorted, func(a, b git.DiffFile) int {
		return strings.Compare(a.Path, b.Path)
	})
	for _, f := range sorted {
		parts := strings.Split(f.Path, "/")
		for i := 1; i < len(parts); i++ {
			dir := strings.Join(parts[:i], "/")
			if !addedDirs[dir] {
				addedDirs[dir] = true
				nodes = append(nodes, fileTreeNode{Indent: i - 1, Path: dir, IsDir: true})
			}
		}
		nodes = append(nodes, fileTreeNode{Indent: len(parts) - 1, Path: f.Path, Status: f.Status})
	}
	return nodes
}
