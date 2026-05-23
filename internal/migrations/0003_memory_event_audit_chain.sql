ALTER TABLE memory_object_events ADD COLUMN actor_id TEXT NOT NULL DEFAULT 'system';
ALTER TABLE memory_object_events ADD COLUMN mutation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE memory_object_events ADD COLUMN signature TEXT NOT NULL DEFAULT '';
ALTER TABLE memory_object_events ADD COLUMN prev_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE memory_object_events ADD COLUMN event_hash TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_memory_object_events_actor_id ON memory_object_events(actor_id);
CREATE INDEX IF NOT EXISTS idx_memory_object_events_mutation_id ON memory_object_events(mutation_id);
CREATE INDEX IF NOT EXISTS idx_memory_object_events_event_hash ON memory_object_events(event_hash);
