# ENGRAM (Go Core)

Open-source memory intelligence core for AI agents.

ENGRAM is the local memory service layer for long-running AI assistants. It turns scattered chat history, operational events, curated notes, corrections, and project context into governed, searchable, cited memory that an assistant can trust across sessions.

Where a simple script-based memory system is usually a pile of cron jobs, markdown files, Redis buffers, and vector-search helpers, ENGRAM makes memory an operable service: a daemon, a CLI, a versioned schema, an audit trail, retrieval APIs, quality reports, backups, and release gates.

## What ENGRAM is

ENGRAM is a local-first memory intelligence platform composed of:

- **`engramd`** — a daemon that exposes HTTP APIs for ingesting events, curating memory, retrieving cited context, tracking quality, and managing background work.
- **`engramctl`** — an operator CLI for health checks, migrations, search, curation, correction, history, reindexing, retention/compaction reports, backup, restore, and quality reporting.
- **SQLite** — the durable operational ledger for events, memory objects, audit history, worker jobs, checkpoints, and quality samples.
- **Qdrant** — the semantic retrieval layer for vector search and hybrid recall.
- **Policy and governance rules** — validation for memory types, scopes, classifications, source references, secret-like content, protected high-confidence memories, and correction/deprecation reasons.

The goal is not just to store memories. The goal is to make memory **reliable enough to build agentic software on top of it**.

## What ENGRAM does

ENGRAM gives an assistant a governed memory lifecycle:

1. **Ingest** operational events, chat-derived context, workflow events, task outcomes, and external observations.
2. **Curate** accepted memory objects with type, scope, classification, confidence, provenance, and source references.
3. **Retrieve** relevant memories with confidence filters, pagination, deterministic citations, and semantic ranking.
4. **Correct or deprecate** memories through explicit governed mutation paths instead of silent overwrites.
5. **Audit** memory evolution through hash-chained memory-object events.
6. **Index and reindex** memory into Qdrant for semantic recall.
7. **Measure quality** with SLO-oriented reports such as ingest lag, retrieval latency, consolidation freshness, correction latency, and stale-memory rate.
8. **Operate safely** through health checks, backups, restore, systemd deployment, release artifacts, and a runbook.

## Why this improves OpenClaw

OpenClaw is excellent at orchestrating live conversations, tools, sessions, agents, and plugins. ENGRAM improves the long-term memory side of that system by moving memory from script convention into a real service boundary.

With ENGRAM, OpenClaw gets:

- **Durable continuity** — important context survives restarts, compactions, session churn, and agent handoffs.
- **Cited recall** — memory results carry source references so the assistant can show where a claim came from instead of relying on vague recollection.
- **Governed writes** — memory updates are validated, classified, sourced, and audited before they become durable knowledge.
- **Safer memory behavior** — secret-like content is rejected, protected high-confidence memories require explicit force for destructive mutation, and correction reasons must be meaningful.
- **Semantic search as infrastructure** — Qdrant indexing and reindexing are part of the operator surface rather than one-off scripts.
- **Observable quality** — operators can check whether memory ingestion, retrieval, consolidation, and stale-memory rates are healthy.
- **Operational recovery** — SQLite backup/restore and runbook workflows make memory recoverable instead of fragile.
- **Legacy cutover support** — existing script-based memory workflows can route through ENGRAM while keeping fallback paths during migration.

In practice, this means OpenClaw agents can remember project decisions, user preferences, lessons learned, failures, corrections, and task history with better provenance and less drift.

## Benefits

- **For users:** the assistant becomes more consistent, less forgetful, and better at explaining why it believes something.
- **For operators:** memory has health checks, backups, restore paths, quality reports, release artifacts, and service management.
- **For developers:** memory has contracts, migrations, tests, APIs, and deterministic behavior instead of ad hoc scripts.
- **For agent teams:** shared project context, specialist-agent lessons, and cross-agent patterns can be recalled with citations.
- **For production VPS deployments:** ENGRAM can run locally beside OpenClaw with SQLite + Qdrant, avoiding dependence on a remote SaaS memory backend.

