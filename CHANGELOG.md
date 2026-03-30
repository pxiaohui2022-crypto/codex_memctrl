# Changelog

All notable changes to `memctl` will be documented in this file.

The format is based on Keep a Changelog and this project follows Semantic Versioning.

## [Unreleased]

### Added

- SQLite runtime store with JSON import/export and automatic legacy `memories.json` migration.
- Cross-platform `memctl` CLI with `init`, `add`, `search`, `status`, `extract`, `review`, `pack`, `export`, `import`, `codex`, `run`, and `version`.
- Scope-aware search and context pack generation for project and global memories.
- Codex wrapper that injects durable memory into the initial prompt and exports pack metadata through environment variables.
- Candidate review workflow with single-item and bulk `--accept-all` / `--archive-all` actions.
- Heuristic memory extraction from plain text and Codex `history.jsonl`.
- Deterministic IDs for extracted memories so repeated extraction from the same source does not multiply identical candidates.
- `memctl status` for path resolution, scope inspection, memory counts, and Codex history diagnostics.
- GitHub Actions CI and GoReleaser-based release automation for Linux, macOS, and Windows.

### Notes

- The first public release is intended to be tagged as `v0.1.0`.
- The current module path targets `github.com/pxiaohui2022-crypto/codex_memctrl`.
