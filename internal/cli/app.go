package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/config"
	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/extractor"
	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/packer"
	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/runner"
	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/store"
)

type App struct {
	version string
	stdout  io.Writer
	stderr  io.Writer
}

func New(version string, stdout, stderr io.Writer) *App {
	return &App{
		version: version,
		stdout:  stdout,
		stderr:  stderr,
	}
}

func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		a.printRootHelp()
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		a.printRootHelp()
		return nil
	case "version":
		_, err := fmt.Fprintln(a.stdout, a.version)
		return err
	case "init":
		return a.runInit(args[1:])
	case "add":
		return a.runAdd(ctx, args[1:])
	case "search":
		return a.runSearch(ctx, args[1:])
	case "status":
		return a.runStatus(ctx, args[1:])
	case "extract":
		return a.runExtract(ctx, args[1:])
	case "review":
		return a.runReview(ctx, args[1:])
	case "pack":
		return a.runPack(ctx, args[1:])
	case "export":
		return a.runExport(ctx, args[1:])
	case "import":
		return a.runImport(ctx, args[1:])
	case "codex":
		return a.runCodex(ctx, args[1:])
	case "run":
		return a.runWrapped(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (a *App) printRootHelp() {
	fmt.Fprint(a.stdout, `memctl manages provider-agnostic long-term memory for Codex-style workflows.

Usage:
  memctl <command> [flags]

Commands:
  init      Write a default config file
  add       Add a structured memory
  search    Search memories with scope-aware ranking
  status    Show resolved paths, scope, store counts, and Codex history status
  extract   Extract candidate memories from text or Codex history
  review    Review candidate memories and accept/archive them
  pack      Render a context pack for a new session
  export    Export memories as JSON or Markdown
  import    Import memories from JSON
  codex     Launch Codex with injected durable memory
  run       Run a command with a generated context pack in env vars
  version   Print build version

Examples:
  memctl init
  memctl add --kind decision --summary "Prefer Go for release tooling" --tags release
  memctl add --kind decision --summary "Check release notes before tagging" --candidate
  memctl status
  cat notes.txt | memctl extract --apply
  memctl extract --history --apply
  memctl review --accept-all
  memctl search --query release --repo memctl --status accepted
  memctl review
  memctl pack
  memctl codex --prompt "fix the failing tests"
  memctl run --prompt "fix the failing tests" codex
`)
}

func (a *App) runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	force := fs.Bool("force", false, "overwrite config if it already exists")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, paths, err := config.WriteDefault(*force)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.stdout, "config: %s\nstore: %s\npack_limit: %d\n", paths.ConfigPath, cfg.StorePath, cfg.DefaultPackLimit)
	return err
}

func (a *App) runAdd(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	storePath := fs.String("store", "", "path to the memory store")
	kind := fs.String("kind", "", "memory kind")
	summary := fs.String("summary", "", "one-line memory summary")
	details := fs.String("details", "", "optional detailed context")
	workspace := fs.String("workspace", "", "workspace scope")
	repo := fs.String("repo", "", "repo scope")
	provider := fs.String("provider", "", "provider scope")
	global := fs.Bool("global", false, "store this memory without auto-detected project scope")
	candidate := fs.Bool("candidate", false, "store this memory as a candidate for later review")
	status := fs.String("status", memory.StatusAccepted, "candidate|accepted|archived")
	confidence := fs.Float64("confidence", 0.85, "confidence score between 0 and 1")
	sourceProvider := fs.String("source-provider", "", "source provider identifier")
	sourceSession := fs.String("source-session", "", "source session identifier")
	sourceTurn := fs.Int("source-turn", 0, "source turn number")
	tagsCSV := fs.String("tags", "", "comma-separated tags")
	pinned := fs.Bool("pin", false, "mark this memory as pinned")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*kind) == "" || strings.TrimSpace(*summary) == "" {
		return errors.New("add requires --kind and --summary")
	}
	if *candidate {
		*status = memory.StatusCandidate
	}

	if !*global && strings.TrimSpace(*workspace) == "" && strings.TrimSpace(*repo) == "" && shouldAutoScopeKind(*kind) {
		scope, err := resolveProjectScope("", "", true)
		if err != nil {
			return err
		}
		*workspace = scope.Workspace
		*repo = scope.Repo
	}

	st, err := a.openStore(*storePath)
	if err != nil {
		return err
	}
	defer st.Close()

	m, err := memory.New(*kind, *summary)
	if err != nil {
		return err
	}
	m.Details = *details
	m.Scope = memory.Scope{
		Workspace: *workspace,
		Repo:      *repo,
		Provider:  *provider,
	}
	m.Tags = splitCSV(*tagsCSV)
	m.Status = *status
	m.Confidence = *confidence
	m.Pinned = *pinned
	m.Source = memory.Source{
		Provider:  *sourceProvider,
		SessionID: *sourceSession,
		Turn:      *sourceTurn,
	}
	m.UpdatedAt = time.Now().UTC()
	m.Normalize()
	if err := st.Add(ctx, m); err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.stdout, "added %s to %s\n", m.ID, st.Path())
	return err
}

