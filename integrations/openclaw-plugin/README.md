# @isbe/openclaw-engram

Native OpenClaw plugin for ENGRAM governed memory.

See [`../../docs/OPENCLAW_PLUGIN.md`](../../docs/OPENCLAW_PLUGIN.md) for install/configuration details.

## Capabilities

- Registers ENGRAM as a memory corpus supplement.
- Adds prompt guidance for governed/cited ENGRAM memory.
- Registers tools:
  - `engram_status`
  - `engram_search`
  - `engram_get`
  - `engram_curate` optional write tool
- Uses HTTP first with safe `engramctl` fallback.

## Local test

```bash
node --test test/*.test.js
```
