PRAGMA foreign_keys = ON;

CREATE TABLE memories (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  summary TEXT NOT NULL,
  details TEXT NOT NULL DEFAULT '',
  workspace TEXT NOT NULL DEFAULT '',
  repo TEXT NOT NULL DEFAULT '',
  provider TEXT NOT NULL DEFAULT '',
  confidence REAL NOT NULL DEFAULT 0.80 CHECK (confidence >= 0 AND confidence <= 1),
  pinned INTEGER NOT NULL DEFAULT 0 CHECK (pinned IN (0, 1)),
  source_provider TEXT NOT NULL DEFAULT '',
  source_session_id TEXT NOT NULL DEFAULT '',
  source_turn INTEGER NOT NULL DEFAULT 0,
  supersedes_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE memory_tags (
  memory_id TEXT NOT NULL,
  tag TEXT NOT NULL,
  PRIMARY KEY (memory_id, tag),
  FOREIGN KEY (memory_id) REFERENCES memories(id) ON DELETE CASCADE
);

CREATE INDEX idx_memories_scope
  ON memories (workspace, repo, provider, status);

CREATE INDEX idx_memories_updated_at
  ON memories (updated_at DESC);

CREATE INDEX idx_memory_tags_tag
  ON memory_tags (tag);

CREATE VIRTUAL TABLE memories_fts USING fts5(
  memory_id UNINDEXED,
  summary,
  details,
  tags,
  tokenize = 'unicode61'
);
