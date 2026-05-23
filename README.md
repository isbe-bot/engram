# ENGRAM (Go Core)

Open-source memory intelligence core for AI agents.

## Binaries
- `engramd`: local daemon/service (ingest, curation, retrieval, governance, quality)
- `engramctl`: operator CLI (status, migrate, quality, health, ingest, curate, search, get, correct, deprecate, history)

## Quick start
```bash
make build
make test
./bin/engramd --config ./configs/example.yaml
./bin/engramctl migrate --config ./configs/example.yaml
./bin/engramctl status --config ./configs/example.yaml
./bin/engramctl health --config ./configs/example.yaml
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
- quality metrics endpoint for ingestion freshness, memory counts, audit action counts, and governance rates
