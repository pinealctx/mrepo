package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	gitops "github.com/pinealctx/mrepo/internal/git"
)

const operationMessageWidth = 46

type operationProgress struct {
	action  string
	enabled bool
	mu      sync.Mutex
}

type operationRow struct {
	Icon        string
	Name        string
	Action      string
	Result      string
	ResultStyle lipgloss.Style
}

func newOperationProgress(action string, enabled bool) *operationProgress {
	return &operationProgress{
		action:  action,
		enabled: enabled && term.IsTerminal(os.Stderr.Fd()),
	}
}

func (p *operationProgress) Update(completed, total int, result *gitops.OperationResult) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	icon := successIcon()
	if result.Error != nil {
		icon = errorIcon()
	}
	name := truncate(displayRepoName(result.Name), 34)
	fmt.Fprintf(os.Stderr, "\r\x1b[2K  %s %-8s %d/%d  %s", icon, p.action, completed, total, name)
}

func (p *operationProgress) Done() {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprint(os.Stderr, "\r\x1b[2K")
}

func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func operationOutputSummary(output, fallback string) string {
	s := singleLine(output)
	if s == "" {
		return fallback
	}
	lower := strings.ToLower(s)
	switch {
	case strings.Contains(lower, "already up to date"):
		return "up to date"
	case strings.Contains(lower, "fast-forward"):
		return "updated · fast-forward"
	case strings.Contains(lower, "cloning into"):
		return "cloned"
	default:
		return truncate(s, operationMessageWidth)
	}
}

func operationErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	s := singleLine(err.Error())
	lower := strings.ToLower(s)

	switch {
	case strings.Contains(lower, "context deadline exceeded"):
		return "timed out"
	case strings.Contains(lower, "ssl_error_syscall") || strings.Contains(lower, "tls handshake timeout"):
		return "network error · TLS connection failed"
	case strings.Contains(lower, "could not resolve host"):
		return "network error · host could not be resolved"
	case strings.Contains(lower, "connection reset") || strings.Contains(lower, "remote end hung up"):
		return "network error · connection reset"
	case strings.Contains(lower, "authentication failed") || strings.Contains(lower, "access denied"):
		return "authentication failed"
	}

	for _, prefix := range []string{"fetch: fatal: ", "merge: fatal: ", "pull: fatal: ", "fetch: ", "merge: ", "pull: ", "fatal: "} {
		if strings.HasPrefix(strings.ToLower(s), prefix) {
			s = s[len(prefix):]
			break
		}
	}
	if marker := strings.LastIndex(s, "': "); marker >= 0 {
		s = s[marker+3:]
	}
	return truncate(s, operationMessageWidth)
}

func renderOperationTable(rows []operationRow) string {
	nameWidth := len("REPOSITORY")
	actionWidth := 0
	for i := range rows {
		rows[i].Name = truncate(singleLine(rows[i].Name), 34)
		nameWidth = max(nameWidth, lipgloss.Width(rows[i].Name))
		actionWidth = max(actionWidth, lipgloss.Width(rows[i].Action))
	}
	resultWidth := max(20, outputWidth()-nameWidth-actionWidth-9)

	var b strings.Builder
	if actionWidth > 0 {
		actionWidth = max(actionWidth, len("ACTION"))
		resultWidth = max(20, outputWidth()-nameWidth-actionWidth-9)
		fmt.Fprintf(&b, "   %s  %s  %s\n", boldStyle.Render(padVisual("REPOSITORY", nameWidth)), boldStyle.Render(padVisual("ACTION", actionWidth)), boldStyle.Render("RESULT"))
		fmt.Fprintf(&b, "   %s  %s  %s\n", strings.Repeat("─", nameWidth), strings.Repeat("─", actionWidth), strings.Repeat("─", min(resultWidth, 24)))
		for _, row := range rows {
			result := truncate(singleLine(row.Result), resultWidth)
			fmt.Fprintf(&b, " %s %s  %s  %s\n", row.Icon, padVisual(row.Name, nameWidth), padVisual(row.Action, actionWidth), row.ResultStyle.Render(result))
		}
	} else {
		fmt.Fprintf(&b, "   %s  %s\n", boldStyle.Render(padVisual("REPOSITORY", nameWidth)), boldStyle.Render("RESULT"))
		fmt.Fprintf(&b, "   %s  %s\n", strings.Repeat("─", nameWidth), strings.Repeat("─", min(resultWidth, 24)))
		for _, row := range rows {
			result := truncate(singleLine(row.Result), resultWidth)
			fmt.Fprintf(&b, " %s %s  %s\n", row.Icon, padVisual(row.Name, nameWidth), row.ResultStyle.Render(result))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func padVisual(s string, width int) string {
	if padding := width - lipgloss.Width(s); padding > 0 {
		return s + strings.Repeat(" ", padding)
	}
	return s
}

func printOperationSummary(action string, succeeded, failed, skipped int, elapsed time.Duration) {
	parts := []string{fmt.Sprintf("%d succeeded", succeeded)}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	parts = append(parts, elapsed.Round(100*time.Millisecond).String())
	fmt.Printf("\n  %s\n", dimStyle.Render(action+" · "+strings.Join(parts, " · ")))
}

func defaultNetworkParallel() int {
	return gitops.DefaultParallelism()
}

func outputWidth() int {
	width, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || width <= 0 {
		return 96
	}
	if width < 72 {
		return 72
	}
	if width > 120 {
		return 120
	}
	return width
}
