package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
)

type Store interface {
	Path() string
	Close() error
	Add(context.Context, memory.Memory) error
	Get(context.Context, string) (memory.Memory, error)
	List(context.Context) ([]memory.Memory, error)
	Search(context.Context, memory.SearchOptions) ([]memory.Memory, error)
	Import(context.Context, []memory.Memory) error
	Export(context.Context, memory.SearchOptions) (ExportEnvelope, error)
	UpdateStatus(context.Context, string, string) (memory.Memory, error)
}

type ExportEnvelope struct {
	Version    int             `json:"version"`
	ExportedAt time.Time       `json:"exported_at"`
	Memories   []memory.Memory `json:"memories"`
}

func DecodeImport(raw []byte) ([]memory.Memory, error) {
	var envelope ExportEnvelope
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope.Memories) > 0 {
		return envelope.Memories, nil
	}

	var memories []memory.Memory
	if err := json.Unmarshal(raw, &memories); err == nil {
		return memories, nil
	}
	return nil, errors.New("input must be a memctl export envelope or a JSON array of memories")
}

func sortMemories(memories []memory.Memory, opts memory.SearchOptions) {
	sort.SliceStable(memories, func(i, j int) bool {
		left := memories[i].Score(opts)
		right := memories[j].Score(opts)
		if left == right {
			return memories[i].UpdatedAt.After(memories[j].UpdatedAt)
		}
		return left > right
	})
}
