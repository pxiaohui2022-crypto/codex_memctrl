package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProjectScopeUsesGitRootWhenAuto(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	subdir := filepath.Join(root, "nested", "pkg")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	scope, err := resolveProjectScope("", "", true)
	if err != nil {
		t.Fatalf("resolveProjectScope() error = %v", err)
	}
	if scope.Workspace != root {
		t.Fatalf("Workspace = %q, want %q", scope.Workspace, root)
	}
	if scope.Repo != filepath.Base(root) {
		t.Fatalf("Repo = %q, want %q", scope.Repo, filepath.Base(root))
	}
	if !scope.InGitRepo {
		t.Fatal("InGitRepo = false, want true")
	}
}

func TestResolveProjectScopeRespectsExplicitWorkspace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	explicit := filepath.Join(root, "subdir")
	if err := os.MkdirAll(explicit, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	scope, err := resolveProjectScope(explicit, "", true)
	if err != nil {
		t.Fatalf("resolveProjectScope() error = %v", err)
	}
	if scope.Workspace != explicit {
		t.Fatalf("Workspace = %q, want %q", scope.Workspace, explicit)
	}
	if scope.Repo != filepath.Base(root) {
		t.Fatalf("Repo = %q, want %q", scope.Repo, filepath.Base(root))
	}
	if !scope.InGitRepo {
		t.Fatal("InGitRepo = false, want true")
	}
}
