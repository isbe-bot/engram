# ENGRAM v1 Development Plan

Status: active
Target: v1.0.0 production-ready local memory service for AILEUN/OpenClaw client VPS deployments.

## Release definition

ENGRAM v1 is shippable when a VPS can install `engramd` + `engramctl`, ingest memory events, curate governed memory objects, retrieve cited memory through hybrid search, survive restarts, expose quality/health signals, and pass backup/restore and CI release gates.

## Non-negotiables

- No memory write without source refs.
- No retrieval response without confidence and citations.
- No schema change without migration.
- No silent deprecations.
- No sensitive data persistence without explicit policy.
- All worker operations must be idempotent and checkpointed.
- All release claims require green local gates and green GitHub CI.

## Milestone 0 — Current baseline

Complete:

- Go repo scaffold.
- `engramd` and `engramctl` binaries.
- SQLite store and migrations.
- Event ingest API.
- Memory curate/search/correct/deprecate/history APIs.
- Governance rules for source refs, classifications, correction/deprecation reasons, high-confidence force gates.
- Hash-chain audit event trail.
- Quality metrics API and CLI command.
- HTTP integration tests.
- CI gates for fmt, tidy, vet, test, race, staticcheck, govulncheck, build.

## Milestone 1 — Complete local API + CLI surface

Goal: make v1 service operable without custom curl calls.

Tasks:

- Add `GET /v1/memory/{id}`.
- Add API tests for direct fetch and not-found behavior.
- Add API-backed `engramctl` commands:
  - `health`
  - `ingest`
  - `curate`
  - `search`
  - `get`
  - `correct`
  - `deprecate`
  - `history`
- Keep existing direct local commands:
  - `migrate`
  - `status`
  - `quality`
- Add machine-readable JSON output for API commands.
- Update README examples.

Exit gates:

- `go test ./...`
- `go vet ./...`
- `staticcheck ./...`
- `govulncheck ./...`
- `go test -race ./...`
- `make build`
- GitHub CI green.

## Milestone 2 — Contracts, schemas, and ledger completeness

Goal: formalize memory state beyond the bootstrap tables.

Tasks:

- Enforce canonical memory object taxonomy:
  - `fact`
  - `preference`
  - `decision`
  - `project_context`
  - `contact_context`
  - `workflow_pattern`
  - `lesson`
  - `report_insight`
- Add `scope` and `provenance_hash` to memory objects through migration.
- Add tables:
  - `memory_candidates`
  - `knowledge_links`
  - `pattern_signals`
  - `improvement_suggestions`
  - `adaptation_actions`
  - `adaptation_verifications`
  - `evolution_reports`
- Add schema migration tests and fixture migration from empty DB.
- Add import/export format for memory objects.

Exit gates:

- Migration tests cover fresh DB and upgraded DB.
- Contract tests reject invalid taxonomy/scope/source/classification.

## Milestone 3 — Safety and governance hardening

Goal: prevent unsafe memory writes by default.

Tasks:

- Add secret detection for ingest and curate payloads.
- Add basic PII/sensitive data classifier.
- Add explicit classification policy for public/internal/confidential/secret-like data.
- Reject secret-like persistence unless a future policy explicitly permits it.
- Add signed mutation envelope verifier abstraction.
- Add role/read-scope model for future multi-actor access.
- Add audit entries for policy rejections.

Exit gates:

- Tests for common tokens/secrets/passwords/API keys/private keys.
- Tests for allowed vs rejected classifications.
- No raw secret fixtures committed.

## Milestone 4 — Qdrant semantic index + hybrid retrieval

Goal: make ENGRAM real semantic memory, not just SQLite text search.

Tasks:

- Implement Qdrant client adapter.
- Implement embedding provider abstraction.
- Add index document model with deterministic vector IDs.
- Add embedding/index worker.
- Add hybrid retrieval:
  - vector similarity
  - SQLite metadata filters
  - recency weighting
  - confidence weighting
- Add retrieval diagnostics:
  - rank source
  - score components
  - citation coverage
  - confidence distribution
- Add Qdrant integration tests, preferably testcontainer/local optional gate.

Exit gates:

- Search returns cited semantic hits for non-exact terms.
- Retrieval p95 baseline measured locally.
- Tests run without Qdrant by default and with Qdrant when enabled.

## Milestone 5 — Worker runtime and reliability

Goal: durable autonomous processing.

Tasks:

