CREATE TABLE IF NOT EXISTS memory_objects (
  object_id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  schema_version TEXT NOT NULL,
  content TEXT NOT NULL,
  source_refs_json TEXT NOT NULL,
  confidence REAL NOT NULL,
  classification TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memory_objects_type ON memory_objects(type);
CREATE INDEX IF NOT EXISTS idx_memory_objects_status ON memory_objects(status);
CREATE INDEX IF NOT EXISTS idx_memory_objects_updated_at ON memory_objects(updated_at DESC);

CREATE TABLE IF NOT EXISTS memory_object_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  object_id TEXT NOT NULL,
  action TEXT NOT NULL,
  reason TEXT,
  payload_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(object_id) REFERENCES memory_objects(object_id)
);

CREATE INDEX IF NOT EXISTS idx_memory_object_events_object_id ON memory_object_events(object_id);
CREATE INDEX IF NOT EXISTS idx_memory_object_events_action ON memory_object_events(action);
CREATE INDEX IF NOT EXISTS idx_memory_object_events_created_at ON memory_object_events(created_at DESC);
