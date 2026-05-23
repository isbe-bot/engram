ALTER TABLE memory_objects ADD COLUMN scope TEXT NOT NULL DEFAULT 'local';
ALTER TABLE memory_objects ADD COLUMN provenance_hash TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_memory_objects_scope ON memory_objects(scope);
CREATE INDEX IF NOT EXISTS idx_memory_objects_provenance_hash ON memory_objects(provenance_hash);