## Binaries
- `engramd`: local daemon/service (ingest, curation, retrieval, governance, quality)
- `engramctl`: operator CLI (status, migrate, quality, report, retention, compact, reindex, backup, restore, health, ingest, curate, search, get, correct, deprecate, history)

## Quick start

### Docker + Qdrant development path

The fastest way to try ENGRAM with semantic retrieval is Docker Compose:

```bash
docker compose up --build
```

This starts ENGRAM plus Qdrant and stores local runtime data under `./data/`.

### Local binary path

```bash
make build
./bin/engramctl init --config ./engram.yaml --data-dir ./data --read-api-key dev-read --write-api-key dev-write
./bin/engramd --config ./engram.yaml
./bin/engramctl migrate --config ./engram.yaml
./bin/engramctl status --config ./engram.yaml
./bin/engramctl health --config ./engram.yaml
./bin/engramctl reindex --config ./engram.yaml
./bin/engramctl report --config ./engram.yaml
./bin/engramctl retention --config ./engram.yaml
./bin/engramctl export --config ./engram.yaml --out ./data/backups/engram-export.jsonl
./bin/engramctl backup --config ./engram.yaml --out ./data/backups/engram.sqlite.bak
```

For contributor verification:

```bash
make test
```

## Configuration

ENGRAM loads YAML first, applies sensible defaults, then applies environment variable overrides.

Key YAML fields:

```yaml
server:
  bind: "127.0.0.1"
  port: 8787
  api_key: ""                 # legacy admin Bearer token; prefer scoped api_keys when networked
  api_keys:
    - name: read-client
      token: ""
      scopes: [read]
    - name: writer
      token: ""
      scopes: [read, write]
    - name: operator
      token: ""
      scopes: [admin]
  max_body_bytes: 1048576     # write-endpoint JSON body limit
  rate_limit_per_minute: 0    # optional per-client limit; 0 disables

storage:
  sqlite_path: "./data/engram.sqlite"
  qdrant_url: "http://127.0.0.1:6333"
  qdrant_collection: "engram_memory"

ingestion:
  max_batch_size: 200
  worker_count: 2

retention:
  event_retention_days: 90
  deprecated_memory_retention_days: 180
  stale_memory_days: 30
  max_candidates: 1000
```

Supported environment overrides:

- `ENGRAM_SERVER_BIND`
- `ENGRAM_SERVER_PORT`
- `ENGRAM_API_KEY` legacy admin token
- `ENGRAM_READ_API_KEY` appends a read-scoped token
- `ENGRAM_WRITE_API_KEY` appends a read/write-scoped token
- `ENGRAM_ADMIN_API_KEY` appends an admin-scoped token
- `ENGRAM_MAX_BODY_BYTES`
- `ENGRAM_RATE_LIMIT_PER_MINUTE`
- `ENGRAM_SQLITE_PATH`
- `ENGRAM_QDRANT_URL`
- `ENGRAM_QDRANT_COLLECTION`
- `ENGRAM_MAX_BATCH_SIZE`
- `ENGRAM_WORKER_COUNT`
- `ENGRAM_QUALITY_EVAL_INTERVAL`
- `ENGRAM_RETENTION_EVENT_DAYS`
- `ENGRAM_RETENTION_DEPRECATED_MEMORY_DAYS`
- `ENGRAM_RETENTION_STALE_MEMORY_DAYS`
- `ENGRAM_RETENTION_MAX_CANDIDATES`

## Production security recommendations

- Keep `server.bind` on `127.0.0.1` unless ENGRAM is intentionally being exposed.
- Set scoped `server.api_keys` or `ENGRAM_READ_API_KEY` / `ENGRAM_WRITE_API_KEY` / `ENGRAM_ADMIN_API_KEY` before putting ENGRAM behind Docker port publishing, a reverse proxy, Tailscale, or any non-local interface. The legacy `server.api_key` / `ENGRAM_API_KEY` still works as an admin-compatible token for simple local deployments.
- Authenticated clients must send `Authorization: Bearer <token>`.
- `/v1/health` remains unauthenticated for local liveness checks. All other API endpoints, including `/metrics`, require a valid token when configured.
- Prefer least privilege: give dashboards/search adapters `read`, ingestion/curation jobs `read, write`, and operators `admin`.
- Keep `server.max_body_bytes` low enough for expected memory payloads. The default is 1 MiB.
- Set `server.rate_limit_per_minute` when exposing ENGRAM beyond trusted local automation. `0` disables rate limiting for local sidecars.
- Rotate API keys by changing `server.api_key` or `ENGRAM_API_KEY`, restarting `engramd`, then updating clients. Avoid logging or committing tokens.
- Prefer backup/restore workflows over direct SQLite edits.

