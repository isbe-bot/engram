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
- `GET /v1/memory/search?q=<term>&limit=<n>`

### Example ingest + search

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
- migration runner (embedded SQL)
- basic text search over ingested event ledger
- CLI status/migrate commands
