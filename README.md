# ENGRAM (Go Core)

Open-source memory intelligence core for AI agents.

## Binaries
- `engramd`: local daemon/service (ingest, curation, retrieval, governance, quality)
- `engramctl`: operator CLI (status, migrate, quality, maintenance)

## Quick start
```bash
make build
make test
./bin/engramd --config ./configs/example.yaml
./bin/engramctl migrate --config ./configs/example.yaml
./bin/engramctl status --config ./configs/example.yaml
```

## Current API (v1 bootstrap)

- `GET /v1/health`
- `POST /v1/events/ingest`
- `POST /v1/memory/curate`
- `POST /v1/memory/{object_id}/correct`
- `POST /v1/memory/{object_id}/deprecate`
- `GET /v1/memory/search?q=<term>&limit=<n>`

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
    "confidence":0.95,
    "classification":"product"
  }'

curl -s -X POST http://127.0.0.1:8787/v1/memory/mem-1/correct \
  -H 'content-type: application/json' \
  -d '{
    "content":"Use Go + SQLite for ENGRAM core",
    "reason":"more precise",
    "source_refs":["adr:0009","blueprint:v1"]
  }'

curl -s -X POST http://127.0.0.1:8787/v1/memory/mem-1/deprecate \
  -H 'content-type: application/json' \
  -d '{"reason":"superseded by revised architecture"}'

curl -s 'http://127.0.0.1:8787/v1/memory/search?q=Kickoff&limit=10'
```

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
- curation and governance endpoints (`curate`, `correct`, `deprecate`)
- basic text search over ingested event ledger
- CLI status/migrate commands (event + memory counts)
