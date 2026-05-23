# ENGRAM (Go Core Scaffold)

Enterprise-ready memory intelligence core for AILEUN.

## Binaries
- `engramd`: local daemon/service (ingest, curation, retrieval, governance, quality)
- `engramctl`: operator CLI (status, migrate, quality, maintenance)

## Quick start
```bash
make build
make test
./bin/engramd --config ./configs/example.yaml
./bin/engramctl status --config ./configs/example.yaml
```

## Project layout
```text
cmd/engramd
cmd/engramctl
internal/{api,config,ingest,curate,retrieve,govern,quality,storage,workers,events,models,migrations}
pkg/contracts
configs
```

This is an initial scaffold aligned with:
- `engram-go-service-blueprint-v1.md`
