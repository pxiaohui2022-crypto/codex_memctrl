package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
)

var ftsTermPattern = regexp.MustCompile(`[[:alnum:]]+`)

type SQLiteStore struct {
	path string
	db   *sql.DB
}

func Open(path string, legacyCandidates ...string) (Store, error) {
	ctx := context.Background()
	sqlitePath := defaultSQLitePath(path)
	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return nil, err
	}

	store := &SQLiteStore{
		path: sqlitePath,
		db:   db,
	}
	if err := store.init(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.ensureFTSBackfill(ctx); err != nil {
		db.Close()
		return nil, err
	}

	candidates := detectLegacyCandidates(path, legacyCandidates)
	if err := store.migrateLegacyIfNeeded(ctx, candidates); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Path() string {
	return s.path
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) Add(ctx context.Context, m memory.Memory) error {
	m.Normalize()
	if err := m.Validate(); err != nil {
		return err
	}
	return s.upsert(ctx, []memory.Memory{m})
}

func (s *SQLiteStore) Get(ctx context.Context, id string) (memory.Memory, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			m.id,
			m.kind,
			m.status,
			m.summary,
			m.details,
			m.workspace,
			m.repo,
			m.provider,
			m.confidence,
			m.pinned,
			m.source_provider,
			m.source_session_id,
			m.source_turn,
			m.supersedes_json,
			m.created_at,
			m.updated_at,
			COALESCE(GROUP_CONCAT(t.tag, char(31)), ''),
			0.0
		FROM memories m
		LEFT JOIN memory_tags t ON t.memory_id = m.id
		WHERE m.id = ?
		GROUP BY m.id
	`, id)
	m, err := scanMemory(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return memory.Memory{}, fmt.Errorf("memory %q not found", id)
		}
		return memory.Memory{}, err
	}
	return m, nil
}

func (s *SQLiteStore) List(ctx context.Context) ([]memory.Memory, error) {
	memories, err := s.query(ctx, memory.SearchOptions{}, false)
	if err != nil {
		return nil, err
	}
	sortMemories(memories, memory.SearchOptions{})
	return memories, nil
}

func (s *SQLiteStore) Search(ctx context.Context, opts memory.SearchOptions) ([]memory.Memory, error) {
	memories, err := s.query(ctx, opts, true)
	if err != nil {
		return nil, err
	}
	filtered := memories[:0]
	for _, m := range memories {
		if !m.Matches(opts) {
			continue
		}
		if opts.Query != "" && m.Score(opts) == 0 {
			continue
		}
		filtered = append(filtered, m)
	}
	sortMemories(filtered, opts)
	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}
	return filtered, nil
}

func (s *SQLiteStore) Import(ctx context.Context, memories []memory.Memory) error {
	return s.upsert(ctx, memories)
}

func (s *SQLiteStore) Export(ctx context.Context, opts memory.SearchOptions) (ExportEnvelope, error) {
	memories, err := s.Search(ctx, opts)
	if err != nil {
		return ExportEnvelope{}, err
	}
	return ExportEnvelope{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Memories:   memories,
	}, nil
}

func (s *SQLiteStore) UpdateStatus(ctx context.Context, id, status string) (memory.Memory, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case memory.StatusCandidate, memory.StatusAccepted, memory.StatusArchived:
	default:
		return memory.Memory{}, fmt.Errorf("invalid status %q", status)
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE memories
		SET status = ?, updated_at = ?
		WHERE id = ?
	`, status, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return memory.Memory{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return memory.Memory{}, err
	}
	if rowsAffected == 0 {
		return memory.Memory{}, fmt.Errorf("memory %q not found", id)
	}
	return s.Get(ctx, id)
}

func (s *SQLiteStore) init(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA busy_timeout = 5000;`,
		`PRAGMA journal_mode = WAL;`,
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			status TEXT NOT NULL,
			summary TEXT NOT NULL,
			details TEXT NOT NULL DEFAULT '',
			workspace TEXT NOT NULL DEFAULT '',
			repo TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
			pinned INTEGER NOT NULL DEFAULT 0 CHECK (pinned IN (0, 1)),
			source_provider TEXT NOT NULL DEFAULT '',
			source_session_id TEXT NOT NULL DEFAULT '',
			source_turn INTEGER NOT NULL DEFAULT 0,
			supersedes_json TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS memory_tags (
			memory_id TEXT NOT NULL,
			tag TEXT NOT NULL,
			PRIMARY KEY (memory_id, tag),
			FOREIGN KEY (memory_id) REFERENCES memories(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_memories_scope
			ON memories (workspace, repo, provider, status);`,
		`CREATE INDEX IF NOT EXISTS idx_memories_updated_at
			ON memories (updated_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_memory_tags_tag
			ON memory_tags (tag);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
			memory_id UNINDEXED,
			summary,
			details,
			tags,
			tokenize = 'unicode61'
		);`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) ensureFTSBackfill(ctx context.Context) error {
	var totalMemories int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM memories`).Scan(&totalMemories); err != nil {
		return err
	}
	if totalMemories == 0 {
		return nil
	}

	var totalFTS int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM memories_fts`).Scan(&totalFTS); err != nil {
		return err
	}
	if totalFTS == totalMemories {
		return nil
	}
	return s.rebuildFTS(ctx)
}

