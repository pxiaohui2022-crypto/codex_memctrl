package packer

import (
	"fmt"
	"strings"
	"time"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
)

type Scope struct {
	Workspace string
	Repo      string
	Provider  string
}

func RenderMarkdown(scope Scope, memories []memory.Memory) string {
	var b strings.Builder
	b.WriteString("# Memctl Context Pack\n\n")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))
	b.WriteString("## Scope\n")
	b.WriteString(fmt.Sprintf("- workspace: %s\n", emptyFallback(scope.Workspace, "<global>")))
	b.WriteString(fmt.Sprintf("- repo: %s\n", emptyFallback(scope.Repo, "<unset>")))
	b.WriteString(fmt.Sprintf("- provider: %s\n\n", emptyFallback(scope.Provider, "<unset>")))

	if len(memories) == 0 {
		b.WriteString("## Memories\n")
		b.WriteString("No accepted memory matched this scope.\n")
		return b.String()
	}

	b.WriteString("## Memories\n")
	for _, m := range memories {
		b.WriteString(fmt.Sprintf("### [%s] %s\n", m.Kind, m.Summary))
		if len(m.Tags) > 0 {
			b.WriteString(fmt.Sprintf("- tags: %s\n", strings.Join(m.Tags, ", ")))
		}
		b.WriteString(fmt.Sprintf("- confidence: %.2f\n", m.Confidence))
		if m.Scope.Workspace != "" {
			b.WriteString(fmt.Sprintf("- workspace: %s\n", m.Scope.Workspace))
		}
		if m.Scope.Repo != "" {
			b.WriteString(fmt.Sprintf("- repo: %s\n", m.Scope.Repo))
		}
		if m.Scope.Provider != "" {
			b.WriteString(fmt.Sprintf("- provider: %s\n", m.Scope.Provider))
		}
		if m.Pinned {
			b.WriteString("- pinned: true\n")
		}
		if m.Details != "" {
			b.WriteString(fmt.Sprintf("- details: %s\n", m.Details))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func RenderMarkdownExport(memories []memory.Memory) string {
	scope := Scope{}
	return RenderMarkdown(scope, memories)
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