func (a *App) runSearch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	storePath := fs.String("store", "", "path to the memory store")
	query := fs.String("query", "", "search query")
	workspace := fs.String("workspace", "", "workspace scope")
	repo := fs.String("repo", "", "repo scope")
	provider := fs.String("provider", "", "provider scope")
	kind := fs.String("kind", "", "memory kind")
	status := fs.String("status", "", "candidate|accepted|archived")
	limit := fs.Int("limit", 10, "maximum results")
	asJSON := fs.Bool("json", false, "print results as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := a.openStore(*storePath)
	if err != nil {
		return err
	}
	defer st.Close()
	memories, err := st.Search(ctx, memory.SearchOptions{
		Query:     *query,
		Workspace: *workspace,
		Repo:      *repo,
		Provider:  *provider,
		Kind:      *kind,
		Status:    *status,
		Limit:     *limit,
	})
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(a.stdout, memories)
	}
	return a.printMemories(memories)
}

func (a *App) runExtract(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	storePath := fs.String("store", "", "path to the memory store")
	input := fs.String("input", "", "path to a text or markdown file; defaults to stdin when piped")
	history := fs.Bool("history", false, "extract from Codex history.jsonl instead of plain text")
	historyFile := fs.String("history-file", "", "path to Codex history.jsonl")
	sessionID := fs.String("session-id", "", "Codex session id; defaults to the latest session in history")
	recentTurns := fs.Int("recent-turns", 50, "number of recent turns to inspect when using --history")
	workspace := fs.String("workspace", "", "workspace scope")
	repo := fs.String("repo", "", "repo scope")
	provider := fs.String("provider", "", "provider scope")
	global := fs.Bool("global", false, "extract memories without project scope")
	apply := fs.Bool("apply", false, "write extracted memories into the store as candidates")
	maxItems := fs.Int("max", 20, "maximum number of extracted memories")
	minConfidence := fs.Float64("min-confidence", 0.65, "minimum extraction confidence")
	asJSON := fs.Bool("json", false, "print extracted memories as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*global {
		scope, err := resolveProjectScope(*workspace, *repo, true)
		if err != nil {
			return err
		}
		*workspace = scope.Workspace
		*repo = scope.Repo
	}

	items, activeSessionID, err := loadExtractionItems(*input, *history, *historyFile, *sessionID, *recentTurns)
	if err != nil {
		return err
	}
	if *history && strings.TrimSpace(*provider) == "" {
		*provider = "codex"
	}

	memories, err := extractor.Extract(items, extractor.Options{
		Workspace:     *workspace,
		Repo:          *repo,
		Provider:      *provider,
		Status:        memory.StatusCandidate,
		Global:        *global,
		Max:           *maxItems,
		MinConfidence: *minConfidence,
	})
	if err != nil {
		return err
	}

	if *apply {
		st, err := a.openStore(*storePath)
		if err != nil {
			return err
		}
		defer st.Close()
		if err := st.Import(ctx, memories); err != nil {
			return err
		}
		if *history {
			_, err = fmt.Fprintf(a.stdout, "extracted %d candidate memories from Codex session %s into %s\nnext: memctl review\n", len(memories), activeSessionID, st.Path())
			return err
		}
		_, err = fmt.Fprintf(a.stdout, "extracted %d candidate memories into %s\nnext: memctl review\n", len(memories), st.Path())
		return err
	}

	if *asJSON {
		return writeJSON(a.stdout, memories)
	}
	if *history {
		if _, err := fmt.Fprintf(a.stdout, "session: %s\n", activeSessionID); err != nil {
			return err
		}
	}
	return a.printMemories(memories)
}

