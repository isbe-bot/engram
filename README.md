# ENGRAM (Go Core)

Open-source memory intelligence core for AI agents.

ENGRAM is the local memory service layer for long-running AI assistants. It turns scattered chat history, operational events, curated notes, corrections, and project context into governed, searchable, cited memory that an assistant can trust across sessions.

Where a simple script-based memory system is usually a pile of cron jobs, markdown files, Redis buffers, and vector-search helpers, ENGRAM makes memory an operable service: a daemon, a CLI, a versioned schema, an audit trail, retrieval APIs, quality reports, backups, and release gates.

## What ENGRAM is

ENGRAM is a local-first memory intelligence platform composed of:

- **`engramd`** — a daemon that exposes HTTP APIs for ingesting events, curating memory, retrieving cited context, tracking quality, and managing background work.
- **`engramctl`** — an operator CLI for health checks, migrations, search, curation, correction, history, reindexing, backup, restore, and quality reporting.
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
- `engramctl`: operator CLI (status, migrate, quality, report, reindex, backup, restore, health, ingest, curate, search, get, correct, deprecate, history)

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
./bin/engramctl init --config ./engram.yaml --data-dir ./data
./bin/engramd --config ./engram.yaml
./bin/engramctl migrate --config ./engram.yaml
./bin/engramctl status --config ./engram.yaml
./bin/engramctl health --config ./engram.yaml
./bin/engramctl reindex --config ./engram.yaml
./bin/engramctl report --config ./engram.yaml
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
  api_key: ""                 # optional Bearer token; strongly recommended when networked
  max_body_bytes: 1048576     # write-endpoint JSON body limit
  rate_limit_per_minute: 0    # optional per-client limit; 0 disables

storage:
  sqlite_path: "./data/engram.sqlite"
  qdrant_url: "http://127.0.0.1:6333"
  qdrant_collection: "engram_memory"

ingestion:
  max_batch_size: 200
  worker_count: 2
```

Supported environment overrides:

- `ENGRAM_SERVER_BIND`
- `ENGRAM_SERVER_PORT`
- `ENGRAM_API_KEY`
- `ENGRAM_MAX_BODY_BYTES`
- `ENGRAM_RATE_LIMIT_PER_MINUTE`
- `ENGRAM_SQLITE_PATH`
- `ENGRAM_QDRANT_URL`
- `ENGRAM_QDRANT_COLLECTION`
- `ENGRAM_MAX_BATCH_SIZE`
- `ENGRAM_WORKER_COUNT`
- `ENGRAM_QUALITY_EVAL_INTERVAL`

## Production security recommendations

- Keep `server.bind` on `127.0.0.1` unless ENGRAM is intentionally being exposed.
- Set `server.api_key` or `ENGRAM_API_KEY` before putting ENGRAM behind Docker port publishing, a reverse proxy, Tailscale, or any non-local interface.
- Authenticated clients must send `Authorization: Bearer <api_key>`.
- `/v1/health` remains unauthenticated for local liveness checks. All other API endpoints, including `/metrics`, require the token when configured.
- Keep `server.max_body_bytes` low enough for expected memory payloads. The default is 1 MiB.
- Set `server.rate_limit_per_minute` when exposing ENGRAM beyond trusted local automation. `0` disables rate limiting for local sidecars.
- Rotate API keys by changing `server.api_key` or `ENGRAM_API_KEY`, restarting `engramd`, then updating clients. Avoid logging or committing tokens.
- Prefer backup/restore workflows over direct SQLite edits.

## OpenClaw integration example

ENGRAM can be used as OpenClaw's semantic memory sidecar by routing an OpenClaw memory search script through `engramctl`.

A minimal adapter is provided at:

```text
examples/openclaw/mem-search-engram.js
```

Example:

```bash
ENGRAMCTL_BIN=/usr/local/bin/engramctl \
ENGRAM_CONFIG=/etc/engram/engram.yaml \
node examples/openclaw/mem-search-engram.js "project decision" --json --limit 5
```

The adapter preserves ENGRAM's API key handling, citations, confidence filters, and Qdrant-backed recall while keeping OpenClaw integration script-friendly.

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

See `docs/API.md` for a clean API reference and `docs/openapi.yaml` for a compact OpenAPI spec.

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
- CLI status/migrate/quality commands (event + memory counts, freshness, correction/deprecation rates)
- API-backed CLI commands for health, ingest, curate, search, get, correct, deprecate, and history
- operator reindex command to rebuild Qdrant vectors from SQLite memory objects
- backup/restore commands for SQLite operational recovery
- systemd unit, install script, release script, and operator runbook
- worker runtime durability tables (`worker_jobs`, `worker_checkpoints`) with idempotent enqueue semantics
- quality metrics endpoint for ingestion freshness, memory counts, audit action counts, and governance rates
- quality SLO report endpoint (`/v1/quality/report`) and CLI command (`engramctl report`)
- legacy compatibility wrapper script (`scripts/legacy-memory.sh`) and cutover notes (`docs/LEGACY_COMPAT.md`)