func (s *SQLiteStore) migrateLegacyIfNeeded(ctx context.Context, candidates []string) error {
	count, err := s.count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	memories, err := loadLegacyCandidates(ctx, candidates)
	if err != nil {
		return fmt.Errorf("load legacy store: %w", err)
	}
	if len(memories) == 0 {
		return nil
	}
	return s.Import(ctx, memories)
}

func (s *SQLiteStore) count(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM memories`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *SQLiteStore) upsert(ctx context.Context, memories []memory.Memory) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, m := range memories {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		m.Normalize()
		if err := m.Validate(); err != nil {
			return err
		}

		supersedesRaw, err := json.Marshal(m.Supersedes)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO memories (
				id, kind, status, summary, details,
				workspace, repo, provider,
				confidence, pinned,
				source_provider, source_session_id, source_turn,
				supersedes_json, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				kind = excluded.kind,
				status = excluded.status,
				summary = excluded.summary,
				details = excluded.details,
				workspace = excluded.workspace,
				repo = excluded.repo,
				provider = excluded.provider,
				confidence = excluded.confidence,
				pinned = excluded.pinned,
				source_provider = excluded.source_provider,
				source_session_id = excluded.source_session_id,
				source_turn = excluded.source_turn,
				supersedes_json = excluded.supersedes_json,
				created_at = excluded.created_at,
				updated_at = excluded.updated_at
		`,
			m.ID,
			m.Kind,
			m.Status,
			m.Summary,
			m.Details,
			m.Scope.Workspace,
			m.Scope.Repo,
			m.Scope.Provider,
			m.Confidence,
			boolToInt(m.Pinned),
			m.Source.Provider,
			m.Source.SessionID,
			m.Source.Turn,
			string(supersedesRaw),
			m.CreatedAt.UTC().Format(time.RFC3339Nano),
			m.UpdatedAt.UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM memory_tags WHERE memory_id = ?`, m.ID); err != nil {
			return err
		}
		for _, tag := range m.Tags {
			if _, err := tx.ExecContext(ctx, `INSERT INTO memory_tags (memory_id, tag) VALUES (?, ?)`, m.ID, tag); err != nil {
				return err
			}
		}
		if err := syncFTS(ctx, tx, m); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) query(ctx context.Context, opts memory.SearchOptions, applyQueryFilter bool) ([]memory.Memory, error) {
	args := make([]any, 0, 8)
	clauses := make([]string, 0, 8)
	ftsQuery := ""

	if opts.Kind != "" {
		clauses = append(clauses, `m.kind = ?`)
		args = append(args, strings.ToLower(strings.TrimSpace(opts.Kind)))
	}
	if opts.Status != "" {
		clauses = append(clauses, `m.status = ?`)
		args = append(args, strings.ToLower(strings.TrimSpace(opts.Status)))
	}
	if opts.Workspace != "" {
		clauses = append(clauses, `(m.workspace = '' OR m.workspace = ?)`)
		args = append(args, filepath.Clean(opts.Workspace))
	}
	if opts.Repo != "" {
		clauses = append(clauses, `(m.repo = '' OR LOWER(m.repo) = LOWER(?))`)
		args = append(args, strings.TrimSpace(opts.Repo))
	}
	if opts.Provider != "" {
		clauses = append(clauses, `(m.provider = '' OR LOWER(m.provider) = LOWER(?))`)
		args = append(args, strings.TrimSpace(opts.Provider))
	}
	if applyQueryFilter && strings.TrimSpace(opts.Query) != "" {
		ftsQuery = buildFTSQuery(opts.Query)
		if ftsQuery == "" {
			like := "%" + strings.ToLower(strings.TrimSpace(opts.Query)) + "%"
			clauses = append(clauses, `(LOWER(m.summary) LIKE ? OR LOWER(m.details) LIKE ? OR LOWER(COALESCE(t.tag, '')) LIKE ?)`)
			args = append(args, like, like, like)
		}
	}

	queryPrefix := ""
	if ftsQuery != "" {
		queryPrefix = `
			WITH ranked_fts AS (
				SELECT memory_id, 0.0 AS fts_rank
				FROM memories_fts
				WHERE memories_fts MATCH ?
			)
		`
		args = append([]any{ftsQuery}, args...)
	}

	query := queryPrefix + `
		SELECT
			m.id,
			m.kind,
			m.status,
			m.summary,
			m.details,
			m.workspace,
			m.repo,
			m.provider,
			m.confidence,
			m.pinned,
			m.source_provider,
			m.source_session_id,
			m.source_turn,
			m.supersedes_json,
			m.created_at,
			m.updated_at,
			COALESCE(GROUP_CONCAT(t.tag, char(31)), ''),
			COALESCE(r.fts_rank, 0)
		FROM memories m
	`
	if ftsQuery != "" {
		query += "\nJOIN ranked_fts r ON r.memory_id = m.id"
	} else {
		query += "\nLEFT JOIN (SELECT '' AS memory_id, 0 AS fts_rank) r ON r.memory_id = m.id"
	}
	query += "\nLEFT JOIN memory_tags t ON t.memory_id = m.id"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += `
		GROUP BY m.id
		ORDER BY m.pinned DESC, r.fts_rank ASC, m.updated_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memories := make([]memory.Memory, 0)
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		memories = append(memories, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return memories, nil
}

func scanMemory(scanner interface {
	Scan(dest ...any) error
}) (memory.Memory, error) {
	var (
		m             memory.Memory
		pinned        int
		supersedesRaw string
		createdAtRaw  string
		updatedAtRaw  string
		tagsRaw       string
		ftsRank       float64
	)

	if err := scanner.Scan(
		&m.ID,
		&m.Kind,
		&m.Status,
		&m.Summary,
		&m.Details,
		&m.Scope.Workspace,
		&m.Scope.Repo,
		&m.Scope.Provider,
		&m.Confidence,
		&pinned,
		&m.Source.Provider,
		&m.Source.SessionID,
		&m.Source.Turn,
		&supersedesRaw,
		&createdAtRaw,
		&updatedAtRaw,
		&tagsRaw,
		&ftsRank,
	); err != nil {
		return memory.Memory{}, err
	}

	if supersedesRaw != "" {
		if err := json.Unmarshal([]byte(supersedesRaw), &m.Supersedes); err != nil {
			return memory.Memory{}, err
		}
	}
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtRaw)
	if err != nil {
		return memory.Memory{}, err
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtRaw)
	if err != nil {
		return memory.Memory{}, err
	}
	m.CreatedAt = createdAt
	m.UpdatedAt = updatedAt
	m.Pinned = pinned == 1
	if tagsRaw != "" {
		m.Tags = strings.Split(tagsRaw, string(rune(31)))
	}
	_ = ftsRank
	m.Normalize()
	return m, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *SQLiteStore) rebuildFTS(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM memories_fts`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO memories_fts (memory_id, summary, details, tags)
		SELECT
			m.id,
			m.summary,
			m.details,
			COALESCE(GROUP_CONCAT(t.tag, ' '), '')
		FROM memories m
		LEFT JOIN memory_tags t ON t.memory_id = m.id
		GROUP BY m.id
	`); err != nil {
		return err
	}
	return tx.Commit()
}

func syncFTS(ctx context.Context, tx *sql.Tx, m memory.Memory) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM memories_fts WHERE memory_id = ?`, m.ID); err != nil {
		return err
	}
	tags := ""
	if len(m.Tags) > 0 {
		tags = strings.Join(m.Tags, " ")
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO memories_fts (memory_id, summary, details, tags)
		VALUES (?, ?, ?, ?)
	`, m.ID, m.Summary, m.Details, tags)
	return err
}

func buildFTSQuery(query string) string {
	terms := ftsTermPattern.FindAllString(strings.ToLower(query), -1)
	if len(terms) == 0 {
		return ""
	}
	parts := make([]string, 0, len(terms))
	for _, term := range terms {
		if term == "" {
			continue
		}
		parts = append(parts, term+"*")
	}
	return strings.Join(parts, " ")
}
