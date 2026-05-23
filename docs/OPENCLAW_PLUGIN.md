# OpenClaw plugin integration

ENGRAM ships a native OpenClaw plugin package under `integrations/openclaw-plugin/`.

The plugin makes ENGRAM available as:

- a memory corpus supplement, so normal `memory_search` can include ENGRAM results;
- explicit tools: `engram_status`, `engram_search`, `engram_get`, and optional `engram_curate`;
- a prompt supplement that tells the agent ENGRAM is a governed, cited memory source;
- HTTP-first access with safe `engramctl` fallback.

## Install from a local checkout

From the ENGRAM repository:

```bash
openclaw plugins install ./integrations/openclaw-plugin
```

Then enable/configure the plugin in OpenClaw config:

```json5
{
  plugins: {
    entries: {
      engram: {
        enabled: true,
        config: {
          endpoint: "http://127.0.0.1:8787",
          // Prefer a read token for search/get. Use write/admin only if you need engram_curate.
          apiKey: "${ENGRAM_READ_API_KEY}",
          configPath: "~/.config/engram/engram.yaml",
          engramctl: "engramctl",
          fallbackToCli: true,
          defaultLimit: 5,
          defaultStatus: "accepted",
          defaultMinConfidence: 0
        }
      }
    }
  }
}
```

Restart OpenClaw after plugin install/config changes.

## Configuration

| Field | Default | Purpose |
| --- | --- | --- |
| `endpoint` | `http://127.0.0.1:8787` | ENGRAM daemon base URL. |
| `apiKey` | env fallback | Bearer token for ENGRAM API. Reads `ENGRAM_API_KEY`, `ENGRAM_READ_API_KEY`, or `ENGRAM_ADMIN_API_KEY` when omitted. |
| `configPath` | `~/.config/engram/engram.yaml` | Config path passed to `engramctl` fallback. |
| `engramctl` | `engramctl` | CLI binary used for fallback. |
| `fallbackToCli` | `true` | Use `engramctl` when HTTP is unavailable or rejects a request. |
| `timeoutMs` | `5000` | HTTP timeout. |
| `defaultLimit` | `5` | Default memory result count. |
| `defaultStatus` | `accepted` | Default memory status filter. |
| `defaultMinConfidence` | `0` | Default confidence threshold. |

## Tools

### `engram_status`

Checks ENGRAM daemon health. If HTTP is unavailable and fallback is enabled, it runs:

```bash
engramctl health --config <configPath>
```

### `engram_search`

Searches governed memory:

```json
{
  "query": "OpenClaw integration",
  "limit": 5,
  "status": "accepted",
  "minConfidence": 0.7,
  "includeEvents": false
}
```

### `engram_get`

Reads a memory object by `object_id`.

### `engram_curate`

Creates a governed memory object. This tool is marked optional in the manifest and requires a write/admin ENGRAM token or a CLI config with write access.

## Safe fallback behavior

The plugin tries HTTP first because it is the normal daemon path and respects API auth/scopes/rate limits. If HTTP fails and `fallbackToCli` is true, the plugin runs `engramctl` with the configured `configPath`.

If both transports fail, the tool returns a clear error instead of silently returning empty memory.

## Security guidance

- Use a read-scoped token for normal recall.
- Enable `engram_curate` only for trusted agents/sessions that should write memory.
- Prefer localhost binding for `engramd` unless you have TLS/reverse-proxy controls.
- Keep `fallbackToCli` enabled only on hosts where `engramctl` and `engram.yaml` are trusted.

## Smoke test

With `engramd` running:

```bash
node integrations/openclaw-plugin/test/smoke-local.js
```

Or run the plugin unit tests:

```bash
node --test integrations/openclaw-plugin/test/*.test.js
```
