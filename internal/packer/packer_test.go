package packer

import (
	"strings"
	"testing"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
)

func TestRenderMarkdown(t *testing.T) {
	t.Parallel()

	m, err := memory.New(memory.KindProject, "Repository uses GoReleaser for releases")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	m.Tags = []string{"release"}
	m.Details = "Keep the binary CGO-free for easy GitHub distribution."

	out := RenderMarkdown(Scope{Repo: "memctl", Provider: "codex"}, []memory.Memory{m})
	if !strings.Contains(out, "Memctl Context Pack") {
		t.Fatal("output missing title")
	}
	if !strings.Contains(out, m.Summary) {
		t.Fatal("output missing summary")
	}
	if !strings.Contains(out, m.Details) {
		t.Fatal("output missing details")
	}
}
