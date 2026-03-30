package extractor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCodexHistoryUsesLatestSessionAndRecentTurns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	content := "" +
		"{\"session_id\":\"old\",\"ts\":1,\"text\":\"Prefer concise replies\"}\n" +
		"{\"session_id\":\"new\",\"ts\":2,\"text\":\"Use GoReleaser for release tooling\"}\n" +
		"{\"session_id\":\"new\",\"ts\":3,\"text\":\"Check release notes before tagging\"}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	items, sessionID, err := LoadCodexHistory(path, "", 1)
	if err != nil {
		t.Fatalf("LoadCodexHistory() error = %v", err)
	}
	if sessionID != "new" {
		t.Fatalf("sessionID = %q, want %q", sessionID, "new")
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Text != "Check release notes before tagging" {
		t.Fatalf("items[0].Text = %q", items[0].Text)
	}
}

func TestInspectCodexHistoryReportsLatestSessionSummary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	content := "" +
		"{\"session_id\":\"old\",\"ts\":1,\"text\":\"Prefer concise replies\"}\n" +
		"{\"session_id\":\"new\",\"ts\":2,\"text\":\"Use GoReleaser for release tooling\"}\n" +
		"{\"session_id\":\"new\",\"ts\":3,\"text\":\"Check release notes before tagging\"}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	summary, err := InspectCodexHistory(path)
	if err != nil {
		t.Fatalf("InspectCodexHistory() error = %v", err)
	}
	if !summary.Exists {
		t.Fatal("summary.Exists = false, want true")
	}
	if summary.TotalEntries != 3 {
		t.Fatalf("summary.TotalEntries = %d, want 3", summary.TotalEntries)
	}
	if summary.LatestSessionID != "new" {
		t.Fatalf("summary.LatestSessionID = %q, want %q", summary.LatestSessionID, "new")
	}
	if summary.LatestSessionTurns != 2 {
		t.Fatalf("summary.LatestSessionTurns = %d, want 2", summary.LatestSessionTurns)
	}
	if got := summary.LatestTimestamp.UTC().Unix(); got != 3 {
		t.Fatalf("summary.LatestTimestamp = %d, want 3", got)
	}
}
