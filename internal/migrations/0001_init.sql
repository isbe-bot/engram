CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at TEXT NOT NULL,
  checksum TEXT
);

CREATE TABLE IF NOT EXISTS ingested_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  event_id TEXT NOT NULL UNIQUE,
  event_type TEXT NOT NULL,
  environment_id TEXT NOT NULL,
  occurred_at TEXT NOT NULL,
  data_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ingested_events_occurred_at ON ingested_events(occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_ingested_events_type ON ingested_events(event_type);
CREATE INDEX IF NOT EXISTS idx_ingested_events_env ON ingested_events(environment_id);
