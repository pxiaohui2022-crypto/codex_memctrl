package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/store"
)

func TestPrintMemoriesIncludesDetailsSourceAndConfidence(t *testing.T) {
	var stdout bytes.Buffer
	app := New("dev", &stdout, &bytes.Buffer{})

	m, err := memory.New(memory.KindDecision, "Prefer Go for release tooling")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	m.Status = memory.StatusCandidate
	m.Details = "Single binary release flow\nNo extra runtime needed"
	m.Scope = memory.Scope{
		Workspace: filepath.Clean("/tmp/repo"),
		Repo:      "repo",
	}
	m.Source = memory.Source{
		Provider:  "codex",
		SessionID: "sess-1",
		Turn:      3,
	}
	m.Tags = []string{"release"}
	m.Confidence = 0.91
	m.Normalize()

	if err := app.printMemories([]memory.Memory{m}); err != nil {
		t.Fatalf("printMemories() error = %v", err)
	}

	out := stdout.String()
	for _, snippet := range []string{
		"details: Single binary release flow",
		"No extra runtime needed",
		"source: provider=codex session=sess-1 turn=3",
		"confidence: 0.91",
	} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("output missing %q:\n%s", snippet, out)
		}
	}
}

func TestRunExtractApplyPrintsReviewHint(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	workspace := filepath.Join(dir, "repo")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	storePath := filepath.Join(dir, "memories.db")
	inputPath := filepath.Join(dir, "notes.txt")
	input := "Use GoReleaser for release tooling\nCheck release notes before tagging\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := New("dev", &stdout, &stderr)
	err := app.Run(ctx, []string{
		"extract",
		"--store", storePath,
		"--input", inputPath,
		"--workspace", workspace,
		"--repo", "repo",
		"--apply",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "next: memctl review") {
		t.Fatalf("stdout missing review hint:\n%s", stdout.String())
	}

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	results, err := st.Search(ctx, memory.SearchOptions{
		Workspace: workspace,
		Repo:      "repo",
		Status:    memory.StatusCandidate,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search() got %d results, want 2", len(results))
	}
}

func TestRunReviewAcceptAllUpdatesMatchingCandidates(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	workspace := filepath.Join(dir, "repo")
	otherWorkspace := filepath.Join(dir, "other")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(otherWorkspace, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	storePath := filepath.Join(dir, "memories.db")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	memories := []memory.Memory{
		mustTestMemory(t, memory.KindTodo, "Review release notes", memory.StatusCandidate, workspace, "repo"),
		mustTestMemory(t, memory.KindDecision, "Use GoReleaser for release tooling", memory.StatusCandidate, workspace, "repo"),
		mustTestMemory(t, memory.KindTodo, "Other repo candidate", memory.StatusCandidate, otherWorkspace, "other"),
	}
	if err := st.Import(ctx, memories); err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := New("dev", &stdout, &stderr)
	err = app.Run(ctx, []string{
		"review",
		"--store", storePath,
		"--workspace", workspace,
		"--repo", "repo",
		"--accept-all",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "accepted 2 candidate memories") {
		t.Fatalf("stdout missing bulk accept result:\n%s", stdout.String())
	}

	accepted, err := st.Search(ctx, memory.SearchOptions{
		Workspace: workspace,
		Repo:      "repo",
		Status:    memory.StatusAccepted,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Search() accepted error = %v", err)
	}
	if len(accepted) != 2 {
		t.Fatalf("accepted count = %d, want 2", len(accepted))
	}

	remaining, err := st.Search(ctx, memory.SearchOptions{
		Workspace: otherWorkspace,
		Repo:      "other",
		Status:    memory.StatusCandidate,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Search() remaining error = %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("remaining candidate count = %d, want 1", len(remaining))
	}
}

func TestRunStatusReportsCountsAndCodexHistory(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	workspace := filepath.Join(dir, "repo")
	otherWorkspace := filepath.Join(dir, "other")
	historyPath := filepath.Join(dir, "history.jsonl")

	t.Setenv("MEMCTL_HOME", home)

	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(otherWorkspace, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	storePath := filepath.Join(home, "memories.db")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	memories := []memory.Memory{
		mustTestMemory(t, memory.KindDecision, "Use GoReleaser for release tooling", memory.StatusAccepted, workspace, "repo"),
		mustTestMemory(t, memory.KindTodo, "Check release notes before tagging", memory.StatusCandidate, workspace, "repo"),
		mustTestMemory(t, memory.KindArtifact, "Old archived note", memory.StatusArchived, otherWorkspace, "other"),
	}
	if err := st.Import(ctx, memories); err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	historyContent := "" +
		"{\"session_id\":\"sess-old\",\"ts\":1,\"text\":\"Older note\"}\n" +
		"{\"session_id\":\"sess-new\",\"ts\":2,\"text\":\"Use GoReleaser for release tooling\"}\n" +
		"{\"session_id\":\"sess-new\",\"ts\":3,\"text\":\"Check release notes before tagging\"}\n"
	if err := os.WriteFile(historyPath, []byte(historyContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := New("dev", &stdout, &stderr)
	err = app.Run(ctx, []string{
		"status",
		"--store", storePath,
		"--workspace", workspace,
		"--repo", "repo",
		"--history-file", historyPath,
		"--json",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var report statusReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nraw=%s", err, stdout.String())
	}

	if report.StoreCounts.Total != 3 || report.StoreCounts.Accepted != 1 || report.StoreCounts.Candidate != 1 || report.StoreCounts.Archived != 1 {
		t.Fatalf("unexpected store counts: %+v", report.StoreCounts)
	}
	if report.ScopeCounts.Total != 2 || report.ScopeCounts.Accepted != 1 || report.ScopeCounts.Candidate != 1 || report.ScopeCounts.Archived != 0 {
		t.Fatalf("unexpected scope counts: %+v", report.ScopeCounts)
	}
	if !report.CodexHistory.Exists {
		t.Fatal("report.CodexHistory.Exists = false, want true")
	}
	if report.CodexHistory.LatestSessionID != "sess-new" {
		t.Fatalf("report.CodexHistory.LatestSessionID = %q, want %q", report.CodexHistory.LatestSessionID, "sess-new")
	}
	if report.CodexHistory.LatestSessionTurns != 2 {
		t.Fatalf("report.CodexHistory.LatestSessionTurns = %d, want 2", report.CodexHistory.LatestSessionTurns)
	}
}

func mustTestMemory(t *testing.T, kind, summary, status, workspace, repo string) memory.Memory {
	t.Helper()

	m, err := memory.New(kind, summary)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	m.Status = status
	m.Scope.Workspace = workspace
	m.Scope.Repo = repo
	m.Normalize()
	return m
}
