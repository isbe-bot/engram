# Legacy compatibility notes (Milestone 7)

ENGRAM v1 keeps older memory automation scripts functional during cutover.

## Wrapper entrypoint

Use `scripts/legacy-memory.sh` as a drop-in command router:

```bash
./scripts/legacy-memory.sh search --q "handoff"
./scripts/legacy-memory.sh curate --file ./payload.json
```

Behavior:

1. If `engramctl` is present and `engramd` health check succeeds, the wrapper forwards the command to `engramctl`.
2. If ENGRAM is unavailable, wrapper falls back to `LEGACY_MEMORY_SCRIPT` (if configured).
3. If neither path is available, wrapper exits non-zero and prints explicit failure.

## Environment knobs

- `ENGRAM_CONFIG` override config path (default `./configs/example.yaml`)
- `ENGRAMCTL_BIN` override CLI binary path (default `./bin/engramctl`)
- `LEGACY_MEMORY_SCRIPT` fallback executable for old JS memory tooling

## Suggested cutover pattern

1. Keep legacy read/debug scripts in place.
2. Route write/search commands through `legacy-memory.sh`.
3. Monitor `engramctl report` and `/v1/quality/report` for SLO drift.
4. Remove fallback once ENGRAM has stable SLO and parity confidence.

## Fallback expectations

- ENGRAM outage should degrade to legacy script behavior without data-loss from the wrapper layer.
- Operators should investigate ENGRAM health and replay queued jobs before disabling fallback.
