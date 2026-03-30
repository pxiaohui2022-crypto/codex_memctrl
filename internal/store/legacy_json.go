package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
)

type legacyDatabase struct {
	Version  int             `json:"version"`
	Memories []memory.Memory `json:"memories"`
}

func loadLegacyJSON(ctx context.Context, path string) ([]memory.Memory, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}

	var db legacyDatabase
	if err := json.Unmarshal(raw, &db); err == nil && len(db.Memories) > 0 {
		memories := slices.Clone(db.Memories)
		for i := range memories {
			memories[i].Normalize()
		}
		return memories, nil
	}

	memories, err := DecodeImport(raw)
	if err != nil {
		return nil, err
	}
	for i := range memories {
		memories[i].Normalize()
	}
	return memories, nil
}

func defaultSQLitePath(path string) string {
	path = filepath.Clean(path)
	ext := filepath.Ext(path)
	if ext == ".json" {
		return path[:len(path)-len(ext)] + ".db"
	}
	return path
}

func detectLegacyCandidates(path string, extra []string) []string {
	candidates := make([]string, 0, 1+len(extra))
	if filepath.Ext(path) == ".json" {
		candidates = append(candidates, filepath.Clean(path))
	}
	for _, candidate := range extra {
		if candidate == "" {
			continue
		}
		candidate = filepath.Clean(candidate)
		if filepath.Ext(candidate) != ".json" {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return dedupeStrings(candidates)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func hasFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func loadLegacyCandidates(ctx context.Context, candidates []string) ([]memory.Memory, error) {
	for _, candidate := range dedupeStrings(candidates) {
		if !hasFile(candidate) {
			continue
		}
		memories, err := loadLegacyJSON(ctx, candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if len(memories) > 0 {
			return memories, nil
		}
	}
	return nil, nil
}
