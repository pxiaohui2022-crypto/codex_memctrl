# memctl v0.1.0

First public release of `memctl`, a local-first memory sidecar for Codex-style agent workflows.

`memctl` keeps long-term memory outside any single API provider, so switching providers does not reset user preferences, project constraints, decisions, or unresolved TODOs.

## Highlights

- SQLite runtime store for durable local memory
- JSON import/export plus automatic legacy `memories.json` migration
- Scope-aware search and context pack generation
- `memctl codex` wrapper with durable memory injected into the initial prompt
- Candidate review workflow with bulk `--accept-all` and `--archive-all`
- Heuristic extraction from plain text and Codex `history.jsonl`
- `memctl status` for store, scope, and Codex history diagnostics

## Included Commands

- `memctl init`
- `memctl add`
- `memctl search`
- `memctl status`
- `memctl extract`
- `memctl review`
- `memctl pack`
- `memctl export`
- `memctl import`
- `memctl codex`
- `memctl run`
- `memctl version`

## Installation

Download the archive for your platform from the release assets, then extract `memctl` and place it on your `PATH`.

Supported release targets:

- Linux `amd64`
- Linux `arm64`
- macOS `amd64`
- macOS `arm64`
- Windows `amd64`
- Windows `arm64`

## Notes

- Runtime storage uses SQLite by default.
- Legacy JSON stores are imported automatically on first open.
- Re-running extraction from the same Codex history session does not duplicate identical extracted candidates.

## Known Gaps

- Extraction is heuristic and intentionally conservative.
- MCP server is not included in `v0.1.0`.
- Provider-specific adapters beyond Codex are planned but not included yet.
