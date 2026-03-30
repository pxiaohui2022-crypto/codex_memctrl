package cli

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/config"
	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/packer"
	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/runner"
)

type codexLaunchOptions struct {
	storePath string
	workspace string
	repo      string
	provider  string
	prompt    string
	limit     int
	dryRun    bool
	execMode  bool
}

func (a *App) runCodex(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("codex", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	storePath := fs.String("store", "", "path to the memory store")
	workspace := fs.String("workspace", "", "workspace scope; defaults to the current repo root")
	repo := fs.String("repo", "", "repo scope; defaults to the current repo name")
	limit := fs.Int("limit", 0, "maximum number of memories in the injected pack")
	prompt := fs.String("prompt", "", "initial task prompt to send to codex")
	dryRun := fs.Bool("dry-run", false, "print the generated command and prompt instead of running codex")
	execMode := fs.Bool("exec", false, "run `codex exec` instead of interactive `codex`")
	if err := fs.Parse(args); err != nil {
		return err
	}

	forwarded := fs.Args()
	if strings.TrimSpace(*prompt) == "" && len(forwarded) > 0 && !strings.HasPrefix(forwarded[0], "-") {
		*prompt = strings.TrimSpace(strings.Join(forwarded, " "))
		forwarded = nil
	}

	return a.launchCodex(ctx, codexLaunchOptions{
		storePath: *storePath,
		workspace: *workspace,
		repo:      *repo,
		provider:  "codex",
		prompt:    *prompt,
		limit:     *limit,
		dryRun:    *dryRun,
		execMode:  *execMode,
	}, forwarded)
}

func (a *App) launchCodex(ctx context.Context, opts codexLaunchOptions, codexArgs []string) error {
	for _, arg := range codexArgs {
		if isCodexSubcommand(arg) {
			return fmt.Errorf("unsupported forwarded codex subcommand %q; use `memctl codex` for interactive sessions or `memctl codex --exec` for one-shot runs", arg)
		}
	}

	cfg, _, err := config.Load()
	if err != nil {
		return err
	}
	if opts.limit <= 0 {
		opts.limit = cfg.DefaultPackLimit
	}
	if strings.TrimSpace(opts.provider) == "" {
		opts.provider = "codex"
	}

	scope, err := resolveProjectScope(opts.workspace, opts.repo, true)
	if err != nil {
		return err
	}

	st, err := a.openStore(opts.storePath)
	if err != nil {
		return err
	}
	defer st.Close()
	memories, err := st.Search(ctx, memory.SearchOptions{
		Workspace: scope.Workspace,
		Repo:      scope.Repo,
		Provider:  opts.provider,
		Status:    memory.StatusAccepted,
		Limit:     opts.limit,
	})
	if err != nil {
		return err
	}

	pack := packer.RenderMarkdown(packer.Scope{
		Workspace: scope.Workspace,
		Repo:      scope.Repo,
		Provider:  opts.provider,
	}, memories)
	initialPrompt := buildCodexInitialPrompt(pack, opts.prompt, len(memories))

	command := []string{"codex"}
	if opts.execMode {
		command = append(command, "exec")
	}
	if scope.Workspace != "" && !hasCodexWorkingDirArg(codexArgs) {
		command = append(command, "-C", scope.Workspace)
	}
	if !scope.InGitRepo && !hasSkipGitRepoCheckArg(codexArgs) {
		command = append(command, "--skip-git-repo-check")
	}
	command = append(command, codexArgs...)
	if initialPrompt != "" {
		command = append(command, initialPrompt)
	}

	if opts.dryRun {
		return a.printCodexDryRun(scope, command, pack, initialPrompt, len(memories))
	}

	return runner.RunWithPack(ctx, command, map[string]string{
		"MEMCTL_CONTEXT_PACK": pack,
		"MEMCTL_PROVIDER":     opts.provider,
		"MEMCTL_STORE_PATH":   st.Path(),
		"MEMCTL_WORKSPACE":    scope.Workspace,
		"MEMCTL_REPO":         scope.Repo,
		"MEMCTL_MEMORY_COUNT": fmt.Sprintf("%d", len(memories)),
	})
}

func buildCodexInitialPrompt(pack, userPrompt string, memoryCount int) string {
	userPrompt = strings.TrimSpace(userPrompt)
	if memoryCount == 0 {
		return userPrompt
	}

	var b strings.Builder
	b.WriteString("Memctl durable memory is loaded for this session.\n\n")
	b.WriteString("Use the memory pack below as background context from previous sessions. ")
	b.WriteString("Treat stable user preferences, project constraints, and prior decisions as durable unless the user changes them in this session. ")
	b.WriteString("Use it when relevant, but do not mechanically repeat it.\n\n")
	b.WriteString("BEGIN MEMCTL MEMORY PACK\n")
	b.WriteString(pack)
	if !strings.HasSuffix(pack, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString("END MEMCTL MEMORY PACK\n\n")
	if userPrompt == "" {
		b.WriteString("There is no task yet. Reply with one short sentence confirming the memory is loaded, then ask what to work on next.")
		return b.String()
	}
	b.WriteString("Current task:\n")
	b.WriteString(userPrompt)
	return b.String()
}

func hasCodexWorkingDirArg(args []string) bool {
	for i, arg := range args {
		if arg == "-C" || arg == "--cd" {
			return true
		}
		if strings.HasPrefix(arg, "--cd=") {
			return true
		}
		if arg == "-C" && i+1 < len(args) {
			return true
		}
	}
	return false
}

func hasSkipGitRepoCheckArg(args []string) bool {
	for _, arg := range args {
		if arg == "--skip-git-repo-check" {
			return true
		}
	}
	return false
}

func isCodexSubcommand(arg string) bool {
	switch arg {
	case "exec", "review", "login", "logout", "mcp", "mcp-server", "app-server", "completion", "sandbox", "debug", "apply", "resume", "fork", "cloud", "features", "help":
		return true
	default:
		return false
	}
}

func (a *App) printCodexDryRun(scope projectScope, command []string, pack, initialPrompt string, memoryCount int) error {
	if _, err := fmt.Fprintf(a.stdout, "workspace: %s\n", emptyOr(scope.Workspace, "<none>")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "repo: %s\n", emptyOr(scope.Repo, "<none>")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "in_git_repo: %t\n", scope.InGitRepo); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "memory_count: %d\n", memoryCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.stdout, "command: %s\n\n", joinCommand(command)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(a.stdout, "initial_prompt:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(a.stdout, initialPrompt); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(a.stdout); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(a.stdout, "context_pack:"); err != nil {
		return err
	}
	_, err := fmt.Fprintln(a.stdout, pack)
	return err
}

func joinCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" {
			quoted = append(quoted, `""`)
			continue
		}
		if strings.ContainsAny(arg, " \t\n\"'") {
			quoted = append(quoted, strconvQuote(arg))
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}

func strconvQuote(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + replacer.Replace(value) + `"`
}

func emptyOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return filepath.Clean(value)
}
