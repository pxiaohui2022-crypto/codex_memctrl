package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
)

type projectScope struct {
	Workspace string
	Repo      string
	InGitRepo bool
}

func resolveProjectScope(workspace, repo string, auto bool) (projectScope, error) {
	workspace = strings.TrimSpace(workspace)
	repo = strings.TrimSpace(repo)

	if !auto && workspace == "" && repo == "" {
		return projectScope{}, nil
	}

	base := workspace
	if base == "" {
		wd, err := os.Getwd()
		if err != nil {
			return projectScope{}, err
		}
		base = wd
	}

	absBase, err := filepath.Abs(base)
	if err != nil {
		return projectScope{}, err
	}
	absBase = filepath.Clean(absBase)

	gitRoot, inGitRepo, err := findGitRoot(absBase)
	if err != nil {
		return projectScope{}, err
	}

	scopeWorkspace := workspace
	if scopeWorkspace == "" {
		if inGitRepo {
			scopeWorkspace = gitRoot
		} else {
			scopeWorkspace = absBase
		}
	} else {
		scopeWorkspace = absBase
	}

	scopeRepo := repo
	if scopeRepo == "" {
		if inGitRepo {
			scopeRepo = filepath.Base(gitRoot)
		} else {
			scopeRepo = filepath.Base(scopeWorkspace)
		}
	}

	if scopeWorkspace == "." || scopeWorkspace == string(filepath.Separator) {
		scopeWorkspace = absBase
	}
	if scopeRepo == "." || scopeRepo == string(filepath.Separator) {
		scopeRepo = ""
	}

	return projectScope{
		Workspace: scopeWorkspace,
		Repo:      scopeRepo,
		InGitRepo: inGitRepo,
	}, nil
}

func findGitRoot(start string) (string, bool, error) {
	start = filepath.Clean(start)
	info, err := os.Stat(start)
	if err != nil {
		return "", false, err
	}
	if !info.IsDir() {
		return "", false, errors.New("scope detection requires a directory")
	}

	current := start
	for {
		if hasGitMarker(current) {
			return current, true, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false, nil
		}
		current = parent
	}
}

func hasGitMarker(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir() || !info.IsDir()
}

func shouldAutoScopeKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case memory.KindProfile, memory.KindProviderNote:
		return false
	default:
		return true
	}
}
