# memctl

`memctl` is a local-first, cross-platform memory sidecar for Codex-style agents. It keeps long-term memory outside any single API provider, so switching providers does not reset user preferences, project constraints, decisions, or unresolved TODOs.

This repository is intentionally biased toward easy GitHub distribution:

- Go only
- single binary
- standard-library runtime
- no CGO requirement
- release automation with GoReleaser + GitHub Actions

The current runtime store is SQLite for durable local search and cross-session use. JSON remains available for import/export and compatibility migration.

## Installation

The easiest distribution path is GitHub Releases. Source builds also work well for contributors and early adopters.

From GitHub Releases:

1. Download the archive for your platform from `https://github.com/pxiaohui2022-crypto/codex_memctrl/releases`.
2. Download `checksums.txt` from the same release.
3. Verify the archive checksum.
4. Extract the archive and place `memctl` on your `PATH`.

Linux example:

```bash
curl -LO https://github.com/pxiaohui2022-crypto/codex_memctrl/releases/download/v0.1.0/memctl_v0.1.0_linux_amd64.tar.gz
curl -LO https://github.com/pxiaohui2022-crypto/codex_memctrl/releases/download/v0.1.0/checksums.txt
sha256sum -c checksums.txt --ignore-missing
tar -xzf memctl_v0.1.0_linux_amd64.tar.gz
install memctl /usr/local/bin/memctl
```

macOS example:

```bash
curl -LO https://github.com/pxiaohui2022-crypto/codex_memctrl/releases/download/v0.1.0/memctl_v0.1.0_darwin_arm64.tar.gz
curl -LO https://github.com/pxiaohui2022-crypto/codex_memctrl/releases/download/v0.1.0/checksums.txt
shasum -a 256 -c checksums.txt
tar -xzf memctl_v0.1.0_darwin_arm64.tar.gz
install memctl /usr/local/bin/memctl
```

Windows PowerShell example:

```powershell
Invoke-WebRequest https://github.com/pxiaohui2022-crypto/codex_memctrl/releases/download/v0.1.0/memctl_v0.1.0_windows_amd64.zip -OutFile memctl_v0.1.0_windows_amd64.zip
Invoke-WebRequest https://github.com/pxiaohui2022-crypto/codex_memctrl/releases/download/v0.1.0/checksums.txt -OutFile checksums.txt
Expand-Archive memctl_v0.1.0_windows_amd64.zip -DestinationPath .
```

From source:

```bash
go build ./cmd/memctl

./memctl version

./memctl status
```

After the module path is finalized and the repository is public, `go install` also works:

```bash
go install github.com/pxiaohui2022-crypto/codex_memctrl/cmd/memctl@latest
```

## Why This Exists

Raw sessions are provider-specific and brittle to synchronize. What survives provider changes is structured long-term memory:

- `profile`: user preferences, language, output style
- `project`: repo context, build/test commands, constraints
- `decision`: tradeoffs and reasons
- `artifact`: key files, docs, scripts, interfaces
- `todo`: unresolved work that spans sessions
- `provider-note`: provider-specific limitations or behavior notes

`memctl` focuses on that layer instead of trying to mirror opaque provider session state.

## Command Set

```text
memctl init
memctl add
memctl search
memctl status
memctl extract
memctl review
memctl pack
memctl export
memctl import
memctl codex
memctl run
memctl version
```

The key workflow is:

1. `memctl add` stores durable structured memory directly.
2. `memctl status` shows resolved paths, current scope, store counts, and Codex history health.
3. `memctl extract` proposes candidate memories from notes or Codex history.
4. `memctl review` accepts or archives candidate memories one by one or in bulk.
5. `memctl pack` assembles a small context pack for a fresh session.
6. `memctl codex` launches Codex with the generated pack injected into the initial prompt.
7. `memctl run` stays available as a generic wrapper for other commands.

## Quick Start

```bash
go build ./cmd/memctl

./memctl init

./memctl status

./memctl add \
  --kind decision \
  --summary "Prefer Go for release tooling" \
  --details "Single static binaries are easier to distribute on GitHub Releases." \
  --tags release,distribution \
  --pin

./memctl add \
  --kind profile \
  --summary "Prefer concise Chinese responses" \
  --global

./memctl add \
  --kind decision \
  --summary "Check release notes before tagging" \
  --candidate

cat notes.txt | ./memctl extract --apply

./memctl extract --history --apply

./memctl review

./memctl review --accept-all --query release

./memctl review --accept memx_123456abcdef

./memctl search --query release --repo memctl --status accepted

./memctl pack

./memctl codex --prompt "Fix the failing tests"

./memctl codex --exec --prompt "Summarize the release pipeline" -- -m gpt-5.4
```

