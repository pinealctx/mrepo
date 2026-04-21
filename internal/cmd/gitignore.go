package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ensureGitignore adds the repo path to .gitignore if not already present.
// It creates .gitignore if it doesn't exist. Only operates inside git repos.
func ensureGitignore(rootDir, relPath string) error {
	// Only manage .gitignore if root is a git repo.
	gitDir := filepath.Join(rootDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return nil
	}

	// Don't ignore the root itself.
	if relPath == "." || relPath == "" {
		return nil
	}

	gitignorePath := filepath.Join(rootDir, ".gitignore")

	// Read existing entries.
	var lines []string
	f, err := os.Open(gitignorePath)
	if err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		f.Close()
	}

	// Check if already present (match the path with or without trailing /).
	entry := relPath + "/"
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == entry || trimmed == relPath {
			return nil // already ignored
		}
	}

	// Append the mrepo header and the entry.
	w, err := os.OpenFile(gitignorePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}
	defer w.Close()

	hasMrepoHeader := false
	for _, line := range lines {
		if strings.Contains(line, "mrepo") {
			hasMrepoHeader = true
			break
		}
	}

	if !hasMrepoHeader {
		fmt.Fprintln(w, "# mrepo: managed sub-repositories")
	}
	fmt.Fprintln(w, entry)
	return nil
}

// removeFromGitignore removes the repo path from .gitignore.
func removeFromGitignore(rootDir, relPath string) error {
	gitDir := filepath.Join(rootDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return nil
	}

	if relPath == "." || relPath == "" {
		return nil
	}

	gitignorePath := filepath.Join(rootDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		return nil // no .gitignore, nothing to do
	}

	entry := relPath + "/"
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == entry || trimmed == relPath {
			continue
		}
		lines = append(lines, line)
	}

	// Remove trailing empty lines.
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	output := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(gitignorePath, []byte(output), 0o644)
}