## OpenClaw integration

ENGRAM can be used as OpenClaw's governed semantic memory sidecar in two ways:

1. **Native OpenClaw plugin** — preferred when plugin install is available. [`integrations/openclaw-plugin/`](integrations/openclaw-plugin/) registers ENGRAM as a memory corpus supplement plus `engram_status`, `engram_search`, `engram_get`, and optional `engram_curate` tools. See [`docs/OPENCLAW_PLUGIN.md`](docs/OPENCLAW_PLUGIN.md).
2. **Script adapter** — compatibility path for existing memory wrapper scripts.

The script adapter is provided at:

```text
examples/openclaw/mem-search-engram.js
```

Example:

```bash
ENGRAMCTL_BIN=/usr/local/bin/engramctl \
ENGRAM_CONFIG=/etc/engram/engram.yaml \
node examples/openclaw/mem-search-engram.js "project decision" --json --limit 5
```

Both paths preserve ENGRAM's API key handling, citations, confidence filters, and Qdrant-backed recall. The plugin tries HTTP first and safely falls back to `engramctl` when configured.

### Prompt for installing ENGRAM with OpenClaw

Copy/paste this into an OpenClaw session on the target host:

```text
You are installing ENGRAM, a local-first governed memory daemon and CLI for OpenClaw.

Goal:
Install ENGRAM from https://github.com/isbe-bot/engram, run it as a local sidecar, verify health/search, and optionally enable the native OpenClaw plugin.

Rules:
- Act on this host. Do not ask me to run commands unless you are blocked by permissions.
- Do not expose ENGRAM publicly. Keep it bound to 127.0.0.1 unless I explicitly approve otherwise.
- Use scoped API tokens: read for search/get, write for ingest/curate, admin only for operators.
- Preserve existing data/config if ENGRAM is already installed. Back up before replacing binaries or config.
- Prefer systemd user service for a per-user OpenClaw sidecar; use system service only if this host is intentionally managed that way.
- Verify every step with commands and report exact paths, service status, and health output.

Steps:
1. Inspect the host: OS, architecture, Go version, git, systemd availability, OpenClaw workspace path, and whether `engramd` / `engramctl` already exist.
2. Clone or update the repo:
   - If no checkout exists: `git clone https://github.com/isbe-bot/engram.git ~/engram`
   - If a checkout exists: fetch/pull safely after checking `git status`.
3. Build:
   - `make build`
   - Verify `./bin/engramd` and `./bin/engramctl` exist.
4. Initialize local config/data if missing:
   - `./bin/engramctl init --config ~/.config/engram/engram.yaml --data-dir ~/.local/share/engram --read-api-key <generate-read-token> --write-api-key <generate-write-token> --admin-api-key <generate-admin-token>`
   - Generate strong random tokens locally. Do not print full secrets in chat; store them securely.
5. Install binaries:
   - `install -m 0755 ./bin/engramd ~/.local/bin/engramd`
   - `install -m 0755 ./bin/engramctl ~/.local/bin/engramctl`
6. Create or update a systemd user service for `engramd` using `~/.local/bin/engramd --config ~/.config/engram/engram.yaml`.
7. Start and verify:
   - `systemctl --user daemon-reload`
   - `systemctl --user enable --now engramd.service`
   - `systemctl --user is-active engramd.service`
   - `~/.local/bin/engramctl health --config ~/.config/engram/engram.yaml`
   - `~/.local/bin/engramctl status --config ~/.config/engram/engram.yaml`
8. Run a smoke test:
   - Curate one non-sensitive test memory with source ref `spec:install-smoke`.
   - Search for it.
   - Confirm the result includes citations/provenance.
