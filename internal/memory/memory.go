package memory

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	KindProfile      = "profile"
	KindProject      = "project"
	KindDecision     = "decision"
	KindArtifact     = "artifact"
	KindTodo         = "todo"
	KindProviderNote = "provider-note"
)

const (
	StatusCandidate = "candidate"
	StatusAccepted  = "accepted"
	StatusArchived  = "archived"
)

var validKinds = []string{
	KindProfile,
	KindProject,
	KindDecision,
	KindArtifact,
	KindTodo,
	KindProviderNote,
}

var validStatuses = []string{
	StatusCandidate,
	StatusAccepted,
	StatusArchived,
}

var queryTokenPattern = regexp.MustCompile(`[[:alnum:]]+`)

type Scope struct {
	Workspace string `json:"workspace,omitempty"`
	Repo      string `json:"repo,omitempty"`
	Provider  string `json:"provider,omitempty"`
}

type Source struct {
	Provider  string `json:"provider,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Turn      int    `json:"turn,omitempty"`
}

type Memory struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Scope      Scope     `json:"scope"`
	Summary    string    `json:"summary"`
	Details    string    `json:"details,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	Source     Source    `json:"source,omitempty"`
	Status     string    `json:"status"`
	Confidence float64   `json:"confidence"`
	Pinned     bool      `json:"pinned,omitempty"`
	Supersedes []string  `json:"supersedes,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type SearchOptions struct {
	Query     string
	Workspace string
	Repo      string
	Provider  string
	Kind      string
	Status    string
	Limit     int
}

func New(kind, summary string) (Memory, error) {
	now := time.Now().UTC()
	m := Memory{
		ID:         newID(),
		Kind:       strings.TrimSpace(kind),
		Summary:    strings.TrimSpace(summary),
		Status:     StatusAccepted,
		Confidence: 0.8,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	return m, m.Validate()
}

func (m *Memory) Normalize() {
	m.Kind = strings.TrimSpace(strings.ToLower(m.Kind))
	m.Status = strings.TrimSpace(strings.ToLower(m.Status))
	m.Summary = strings.TrimSpace(m.Summary)
	m.Details = strings.TrimSpace(m.Details)
	m.Scope.Workspace = cleanPath(m.Scope.Workspace)
	m.Scope.Repo = strings.TrimSpace(m.Scope.Repo)
	m.Scope.Provider = strings.TrimSpace(strings.ToLower(m.Scope.Provider))
	m.Source.Provider = strings.TrimSpace(strings.ToLower(m.Source.Provider))
	m.Tags = normalizeTags(m.Tags)
}

func (m Memory) Validate() error {
	if !slices.Contains(validKinds, m.Kind) {
		return fmt.Errorf("invalid kind %q", m.Kind)
	}
	if !slices.Contains(validStatuses, m.Status) {
		return fmt.Errorf("invalid status %q", m.Status)
	}
	if m.Summary == "" {
		return errors.New("summary is required")
	}
	if m.Confidence < 0 || m.Confidence > 1 {
		return errors.New("confidence must be between 0 and 1")
	}
	return nil
}

func (m Memory) Matches(opts SearchOptions) bool {
	if opts.Kind != "" && !strings.EqualFold(m.Kind, opts.Kind) {
		return false
	}
	if opts.Status != "" && !strings.EqualFold(m.Status, opts.Status) {
		return false
	}
	if opts.Workspace != "" && m.Scope.Workspace != "" && cleanPath(m.Scope.Workspace) != cleanPath(opts.Workspace) {
		return false
	}
	if opts.Repo != "" && m.Scope.Repo != "" && !strings.EqualFold(m.Scope.Repo, opts.Repo) {
		return false
	}
	if opts.Provider != "" && m.Scope.Provider != "" && !strings.EqualFold(m.Scope.Provider, opts.Provider) {
		return false
	}
	return true
}

func (m Memory) Score(opts SearchOptions) int {
	score := 0
	if m.Pinned {
		score += 50
	}
	if strings.EqualFold(m.Status, StatusAccepted) {
		score += 20
	}
	if opts.Workspace != "" && cleanPath(m.Scope.Workspace) == cleanPath(opts.Workspace) {
		score += 30
	}
	if opts.Repo != "" && strings.EqualFold(m.Scope.Repo, opts.Repo) {
		score += 20
	}
	if opts.Provider != "" && strings.EqualFold(m.Scope.Provider, opts.Provider) {
		score += 10
	}
	if opts.Query == "" {
		return score
	}

	query := strings.ToLower(strings.TrimSpace(opts.Query))
	summary := strings.ToLower(m.Summary)
	details := strings.ToLower(m.Details)
	if strings.Contains(summary, query) {
		score += 35
	}
	if strings.Contains(details, query) {
		score += 20
	}
	terms := queryTerms(query)
	for _, term := range terms {
		if term == "" {
			continue
		}
		if strings.Contains(summary, term) {
			score += 12
		}
		if strings.Contains(details, term) {
			score += 8
		}
		for _, tag := range m.Tags {
			if strings.Contains(strings.ToLower(tag), term) {
				score += 10
			}
		}
	}
	return score
}

func cleanPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return filepath.Clean(path)
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	slices.Sort(normalized)
	return normalized
}

func queryTerms(query string) []string {
	terms := queryTokenPattern.FindAllString(strings.ToLower(query), -1)
	if len(terms) == 0 && strings.TrimSpace(query) != "" {
		return []string{strings.TrimSpace(strings.ToLower(query))}
	}
	return terms
}

func newID() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("mem_%d", time.Now().UnixNano())
	}
	return "mem_" + hex.EncodeToString(buf)
}