- Implement worker manager lifecycle with graceful shutdown.
- Add durable job/checkpoint tables.
- Implement workers:
  - ingest worker
  - curation worker
  - embedding/index worker
  - consolidation worker
  - quality eval worker
  - cleanup/retention worker
- Add retry/dead-letter state.
- Add idempotency keys and checkpoint replay tests.

Exit gates:

- Worker restart/replay tests pass.
- Failed jobs move to dead-letter with reason.
- No duplicate memory/index writes on replay.

## Milestone 6 — Quality evaluation and observability

Goal: v1 can prove memory quality is measurable.

Tasks:

- Add precision@k evaluation harness with fixtures.
- Add historical replay fixture support.
- Add SLO metrics:
  - ingest lag p95
  - retrieval p95
  - consolidation freshness
  - correction apply latency
  - stale-memory rate
- Add `/v1/quality/report` or CLI report command.
- Add OpenTelemetry-compatible metrics path or structured metrics export.

Exit gates:

- Quality report generated from fixture corpus.
- SLO values visible from CLI/API.

## Milestone 7 — Legacy script compatibility and cutover

Goal: OpenClaw memory scripts can route through ENGRAM without breaking current workflows.

Tasks:

- Add compatibility wrappers for existing JS memory scripts.
- Route new writes through `engramd` where possible.
- Add old-vs-new parity tests for search output shape.
- Keep read-only debug scripts during transition.
- Document fallback behavior if `engramd` is unavailable.

Exit gates:

- Existing memory workflows still pass.
- ENGRAM-backed search returns equivalent or better cited results.

## Milestone 8 — Production packaging and release

Goal: installable, operable v1.

Tasks:

- Add systemd service unit.
- Add install/upgrade script.
- Add config template and validation command.
- Add backup/restore commands.
- Add SQLite integrity check command.
- Add release build script with checksums.
- Add release notes.
- Add operator runbook:
  - install
  - upgrade
  - rollback
  - backup/restore
  - incident triage
- Tag `v1.0.0` after all gates pass.

Exit gates:

- Fresh install smoke test passes.
- Upgrade smoke test passes.
- Backup/restore smoke test passes.
- GitHub CI green.
- Release artifacts generated and verified.

## Iteration log

Update this section after each completed implementation slice.

### 2026-05-23 — Milestone 1 slice A

Completed:

- Added v1 development/release plan.
- Added `GET /v1/memory/{object_id}`.
- Added API integration coverage for direct fetch and not-found behavior.
- Added API-backed `engramctl` commands: `health`, `ingest`, `curate`, `search`, `get`, `correct`, `deprecate`, and `history`.
- Updated README with API and CLI usage.

Verification:

- CLI smoke with live `engramd` passed for health, ingest, curate, get, search, correct, history, and quality.
- Local gates passed: formatting, `go mod tidy`, `git diff --check`, `go test ./...`, `go vet ./...`, `staticcheck ./...`, `govulncheck ./...`, `go test -race ./...`, `make build`.

### 2026-05-23 — Milestone 2 slice A

Completed:

- Added canonical memory type validation for `fact`, `preference`, `decision`, `project_context`, `contact_context`, `workflow_pattern`, `lesson`, and `report_insight`.
- Added scope validation with `local`, `agent`, `project`, `client`, and `global`.
- Added migration `0004_memory_contract_fields.sql` for `scope` and `provenance_hash`.
- Added deterministic SHA-256 provenance hash generation for memory objects.
- Updated SQLite create/get/search/correct paths to persist and return contract fields.
- Added tests for contract validation, provenance hashing, and migration-created contract columns.
- Updated README examples to include valid source refs and governance envelopes.

Verification:

- Local gates passed: formatting, `go mod tidy`, `git diff --check`, `go test ./...`, `go vet ./...`, `staticcheck ./...`, `govulncheck ./...`, `go test -race ./...`, `make build`.


### 2026-05-23 — Milestone 3 slice A

Completed:

- Added basic secret-like content detector for private key headers, common provider token shapes, AWS access keys, Slack tokens, OpenAI-style API keys, GitHub token shapes, and generic secret assignments.
- Enforced secret-like rejection on event ingest data.
- Enforced secret-like rejection on memory curation content.
- Enforced secret-like rejection on correction content.
- Added tests for policy detection, event rejection, memory object rejection, and API ingest rejection.

Verification:

- Local gates passed: formatting, `go mod tidy`, `git diff --check`, `go test ./...`, `go vet ./...`, `staticcheck ./...`, `govulncheck ./...`, `go test -race ./...`, `make build`.
