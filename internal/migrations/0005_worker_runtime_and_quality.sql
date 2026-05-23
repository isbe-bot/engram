CREATE TABLE IF NOT EXISTS worker_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_type TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_until TEXT NOT NULL DEFAULT '',
    next_attempt_at TEXT NOT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    completed_at TEXT NOT NULL DEFAULT '',
    UNIQUE(idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_worker_jobs_state_next_attempt
    ON worker_jobs(state, next_attempt_at);
CREATE INDEX IF NOT EXISTS idx_worker_jobs_type_state
    ON worker_jobs(job_type, state);

CREATE TABLE IF NOT EXISTS worker_checkpoints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL,
    step TEXT NOT NULL,
    state TEXT NOT NULL,
    details_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (job_id) REFERENCES worker_jobs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_worker_checkpoints_job_id
    ON worker_checkpoints(job_id);

CREATE TABLE IF NOT EXISTS quality_latency_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    operation TEXT NOT NULL,
    latency_ms REAL NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_quality_latency_operation_created
    ON quality_latency_samples(operation, created_at);
