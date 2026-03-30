package cli

import (
	"strings"
	"testing"
)

func TestBuildCodexInitialPromptWithMemories(t *testing.T) {
	t.Parallel()

	pack := "# Memctl Context Pack\n\n- decision: prefer Go\n"
	out := buildCodexInitialPrompt(pack, "Fix the failing tests.", 2)
	if !strings.Contains(out, "BEGIN MEMCTL MEMORY PACK") {
		t.Fatal("prompt missing memory marker")
	}
	if !strings.Contains(out, pack) {
		t.Fatal("prompt missing pack body")
	}
	if !strings.Contains(out, "Fix the failing tests.") {
		t.Fatal("prompt missing user task")
	}
}

func TestBuildCodexInitialPromptWithoutMemories(t *testing.T) {
	t.Parallel()

	out := buildCodexInitialPrompt("", "Fix the failing tests.", 0)
	if out != "Fix the failing tests." {
		t.Fatalf("prompt = %q, want original user task", out)
	}
}