9. Optional OpenClaw plugin integration:
   - Install the local plugin from the checkout: `openclaw plugins install ~/engram/integrations/openclaw-plugin`
   - Configure plugin `engram` with endpoint `http://127.0.0.1:8787`, the read token, `~/.config/engram/engram.yaml`, and fallbackToCli=true.
   - Restart OpenClaw only after confirming config changes are valid.
   - Verify `engram_status` / `engram_search` or the plugin smoke script if available.
10. Final report:
   - Repo path, binary paths, config path, data path, service status, health result, memory counts, and whether the OpenClaw plugin was enabled.
   - Mention any skipped step and why.
```

## Current API (v1 bootstrap)

- `GET /v1/health`
- `POST /v1/events/ingest`
- `POST /v1/memory/curate`
- `GET /v1/memory/{object_id}`
- `POST /v1/memory/{object_id}/correct` (supports `force=true` for protected high-confidence memory)
- `POST /v1/memory/{object_id}/deprecate` (supports `force=true` for protected high-confidence memory)
- `GET /v1/memory/{object_id}/history?limit=<n>&action=<curated|corrected|deprecated>&before=<event_id>`
- `GET /v1/memory/search?q=<term>&status=<accepted|deprecated>&min_confidence=<0..1>&limit=<n>&cursor=<offset>&include_events=<true|false>`
- `GET /v1/quality/metrics`
- `GET /v1/quality/report`
- `GET /metrics` (Prometheus-style text metrics)

Governance mutation endpoints require signed envelope fields:
- `envelope.actor_id`
- `envelope.mutation_id`
- `envelope.signature`

### Example ingest + curate + governance + search

```bash
curl -s -X POST http://127.0.0.1:8787/v1/events/ingest \
  -H 'content-type: application/json' \
  -d '{
    "event_id":"evt-001",
    "event_type":"task.completed",
    "environment_id":"client-alpha",
    "occurred_at":"2026-05-23T02:00:00Z",
    "data":{"title":"Kickoff complete"}
  }'

curl -s -X POST http://127.0.0.1:8787/v1/memory/curate \
  -H 'content-type: application/json' \
  -d '{
    "object_id":"mem-1",
    "type":"decision",
    "content":"Use Go for ENGRAM core",
    "source_refs":["adr:0009"],
    "confidence":0.85,
    "classification":"product",
    "scope":"local",
    "envelope":{"actor_id":"operator","mutation_id":"mut-001","signature":"sig-001"}
  }'

curl -s -X POST http://127.0.0.1:8787/v1/memory/mem-1/correct \
  -H 'content-type: application/json' \
  -d '{
    "content":"Use Go + SQLite for ENGRAM core",
    "reason":"Clarify storage architecture",
    "source_refs":["adr:0009","spec:blueprint-v1"],
    "envelope":{"actor_id":"operator","mutation_id":"mut-002","signature":"sig-002"}
  }'

curl -s -X POST http://127.0.0.1:8787/v1/memory/mem-1/deprecate \
  -H 'content-type: application/json' \
  -d '{"reason":"Superseded by revised architecture","envelope":{"actor_id":"operator","mutation_id":"mut-003","signature":"sig-003"}}'

curl -s 'http://127.0.0.1:8787/v1/memory/search?q=Go&status=accepted&min_confidence=0.9&limit=10&include_events=true'

curl -s http://127.0.0.1:8787/v1/quality/metrics
curl -s http://127.0.0.1:8787/v1/quality/report
```

### Example CLI API usage

```bash
./bin/engramctl ingest --config ./configs/example.yaml --json '{
  "event_id":"evt-001",
  "event_type":"task.completed",
  "environment_id":"client-alpha",
  "occurred_at":"2026-05-23T02:00:00Z",
  "data":{"title":"Kickoff complete"}
}'

./bin/engramctl curate --config ./configs/example.yaml --json '{
  "object_id":"mem-1",
  "type":"decision",
  "content":"Use Go for ENGRAM core",
  "source_refs":["adr:0009"],
  "confidence":0.85,
  "classification":"product",
  "envelope":{"actor_id":"operator","mutation_id":"mut-001","signature":"sig-001"}
}'

