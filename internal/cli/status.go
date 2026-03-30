package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/config"
	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/extractor"
	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
)

type memoryCounts struct {
	Accepted  int `json:"accepted"`
	Candidate int `json:"candidate"`
	Archived  int `json:"archived"`
	Total     int `json:"total"`
}

type historyStatus struct {
	Path               string    `json:"path"`
	Exists             bool      `json:"exists"`
	LatestSessionID    string    `json:"latest_session_id,omitempty"`
	LatestSessionTurns int       `json:"latest_session_turns,omitempty"`
	LatestTimestamp    time.Time `json:"latest_timestamp,omitempty"`
	Error              string    `json:"error,omitempty"`
}

type statusReport struct {
	Version      string        `json:"version"`
	Home         string        `json:"home"`
	ConfigPath   string        `json:"config_path"`
	ConfigExists bool          `json:"config_exists"`
	StorePath    string        `json:"store_path"`
	Workspace    string        `json:"workspace"`
	Repo         string        `json:"repo,omitempty"`
	Provider     string        `json:"provider,omitempty"`
	InGitRepo    bool          `json:"in_git_repo"`
	StoreCounts  memoryCounts  `json:"store_counts"`
	ScopeCounts  memoryCounts  `json:"scope_counts"`
	CodexHistory historyStatus `json:"codex_history"`
}

func (a *App) runStatus(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	storePath := fs.String("store", "", "path to the memory store")
	workspace := fs.String("workspace", "", "workspace scope; defaults to the current repo root")
	repo := fs.String("repo", "", "repo scope; defaults to the current repo name")
	provider := fs.String("provider", "", "optional provider filter for current scope counts")
	historyFile := fs.String("history-file", "", "path to Codex history.jsonl")
	asJSON := fs.Bool("json", false, "print status as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, paths, err := config.Load()
	if err != nil {
		return err
	}

	scope, err := resolveProjectScope(*workspace, *repo, true)
	if err != nil {
		return err
	}

	st, err := a.openStore(*storePath)
	if err != nil {
		return err
	}
	defer st.Close()

	allMemories, err := st.List(ctx)
	if err != nil {
		return err
	}
	scopeMemories, err := st.Search(ctx, memory.SearchOptions{
		Workspace: scope.Workspace,
		Repo:      scope.Repo,
		Provider:  *provider,
		Limit:     0,
	})
	if err != nil {
		return err
	}

	report := statusReport{
		Version:      a.version,
		Home:         paths.Home,
		ConfigPath:   paths.ConfigPath,
		ConfigExists: fileExists(paths.ConfigPath),
		StorePath:    st.Path(),
		Workspace:    scope.Workspace,
		Repo:         scope.Repo,
		Provider:     strings.TrimSpace(*provider),
		InGitRepo:    scope.InGitRepo,
		StoreCounts:  summarizeMemoryCounts(allMemories),
		ScopeCounts:  summarizeMemoryCounts(scopeMemories),
	}

	report.CodexHistory, err = resolveHistoryStatus(strings.TrimSpace(*historyFile))
	if err != nil {
		return err
	}

	if *asJSON {
		return writeJSON(a.stdout, report)
	}
	if strings.TrimSpace(*storePath) == "" {
		report.StorePath = cfg.StorePath
	}
	return a.printStatus(report)
}

func resolveHistoryStatus(path string) (historyStatus, error) {
	if path == "" {
		defaultPath, err := extractor.DefaultCodexHistoryPath()
		if err != nil {
			return historyStatus{}, err
		}
		path = defaultPath
	}

	summary, err := extractor.InspectCodexHistory(path)
	if err != nil {
		return historyStatus{
			Path:  path,
			Error: err.Error(),
		}, nil
	}

	return historyStatus{
		Path:               summary.Path,
		Exists:             summary.Exists,
		LatestSessionID:    summary.LatestSessionID,
		LatestSessionTurns: summary.LatestSessionTurns,
		LatestTimestamp:    summary.LatestTimestamp,
	}, nil
}

func summarizeMemoryCounts(memories []memory.Memory) memoryCounts {
	var counts memoryCounts
	for _, m := range memories {
		counts.Total++
		switch m.Status {
		case memory.StatusAccepted:
			counts.Accepted++
		case memory.StatusCandidate:
			counts.Candidate++
		case memory.StatusArchived:
			counts.Archived++
		}
	}
	return counts
}

func (a *App) printStatus(report statusReport) error {
	if _, err := fmt.Fprintf(a.stdout, "version: %s\n", report.Version); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "home: %s\n", report.Home); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "config: %s\n", report.ConfigPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "config_exists: %t\n", report.ConfigExists); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "store: %s\n\n", report.StorePath); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(a.stdout, "scope:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "  workspace: %s\n", emptyOr(report.Workspace, "<none>")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "  repo: %s\n", emptyOr(report.Repo, "<none>")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "  provider: %s\n", emptyOr(report.Provider, "<all>")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "  in_git_repo: %t\n\n", report.InGitRepo); err != nil {
		return err
	}

	if err := a.printCountsSection("store_counts", report.StoreCounts); err != nil {
		return err
	}
	if err := a.printCountsSection("scope_counts", report.ScopeCounts); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(a.stdout, "codex_history:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "  path: %s\n", report.CodexHistory.Path); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "  exists: %t\n", report.CodexHistory.Exists); err != nil {
		return err
	}
	if report.CodexHistory.Error != "" {
		_, err := fmt.Fprintf(a.stdout, "  error: %s\n", report.CodexHistory.Error)
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "  latest_session: %s\n", emptyOr(report.CodexHistory.LatestSessionID, "<none>")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "  latest_session_turns: %d\n", report.CodexHistory.LatestSessionTurns); err != nil {
		return err
	}
	if report.CodexHistory.LatestTimestamp.IsZero() {
		_, err := fmt.Fprintln(a.stdout, "  latest_timestamp: <none>")
		return err
	}
	_, err := fmt.Fprintf(a.stdout, "  latest_timestamp: %s\n", report.CodexHistory.LatestTimestamp.Format(time.RFC3339))
	return err
}

func (a *App) printCountsSection(name string, counts memoryCounts) error {
	if _, err := fmt.Fprintf(a.stdout, "%s:\n", name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "  accepted: %d\n", counts.Accepted); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "  candidate: %d\n", counts.Candidate); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "  archived: %d\n", counts.Archived); err != nil {
		return err
	}
	_, err := fmt.Fprintf(a.stdout, "  total: %d\n\n", counts.Total)
	return err
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
