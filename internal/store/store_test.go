package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
)

func TestSQLiteStoreAddAndSearch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "memories.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	m, err := memory.New(memory.KindDecision, "Prefer Go for cross-platform releases")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	m.Scope.Repo = "memctl"
	m.Tags = []string{"release", "distribution"}
	m.Normalize()

	if err := st.Add(context.Background(), m); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	results, err := st.Search(context.Background(), memory.SearchOptions{
		Query:  "release",
		Repo:   "memctl",
		Status: memory.StatusAccepted,
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() got %d results, want 1", len(results))
	}
	if results[0].ID != m.ID {
		t.Fatalf("Search() got ID %q, want %q", results[0].ID, m.ID)
	}

	if _, err := os.Stat(filepath.Join(dir, "memories.db")); err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
}

func TestSQLiteStoreSearchIncludesGlobalMemories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "memories.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	global, err := memory.New(memory.KindProfile, "Respond in concise Chinese by default")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	global.Tags = []string{"style"}
	global.Normalize()

	scoped, err := memory.New(memory.KindDecision, "Prefer Go for release tooling")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	scoped.Scope.Repo = "memctl"
	scoped.Normalize()

	if err := st.Import(context.Background(), []memory.Memory{global, scoped}); err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	results, err := st.Search(context.Background(), memory.SearchOptions{
		Repo:   "memctl",
		Status: memory.StatusAccepted,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search() got %d results, want 2", len(results))
	}
}

func TestSQLiteStoreSearchUsesFTSForMultiTermQueries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "memories.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	m, err := memory.New(memory.KindDecision, "Prefer GoReleaser for release automation")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	m.Details = "The CI pipeline should publish GitHub releases from tags."
	m.Tags = []string{"github", "release"}
	m.Normalize()

	if err := st.Add(context.Background(), m); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	results, err := st.Search(context.Background(), memory.SearchOptions{
		Query:  "release pipeline",
		Status: memory.StatusAccepted,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() got %d results, want 1", len(results))
	}
	if results[0].ID != m.ID {
		t.Fatalf("Search() got ID %q, want %q", results[0].ID, m.ID)
	}
}

func TestSQLiteStoreMigratesLegacyJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "memories.json")

	m, err := memory.New(memory.KindDecision, "Legacy JSON should migrate into SQLite")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	raw, err := json.MarshalIndent(ExportEnvelope{
		Version:  1,
		Memories: []memory.Memory{m},
	}, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	if err := os.WriteFile(legacyPath, raw, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	st, err := Open(filepath.Join(dir, "memories.db"), legacyPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	results, err := st.Search(context.Background(), memory.SearchOptions{
		Status: memory.StatusAccepted,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() got %d results, want 1", len(results))
	}
	if results[0].Summary != m.Summary {
		t.Fatalf("Search() got summary %q, want %q", results[0].Summary, m.Summary)
	}
}

func TestSQLiteStoreUpdateStatus(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "memories.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	m, err := memory.New(memory.KindTodo, "Review this candidate")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	m.Status = memory.StatusCandidate
	m.Normalize()

	if err := st.Add(context.Background(), m); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	updated, err := st.UpdateStatus(context.Background(), m.ID, memory.StatusAccepted)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if updated.Status != memory.StatusAccepted {
		t.Fatalf("UpdateStatus() status = %q, want %q", updated.Status, memory.StatusAccepted)
	}
}