./bin/engramctl get --config ./configs/example.yaml --id mem-1
./bin/engramctl search --config ./configs/example.yaml --q Go --status accepted --min-confidence 0.8 --include-events
./bin/engramctl history --config ./configs/example.yaml --id mem-1 --action curated
```

### Portable JSONL import/export

Use canonical JSONL for migrations, audits, and cross-VPS moves:

```bash
./bin/engramctl export --config ./configs/example.yaml --out ./data/engram-export.jsonl
./bin/engramctl import --config ./configs/example.yaml --file ./data/engram-export.jsonl --dry-run
./bin/engramctl import --config ./configs/example.yaml --file ./data/engram-export.jsonl --reindex=true
```

JSONL records use `kind: "event"` or `kind: "memory_object"` with `version: "engram.portable.v1"`. Imports validate events and memory objects, skip duplicates, and reindex imported memory into Qdrant by default when Qdrant is configured.

### Retention and compaction

ENGRAM separates report-only retention auditing from destructive cleanup:

```bash
./bin/engramctl retention --config ./configs/example.yaml
./bin/engramctl retention --config ./configs/example.yaml --event-retention-days 120 --max-candidates 50
./bin/engramctl compact --config ./configs/example.yaml --apply
```

The report lists:

- raw ingested events old enough to delete;
- deprecated memory objects old enough to delete;
- stale accepted memory objects that need human review before compaction.

`compact --apply` only deletes raw events and deprecated memory that exceed the configured retention cutoffs. Accepted stale memory is review-only and is never deleted automatically.

For JSON-heavy commands, use `--file payload.json` or `--file -` for stdin instead of `--json`.

Search responses include:
- `rank_score` (lightweight relevance ranking)
- `next_cursor` (offset pagination cursor for memory-object results)
- deterministic citation IDs
- `citations` for each result:
  - memory objects: `memory_objects/{object_id}` + linked `source_refs`
  - event hits: `ingested_events/{event_id}`

Governance behavior includes quality guardrails:
- correction/deprecation reasons must be sufficiently descriptive (not vague one-word reasons like `fix`)
- source refs must use approved prefixes (`adr:`, `chat:`, `spec:`, `meeting:`, `task:`, `event:`, `doc:`)
- high-confidence memories (`confidence >= 0.90`) are immutable unless `force=true` is explicitly provided
- memory object events include hash-chain audit fields (`prev_hash`, `event_hash`) for tamper-evidence

See `docs/API.md` for a clean API reference, `docs/openapi.yaml` for a compact OpenAPI spec, and `docs/OPENCLAW_PLUGIN.md` for native OpenClaw plugin setup.

## Project layout
```text
cmd/engramd
cmd/engramctl
internal/{api,config,ingest,curate,retrieve,govern,quality,storage,workers,events,models,migrations}
pkg/contracts
configs
```

This foundation is aligned with `engram-go-service-blueprint-v1.md` and now includes:

- SQLite-backed event ingestion
- memory object contract tables (`memory_objects`, `memory_object_events`)
- migration runner (embedded SQL)
- curation and governance endpoints (`curate`, `get`, `correct`, `deprecate`, `history`)
- filtered retrieval over curated `memory_objects` with confidence/status controls and citation paths
- CLI status/migrate/quality/retention commands (event + memory counts, freshness, correction/deprecation rates, retention/compaction candidates)
- API-backed CLI commands for health, ingest, curate, search, get, correct, deprecate, and history
- operator reindex command to rebuild Qdrant vectors from SQLite memory objects
- backup/restore commands for SQLite operational recovery
- systemd unit, install script, release script, and operator runbook
- worker runtime durability tables (`worker_jobs`, `worker_checkpoints`) with idempotent enqueue semantics
- quality metrics endpoint for ingestion freshness, memory counts, audit action counts, and governance rates
- quality SLO report endpoint (`/v1/quality/report`) and CLI command (`engramctl report`)
- legacy compatibility wrapper script (`scripts/legacy-memory.sh`) and cutover notes (`docs/LEGACY_COMPAT.md`)
