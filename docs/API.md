# ENGRAM API Reference

Base URL defaults to `http://127.0.0.1:8787`.

## Authentication

`GET /v1/health` is intentionally unauthenticated for local liveness checks.

All other endpoints require this header when `server.api_key` / `ENGRAM_API_KEY` is configured:

```http
Authorization: Bearer <api_key>
```

Unauthorized responses are JSON:

```json
{"error":"unauthorized","message":"missing or invalid bearer token"}
```

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/v1/health` | Liveness and SQLite health. |
| `POST` | `/v1/events/ingest` | Ingest an operational event. |
| `POST` | `/v1/memory/curate` | Create an accepted memory object. |
| `GET` | `/v1/memory/{object_id}` | Fetch one memory object. |
| `POST` | `/v1/memory/{object_id}/correct` | Correct a memory object with audit history. |
| `POST` | `/v1/memory/{object_id}/deprecate` | Deprecate a memory object. |
| `GET` | `/v1/memory/{object_id}/history` | Return memory audit events. |
| `GET` | `/v1/memory/search` | Search curated memory and optionally raw ingested events. |
| `GET` | `/v1/quality/metrics` | JSON quality snapshot. |
| `GET` | `/v1/quality/report` | JSON SLO-oriented quality report. |
| `GET` | `/metrics` | Prometheus-style text metrics. |

## Write limits

JSON write endpoints are limited by `server.max_body_bytes` / `ENGRAM_MAX_BODY_BYTES`.
Oversized bodies return:

```json
{"error":"request body too large"}
```

## Search query parameters

`GET /v1/memory/search`

| Parameter | Description |
|---|---|
| `q` | Search text. |
| `status` | Optional memory status filter, e.g. `accepted` or `deprecated`. |
| `min_confidence` | Minimum memory confidence from `0` to `1`. |
| `limit` | Result limit. |
| `cursor` | Offset cursor returned by prior search. |
| `include_events` | Include raw ingested event hits when `true`. |

Search results include deterministic `citations`, `rank_score`, and `retrieval_source`.

## Memory write request

`POST /v1/memory/curate`

```json
{
  "object_id": "mem-1",
  "type": "decision",
  "content": "Use ENGRAM for governed long-term memory.",
  "source_refs": ["adr:0001"],
  "confidence": 0.85,
  "classification": "product",
  "scope": "local",
  "envelope": {
    "actor_id": "operator",
    "mutation_id": "mut-001",
    "signature": "sig-001"
  }
}
```

Allowed source ref prefixes are `adr:`, `chat:`, `spec:`, `meeting:`, `task:`, `event:`, and `doc:`.

## OpenAPI

A compact OpenAPI document is provided at `docs/openapi.yaml`.
