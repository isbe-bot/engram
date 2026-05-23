# ENGRAM v1 Operator Runbook

## Install

```bash
make build
sudo ./scripts/install.sh
sudoedit /etc/engram/engram.yaml
sudo systemctl daemon-reload
sudo systemctl enable --now engramd
```

Validate:

```bash
engramctl health --config /etc/engram/engram.yaml
engramctl status --config /etc/engram/engram.yaml
engramctl quality --config /etc/engram/engram.yaml
engramctl retention --config /etc/engram/engram.yaml
```

## Upgrade

```bash
git pull
make build
sudo ./scripts/install.sh
sudo systemctl restart engramd
engramctl health --config /etc/engram/engram.yaml
```

## Reindex Qdrant

Run after Qdrant collection changes, embedding changes, or restore:

```bash
engramctl reindex --config /etc/engram/engram.yaml
```

## Retention and compaction

Always review the report before applying cleanup:

```bash
engramctl retention --config /etc/engram/engram.yaml
```

If the report is acceptable, compact explicit cleanup candidates:

```bash
engramctl compact --config /etc/engram/engram.yaml --apply
```

`compact --apply` deletes old raw ingested events and old deprecated memory objects only. Accepted stale memory is review-only and must be corrected/deprecated by a governed workflow before it can become a delete candidate.

## Backup

```bash
engramctl backup --config /etc/engram/engram.yaml --out /srv/aileun/backups/engram-$(date -u +%Y%m%dT%H%M%SZ).sqlite.bak
```

## Restore

Stop the service first to avoid SQLite write races:

```bash
sudo systemctl stop engramd
engramctl restore --config /etc/engram/engram.yaml --from /srv/aileun/backups/engram.sqlite.bak
engramctl reindex --config /etc/engram/engram.yaml
sudo systemctl start engramd
engramctl health --config /etc/engram/engram.yaml
```

`restore` creates a pre-restore copy of the current SQLite file when one exists.

## Incident triage

1. Check service:

```bash
systemctl status engramd --no-pager
journalctl -u engramd -n 200 --no-pager
```

2. Check health and quality:

```bash
engramctl health --config /etc/engram/engram.yaml
engramctl quality --config /etc/engram/engram.yaml
```

3. Check Qdrant collection:

```bash
curl -fsS http://127.0.0.1:6333/collections
```

4. If semantic results are missing but SQLite memory exists, run `engramctl reindex`.

## Rollback

1. Stop service.
2. Replace binaries with prior release artifacts.
3. Restore SQLite backup if schema/data rollback is required.
4. Run health checks before reopening traffic.
