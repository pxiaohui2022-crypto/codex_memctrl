# Architecture

## Goal

`memctl` is not a session sync tool. It is a provider-agnostic memory layer for agent workflows.

The core design choice is to separate:

- provider session state
- durable structured memory
- session bootstrap context

That keeps the product stable even when providers change APIs, IDs, or internal session storage formats.

## Runtime Shape

The MVP ships as one CLI:

- `memctl`

Planned later:

- `memd`: optional local daemon
- `mcp` adapter: memory read/write over MCP

## Memory Lifecycle

1. A user or extractor proposes a structured memory item.
2. The item is stored with scope, status, source, and timestamps.
3. `memctl pack` ranks relevant accepted memories for a target workspace/repo/provider.
4. A new provider session starts with that smaller context pack instead of raw chat history.
5. Future storage backends keep the same semantic model and CLI contract.

Today, candidate creation can happen in two ways:

- manual capture with `memctl add --candidate`
- heuristic extraction with `memctl extract` from notes or Codex `history.jsonl`

## Memory Model

Every memory needs:

- `id`
- `kind`
- `scope`
- `summary`
- `details`
- `tags`
- `source`
- `status`
- `confidence`
- `supersedes`
- `created_at`
- `updated_at`

Why those fields matter:

- `scope` prevents global memory pollution
- `source` makes memories auditable
- `status` supports candidate review workflows
- `supersedes` allows conflict-aware updates instead of silent overwrite

## Store Strategy

Current runtime backend:

- SQLite
- one local database file
- FTS5-backed lookup plus scope-aware ranking in the CLI layer
- normalized tags table
- JSON import/export for backup, sync, and migration

Planned backend improvements:

- audit-friendly link tables
- richer ranking and memory relationship handling

The interface boundary is the important part. The current runtime already uses SQLite and FTS, but the search strategy is still intentionally simple so the CLI contract can stay stable while the backend evolves toward richer linking and conflict handling.

## Command Contract

`memctl add`

- manual creation of accepted or candidate memory

`memctl search`

- FTS-backed keyword lookup plus scope-aware ranking

`memctl status`

- show resolved config/store paths and current scope detection
- summarize store-wide and current-scope memory counts
- report whether Codex history is present and which session is latest

`memctl extract`

- propose candidate memories from text or Codex history
- use deterministic extracted IDs so repeated imports do not multiply identical candidates

`memctl review`

- list candidate memories in the current scope
- accept or archive them without editing the database directly
- support bulk `--accept-all` and `--archive-all` for filtered candidate sets

`memctl pack`

- build a compact context pack for a new session

`memctl codex`

- auto-detect the current project scope
- inject the pack into Codex's initial prompt
- launch interactive `codex` or one-shot `codex exec`

`memctl export`

- export JSON or Markdown for sync, backup, or review

`memctl import`

- restore or merge exported memories

`memctl run`

- generate a pack
- expose it via environment variables
- launch the target command

## Why `run` Uses Env Vars

Provider CLIs do not share one universal prompt-injection contract. A generic wrapper is still useful if it:

- resolves the right memory pack
- makes the pack available at runtime
- leaves provider-specific injection to adapters

That keeps the core CLI clean and avoids hardcoding one provider integration path too early.

## Next Iterations

1. Improve extraction heuristics and session targeting
2. Add MCP server
3. Add provider adapters with automatic pack injection
4. Upgrade search to richer ranking and linking
5. Add conflict resolution UI and pack-size budgeting