func (a *App) runReview(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	storePath := fs.String("store", "", "path to the memory store")
	query := fs.String("query", "", "optional search query within candidate memories")
	workspace := fs.String("workspace", "", "workspace scope")
	repo := fs.String("repo", "", "repo scope")
	provider := fs.String("provider", "", "provider scope")
	limit := fs.Int("limit", 20, "maximum number of candidates to show")
	acceptID := fs.String("accept", "", "candidate memory id to accept")
	acceptAll := fs.Bool("accept-all", false, "accept all matching candidate memories")
	archiveID := fs.String("archive", "", "candidate memory id to archive")
	archiveAll := fs.Bool("archive-all", false, "archive all matching candidate memories")
	all := fs.Bool("all", false, "review candidates across all scopes")
	asJSON := fs.Bool("json", false, "print results as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	actionCount := 0
	for _, enabled := range []bool{
		*acceptID != "",
		*acceptAll,
		*archiveID != "",
		*archiveAll,
	} {
		if enabled {
			actionCount++
		}
	}
	if actionCount > 1 {
		return errors.New("review accepts only one action at a time")
	}

	st, err := a.openStore(*storePath)
	if err != nil {
		return err
	}
	defer st.Close()

	if *acceptID != "" {
		m, err := st.UpdateStatus(ctx, *acceptID, memory.StatusAccepted)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(a.stdout, "accepted %s\n", m.ID)
		return err
	}
	if *archiveID != "" {
		m, err := st.UpdateStatus(ctx, *archiveID, memory.StatusArchived)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(a.stdout, "archived %s\n", m.ID)
		return err
	}

	if !*all {
		scope, err := resolveProjectScope(*workspace, *repo, true)
		if err != nil {
			return err
		}
		*workspace = scope.Workspace
		*repo = scope.Repo
	}

	searchOpts := memory.SearchOptions{
		Query:     *query,
		Workspace: *workspace,
		Repo:      *repo,
		Provider:  *provider,
		Status:    memory.StatusCandidate,
		Limit:     *limit,
	}
	if *acceptAll || *archiveAll {
		status := memory.StatusAccepted
		if *archiveAll {
			status = memory.StatusArchived
		}
		updated, err := a.updateMatchingMemoryStatus(ctx, st, searchOpts, status)
		if err != nil {
			return err
		}
		if updated == 0 {
			_, err = fmt.Fprintln(a.stdout, "no candidate memories found")
			return err
		}
		_, err = fmt.Fprintf(a.stdout, "%s %d candidate memories\n", reviewPastTense(status), updated)
		return err
	}

	memories, err := st.Search(ctx, searchOpts)
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(a.stdout, memories)
	}
	if len(memories) == 0 {
		_, err := fmt.Fprintln(a.stdout, "no candidate memories found")
		return err
	}
	return a.printMemories(memories)
}