When you run `memctl pack`, `memctl codex`, or `memctl run` inside a Git repo, `memctl` auto-detects the current project root and repo name. Global memories are still included in results; use `memctl add --global` for memories that should not be tied to one project.

Search is backed by SQLite FTS plus scope-aware ranking, so multi-word queries such as `release pipeline` are matched more reliably than plain substring search.

## Status And Diagnostics

`memctl status` is the fastest way to verify that the tool is pointed at the right store and scope before you start extracting or launching Codex.

```bash
./memctl status

./memctl status --json

./memctl status --workspace /path/to/repo --repo my-repo

./memctl status --history-file /path/to/history.jsonl
```

It reports:

- resolved `MEMCTL_HOME`, config path, and active SQLite store path
- current workspace/repo scope and whether that directory is inside a Git repo
- total store counts and current-scope counts by `accepted`, `candidate`, and `archived`
- Codex history availability plus the latest detected session id, turn count, and timestamp

## Extract Candidate Memories

`memctl extract` is the bridge between raw conversation text and durable long-term memory. It never auto-accepts memories; with `--apply` it stores extracted items as `candidate`, then you confirm them through `memctl review`.

Plain text or markdown:

```bash
cat notes.txt | ./memctl extract

cat notes.txt | ./memctl extract --apply

./memctl extract --input notes.md --workspace /path/to/repo --repo my-repo --apply
```

Codex history:

```bash
./memctl extract --history

./memctl extract --history --apply

./memctl extract --history --session-id abc123 --recent-turns 80 --apply

./memctl review

./memctl review --accept-all
```

Behavior:

- `--history` reads `~/.codex/history.jsonl` by default.
- Without `--session-id`, `memctl` uses the latest Codex session found in that file.
- Re-running `extract --history --apply` on the same session does not create duplicate extracted candidates with the same summary and source.
- `review --accept-all` and `review --archive-all` act on the current candidate filter and scope.

`memctl codex` and `memctl run codex` both export:

- `MEMCTL_CONTEXT_PACK`
- `MEMCTL_PROVIDER`
- `MEMCTL_STORE_PATH`
- `MEMCTL_WORKSPACE`
- `MEMCTL_REPO`
- `MEMCTL_MEMORY_COUNT`

`memctl codex` also injects the pack into Codex's initial prompt, so durable memory is available immediately instead of only through environment variables.

## Storage

Default paths follow `os.UserConfigDir()`:

- Linux: `~/.config/memctl`
- macOS: `~/Library/Application Support/memctl`
- Windows: `%AppData%\memctl`

Files:

- `config.json`: CLI defaults
- `memories.db`: SQLite runtime store

If an older `memories.json` file is present, `memctl` imports it automatically into the SQLite store on first open and keeps the JSON file untouched.

Override the root directory with `MEMCTL_HOME`.

## Repository Layout

```text
CHANGELOG.md          user-facing release notes
cmd/memctl/           CLI entrypoint
internal/cli/         command handlers
internal/config/      path resolution and config loading
internal/memory/      memory model and ranking logic
internal/packer/      context-pack rendering
internal/runner/      wrapped command execution
internal/store/       SQLite runtime store plus JSON import/export
docs/architecture.md  architecture and roadmap
docs/releasing.md     release checklist and workflow
docs/sqlite-schema.sql current runtime schema reference
```

## Usable Today

The current version is already usable for day-to-day cross-session memory:

- add structured memories manually
- extract candidate memories from plain text or Codex `history.jsonl`
- stage uncertain memories as candidates and review them later
- bulk accept or archive matching candidates during review
- keep some memories global and others project-scoped
- render a compact context pack for the current repo
- start `codex` with that memory injected automatically
- export the memory store as JSON or Markdown
- import JSON exports and auto-migrate legacy `memories.json` into SQLite on first open

What is not finished yet:

- MCP server
- richer extraction heuristics and provider-specific adapters

## Release

See [`docs/releasing.md`](./docs/releasing.md) for the full checklist.

Quick version:

```bash
go test ./...

go build ./cmd/memctl

git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions will run GoReleaser and publish archives for:

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`
- `windows/arm64`

Release artifacts include:

- platform-specific `.tar.gz` or `.zip` archives
- `checksums.txt`
- GoReleaser-generated changelog text from git history

## Roadmap

- Add an MCP server for multi-agent read/write access
- Improve extraction heuristics, conflict resolution, and pack budgeting
- Add provider adapters that can inject packs automatically
- Add embeddings once keyword + scope search is no longer enough
