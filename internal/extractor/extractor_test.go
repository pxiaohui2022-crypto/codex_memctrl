package extractor

import (
	"path/filepath"
	"testing"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
)

func TestExtractClassifiesProfileAndProject(t *testing.T) {
	t.Parallel()

	items := []Item{
		{Text: "请用中文简洁回复。"},
		{Text: "要先运行salome shell 再运行python main84.py"},
	}

	memories, err := Extract(items, Options{
		Workspace: filepath.Clean("/tmp/repo"),
		Repo:      "repo",
		Status:    memory.StatusCandidate,
		Max:       10,
	})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if len(memories) != 2 {
		t.Fatalf("Extract() got %d memories, want 2", len(memories))
	}
	if memories[0].Kind != memory.KindProfile {
		t.Fatalf("first memory kind = %q, want %q", memories[0].Kind, memory.KindProfile)
	}
	if memories[1].Kind != memory.KindProject {
		t.Fatalf("second memory kind = %q, want %q", memories[1].Kind, memory.KindProject)
	}
}

func TestExtractRejectsNoise(t *testing.T) {
	t.Parallel()

	items := []Item{{Text: "Traceback (most recent call last): File \"a.py\", line 1"}}
	memories, err := Extract(items, Options{Status: memory.StatusCandidate})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if len(memories) != 0 {
		t.Fatalf("Extract() got %d memories, want 0", len(memories))
	}
}

func TestExtractClassifiesImperativeEnglishStatements(t *testing.T) {
	t.Parallel()

	items := []Item{
		{Text: "Use GoReleaser for release tooling"},
		{Text: "Check release notes before tagging"},
	}

	memories, err := Extract(items, Options{
		Status: memory.StatusCandidate,
		Max:    10,
	})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if len(memories) != 2 {
		t.Fatalf("Extract() got %d memories, want 2", len(memories))
	}
	if memories[0].Kind != memory.KindDecision {
		t.Fatalf("first memory kind = %q, want %q", memories[0].Kind, memory.KindDecision)
	}
	if memories[1].Kind != memory.KindTodo {
		t.Fatalf("second memory kind = %q, want %q", memories[1].Kind, memory.KindTodo)
	}
}

func TestExtractUsesStableIDs(t *testing.T) {
	t.Parallel()

	items := []Item{
		{
			Text: "Use GoReleaser for release tooling",
			Source: memory.Source{
				Provider:  "codex",
				SessionID: "sess1",
				Turn:      3,
			},
		},
	}

	first, err := Extract(items, Options{Status: memory.StatusCandidate})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	second, err := Extract(items, Options{Status: memory.StatusCandidate})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("unexpected extraction lengths: %d %d", len(first), len(second))
	}
	if first[0].ID != second[0].ID {
		t.Fatalf("IDs differ: %q vs %q", first[0].ID, second[0].ID)
	}
}