func (a *App) runPack(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	storePath := fs.String("store", "", "path to the memory store")
	workspace := fs.String("workspace", "", "workspace scope")
	repo := fs.String("repo", "", "repo scope")
	provider := fs.String("provider", "", "provider scope")
	limit := fs.Int("limit", 0, "maximum number of memories in the pack")
	format := fs.String("format", "", "markdown|json")
	output := fs.String("output", "", "write output to a file instead of stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}

	scope, err := resolveProjectScope(*workspace, *repo, true)
	if err != nil {
		return err
	}
	*workspace = scope.Workspace
	*repo = scope.Repo

	cfg, _, err := config.Load()
	if err != nil {
		return err
	}
	if *limit <= 0 {
		*limit = cfg.DefaultPackLimit
	}
	if strings.TrimSpace(*format) == "" {
		*format = cfg.DefaultPackFormat
	}

	st, err := a.openStore(*storePath)
	if err != nil {
		return err
	}
	defer st.Close()
	memories, err := st.Search(ctx, memory.SearchOptions{
		Workspace: *workspace,
		Repo:      *repo,
		Provider:  *provider,
		Status:    memory.StatusAccepted,
		Limit:     *limit,
	})
	if err != nil {
		return err
	}

	var raw []byte
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "markdown", "md":
		raw = []byte(packer.RenderMarkdown(packer.Scope{
			Workspace: *workspace,
			Repo:      *repo,
			Provider:  *provider,
		}, memories))
	case "json":
		raw, err = json.MarshalIndent(memories, "", "  ")
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported format %q", *format)
	}

	if *output == "" {
		_, err = a.stdout.Write(raw)
		if err == nil && len(raw) > 0 && raw[len(raw)-1] != '\n' {
			_, err = fmt.Fprintln(a.stdout)
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(*output, raw, 0o644)
}

func (a *App) runExport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	storePath := fs.String("store", "", "path to the memory store")
	workspace := fs.String("workspace", "", "workspace scope")
	repo := fs.String("repo", "", "repo scope")
	provider := fs.String("provider", "", "provider scope")
	kind := fs.String("kind", "", "memory kind")
	status := fs.String("status", "", "candidate|accepted|archived")
	limit := fs.Int("limit", 1000, "maximum number of exported memories")
	format := fs.String("format", "json", "json|markdown")
	output := fs.String("output", "", "write export to a file instead of stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := a.openStore(*storePath)
	if err != nil {
		return err
	}
	defer st.Close()
	exported, err := st.Export(ctx, memory.SearchOptions{
		Workspace: *workspace,
		Repo:      *repo,
		Provider:  *provider,
		Kind:      *kind,
		Status:    *status,
		Limit:     *limit,
	})
	if err != nil {
		return err
	}

	var raw []byte
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		raw, err = json.MarshalIndent(exported, "", "  ")
		if err != nil {
			return err
		}
	case "markdown", "md":
		raw = []byte(packer.RenderMarkdownExport(exported.Memories))
	default:
		return fmt.Errorf("unsupported format %q", *format)
	}

	if *output == "" {
		_, err = a.stdout.Write(raw)
		if err == nil && len(raw) > 0 && raw[len(raw)-1] != '\n' {
			_, err = fmt.Fprintln(a.stdout)
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(*output, raw, 0o644)
}

func (a *App) runImport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	storePath := fs.String("store", "", "path to the memory store")
	input := fs.String("input", "", "path to a JSON file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*input) == "" {
		return errors.New("import requires --input")
	}

	raw, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	memories, err := store.DecodeImport(raw)
	if err != nil {
		return err
	}
	st, err := a.openStore(*storePath)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Import(ctx, memories); err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.stdout, "imported %d memories into %s\n", len(memories), st.Path())
	return err
}

func (a *App) runWrapped(ctx context.Context, args []string) error {
	opts, command, err := parseRunOptions(args)
	if err != nil {
		return err
	}
	if len(command) == 0 {
		return errors.New("run requires a command, for example: memctl run codex")
	}

	if filepath.Base(command[0]) == "codex" {
		forwarded := command[1:]
		if len(forwarded) > 0 && forwarded[0] == "--" {
			forwarded = forwarded[1:]
		}
		return a.launchCodex(ctx, codexLaunchOptions{
			storePath: opts.storePath,
			workspace: opts.workspace,
			repo:      opts.repo,
			provider:  opts.provider,
			prompt:    opts.prompt,
			limit:     opts.limit,
			dryRun:    opts.dryRun,
		}, forwarded)
	}

	if strings.TrimSpace(opts.prompt) != "" {
		return errors.New("--prompt is only supported when the wrapped command is codex")
	}

	cfg, _, err := config.Load()
	if err != nil {
		return err
	}
	if opts.limit <= 0 {
		opts.limit = cfg.DefaultPackLimit
	}

	scope, err := resolveProjectScope(opts.workspace, opts.repo, true)
	if err != nil {
		return err
	}
	opts.workspace = scope.Workspace
	opts.repo = scope.Repo

	st, err := a.openStore(opts.storePath)
	if err != nil {
		return err
	}
	defer st.Close()
	memories, err := st.Search(ctx, memory.SearchOptions{
		Workspace: opts.workspace,
		Repo:      opts.repo,
		Provider:  opts.provider,
		Status:    memory.StatusAccepted,
		Limit:     opts.limit,
	})
	if err != nil {
		return err
	}

	pack := packer.RenderMarkdown(packer.Scope{
		Workspace: opts.workspace,
		Repo:      opts.repo,
		Provider:  opts.provider,
	}, memories)

	tmp, err := os.CreateTemp("", "memctl-pack-*.md")
	if err != nil {
		return err
	}
	packPath := tmp.Name()
	if _, err := tmp.WriteString(pack); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	defer os.Remove(packPath)

	if opts.dryRun {
		_, err := fmt.Fprintf(a.stdout, "command: %s\npack_path: %s\n\n%s", strings.Join(command, " "), packPath, pack)
		return err
	}

	return runner.RunWithPack(ctx, command, map[string]string{
		"MEMCTL_CONTEXT_PACK":      pack,
		"MEMCTL_CONTEXT_PACK_PATH": packPath,
		"MEMCTL_PROVIDER":          opts.provider,
		"MEMCTL_STORE_PATH":        st.Path(),
	})
}

func (a *App) openStore(override string) (store.Store, error) {
	cfg, paths, err := config.Load()
	if err != nil {
		return nil, err
	}
	path := cfg.StorePath
	legacyCandidates := []string{cfg.LegacyStorePath, paths.LegacyStorePath}
	if strings.TrimSpace(override) != "" {
		path = override
		legacyCandidates = []string{override}
	}
	return store.Open(path, legacyCandidates...)
}

func (a *App) printMemories(memories []memory.Memory) error {
	if len(memories) == 0 {
		_, err := fmt.Fprintln(a.stdout, "no memories found")
		return err
	}
	for _, m := range memories {
		if _, err := fmt.Fprintf(a.stdout, "%s [%s/%s]\n", m.ID, m.Kind, m.Status); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(a.stdout, "  %s\n", m.Summary); err != nil {
			return err
		}
		if details := strings.TrimSpace(m.Details); details != "" {
			if _, err := fmt.Fprintf(a.stdout, "  details: %s\n", strings.ReplaceAll(details, "\n", "\n  ")); err != nil {
				return err
			}
		}
		scope := renderScope(m.Scope)
		if scope != "" {
			if _, err := fmt.Fprintf(a.stdout, "  scope: %s\n", scope); err != nil {
				return err
			}
		}
		source := renderSource(m.Source)
		if source != "" {
			if _, err := fmt.Fprintf(a.stdout, "  source: %s\n", source); err != nil {
				return err
			}
		}
		if len(m.Tags) > 0 {
			if _, err := fmt.Fprintf(a.stdout, "  tags: %s\n", strings.Join(m.Tags, ", ")); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(a.stdout, "  confidence: %.2f\n", m.Confidence); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(a.stdout, "  updated: %s\n", m.UpdatedAt.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return nil
}

func renderScope(scope memory.Scope) string {
	parts := make([]string, 0, 3)
	if scope.Workspace != "" {
		parts = append(parts, "workspace="+scope.Workspace)
	}
	if scope.Repo != "" {
		parts = append(parts, "repo="+scope.Repo)
	}
	if scope.Provider != "" {
		parts = append(parts, "provider="+scope.Provider)
	}
	return strings.Join(parts, " ")
}

func renderSource(source memory.Source) string {
	parts := make([]string, 0, 3)
	if source.Provider != "" {
		parts = append(parts, "provider="+source.Provider)
	}
	if source.SessionID != "" {
		parts = append(parts, "session="+source.SessionID)
	}
	if source.Turn > 0 {
		parts = append(parts, "turn="+strconv.Itoa(source.Turn))
	}
	return strings.Join(parts, " ")
}

func reviewPastTense(status string) string {
	switch status {
	case memory.StatusAccepted:
		return "accepted"
	case memory.StatusArchived:
		return "archived"
	default:
		return "updated"
	}
}

func (a *App) updateMatchingMemoryStatus(ctx context.Context, st store.Store, opts memory.SearchOptions, status string) (int, error) {
	opts.Limit = 0
	memories, err := st.Search(ctx, opts)
	if err != nil {
		return 0, err
	}
	for _, m := range memories {
		if _, err := st.UpdateStatus(ctx, m.ID, status); err != nil {
			return 0, err
		}
	}
	return len(memories), nil
}

func writeJSON(dst io.Writer, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if _, err := dst.Write(raw); err != nil {
		return err
	}
	_, err = fmt.Fprintln(dst)
	return err
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

type runOptions struct {
	storePath string
	workspace string
	repo      string
	provider  string
	prompt    string
	limit     int
	dryRun    bool
}

func parseRunOptions(args []string) (runOptions, []string, error) {
	var opts runOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return opts, args[i+1:], nil
		}
		if !strings.HasPrefix(arg, "-") {
			return opts, args[i:], nil
		}

		key, value, hasValue := strings.Cut(arg, "=")
		switch key {
		case "--store":
			next, err := pickValue(i, value, hasValue, args)
			if err != nil {
				return runOptions{}, nil, err
			}
			opts.storePath = next.value
			i = next.index
		case "--workspace":
			next, err := pickValue(i, value, hasValue, args)
			if err != nil {
				return runOptions{}, nil, err
			}
			opts.workspace = next.value
			i = next.index
		case "--repo":
			next, err := pickValue(i, value, hasValue, args)
			if err != nil {
				return runOptions{}, nil, err
			}
			opts.repo = next.value
			i = next.index
		case "--provider":
			next, err := pickValue(i, value, hasValue, args)
			if err != nil {
				return runOptions{}, nil, err
			}
			opts.provider = next.value
			i = next.index
		case "--prompt":
			next, err := pickValue(i, value, hasValue, args)
			if err != nil {
				return runOptions{}, nil, err
			}
			opts.prompt = next.value
			i = next.index
		case "--limit":
			next, err := pickValue(i, value, hasValue, args)
			if err != nil {
				return runOptions{}, nil, err
			}
			parsed, err := strconv.Atoi(next.value)
			if err != nil {
				return runOptions{}, nil, fmt.Errorf("invalid --limit: %w", err)
			}
			opts.limit = parsed
			i = next.index
		case "--dry-run":
			opts.dryRun = true
		default:
			return runOptions{}, nil, fmt.Errorf("unknown run flag %q", key)
		}
	}
	return opts, nil, nil
}

type valuePick struct {
	value string
	index int
}

func pickValue(current int, value string, hasValue bool, args []string) (valuePick, error) {
	if hasValue {
		return valuePick{value: value, index: current}, nil
	}
	if current+1 >= len(args) {
		return valuePick{}, fmt.Errorf("flag %q requires a value", args[current])
	}
	return valuePick{value: args[current+1], index: current + 1}, nil
}

func loadExtractionItems(input string, history bool, historyFile string, sessionID string, recentTurns int) ([]extractor.Item, string, error) {
	if history {
		if strings.TrimSpace(historyFile) == "" {
			defaultPath, err := extractor.DefaultCodexHistoryPath()
			if err != nil {
				return nil, "", err
			}
			historyFile = defaultPath
		}
		items, activeSessionID, err := extractor.LoadCodexHistory(historyFile, sessionID, recentTurns)
		if err != nil {
			return nil, "", err
		}
		return items, activeSessionID, nil
	}

	raw, err := loadTextInput(input)
	if err != nil {
		return nil, "", err
	}
	return []extractor.Item{{Text: raw}}, "", nil
}

func loadTextInput(input string) (string, error) {
	if strings.TrimSpace(input) != "" {
		raw, err := os.ReadFile(input)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}

	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return "", errors.New("extract requires --input, --history, or piped stdin")
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
