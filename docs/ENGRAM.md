# ENGRAM — Persistent Semantic Memory for ISBE
**Built:** 2026-02-24  
**Author:** ISBE  
**Status:** Production ✅

---

## What Is ENGRAM?

ENGRAM is ISBE's long-term cognitive memory system. It captures every meaningful conversation, extracts facts and decisions, embeds them semantically, and stores them in a vector database — so ISBE can recall context across sessions, days, and weeks, not just within a single chat window.

The name fits: an **engram** is the biological term for the physical trace a memory leaves in the brain. That's exactly what this does — it leaves a permanent, searchable trace of what was discussed.

Three layers work together:

```
Layer 1: Redis       — hot buffer (last 48h of turns, fast write)
Layer 2: Markdown    — daily notes (human-readable, version-controlled)
Layer 3: Qdrant      — long-term semantic memory (vector search, forever)
```

---

## Why We Built It

OpenClaw's built-in `memory_search` is good but limited — it indexes flat markdown files with a 768-dim local embedding model. It has no concept of recency weighting, importance scoring, or structured fact extraction. Every session ISBE wakes up fresh with only what's in those files.

ENGRAM adds:
- **Automatic capture** — no manual note-taking required
- **Intelligent consolidation** — an LLM distills conversations into summaries and facts before storing them
- **Semantic retrieval** — search by meaning, not keywords
- **Importance scoring** — trivial chitchat is filtered out; decisions and context are preserved
- **Portability** — memories live in standard infrastructure (Redis, Qdrant), not locked to OpenClaw

---

## Architecture

```
OpenClaw Session JSONL
~/.openclaw/agents/main/sessions/{uuid}.jsonl
        │
        ▼ (every 5 min, token-free)
  mem-buffer.js
        │
        ▼
  Redis Hot Buffer
  memory:sessions:main:turns  (LIST)
  memory:sessions:main:meta   (HASH — tracks position)
        │
        ▼ (3:00 AM UTC daily)
  mem-consolidate.js
        │
        ├─► qwen3:14b (Mac Mini) ──► summary + facts + topics + importance score
        │
        └─► qwen3-embedding:0.6b (Mac Mini) ──► 1024-dim vector
                │
                ▼
          Qdrant Collections
          ├── isbe_conversations  (chunked session summaries)
          ├── isbe_facts          (extracted facts, decisions, preferences)
          └── isbe_projects       (project context snapshots)
                │
                ▼ (on demand)
          mem-search.js ──► semantic query results
```

---

## Infrastructure

### Redis (Layer 1 — hot buffer)
- **Version:** 8.0.2
- **Bind:** 127.0.0.1:6379 (localhost only — no network exposure)
- **Auth:** `requirepass` (32-byte random hex, stored in `.secrets/redis_password`)
- **Memory cap:** 256MB with `allkeys-lru` eviction policy
- **Persistence:** AOF enabled (survives restarts without losing buffered turns)
- **TTL:** 48h on buffer key (safety net against unbounded growth)
- **Install:** `sudo apt-get install redis-server`
- **Config:** `/etc/redis/redis.conf`

### Qdrant (Layer 3 — vector database)
- **Version:** 1.13.4
- **Bind:** 127.0.0.1:6333 (HTTP), 6334 (gRPC) — localhost only
- **Auth:** `api-key` (32-byte random hex, stored in `.secrets/qdrant_api_key`)
- **Service:** systemd with hardening (`NoNewPrivileges`, `PrivateTmp`, `ProtectSystem`)
- **Storage:** `/var/lib/qdrant/`
- **Install:** binary from GitHub releases → `/usr/local/bin/qdrant`
- **Config:** `/etc/qdrant/config.yaml`
- **Service file:** `/etc/systemd/system/qdrant.service`

### Mac Mini (embeddings + LLM)
- **Address:** `http://100.73.153.121:11434` (Tailscale — encrypted mesh, no public internet)
- **Embedding model:** `qwen3-embedding:0.6b` → 1024-dim vectors
- **Summarization model:** `qwen3:14b` → structured JSON extraction
- **Critical note:** `qwen3:14b` requires `"think": false` in the API call — without it, the model puts all output in a `thinking` field and returns empty `content`. Silent failure with no error.

---

## Files

All scripts live in: `~/.openclaw/workspace/scripts/memory/`

### `config.js`
Shared configuration and client factory. Everything else imports this.

**What it does:**
- Loads secrets from `.secrets/redis_password` and `.secrets/qdrant_api_key` at runtime (never hardcoded)
- Exports `createRedisClient()` — connects and authenticates
- Exports `createQdrantClient()` — connects with API key
- Exports `embed(text)` — calls qwen3-embedding:0.6b, returns float[1024], validates dimensions
- Exports `summarize(text)` — calls qwen3:14b with `think:false`, extracts and validates JSON structure
- Exports `log(file, level, message, data)` — structured logging to file + stdout
- Defines all key constants: Redis keys, Qdrant collection names, paths

**Key design:** secrets loaded lazily via getter properties so they're never in memory until needed.

---

### `mem-init.js`
One-time setup script. Safe to re-run (idempotent).

**What it does:**
1. Connects to Redis — verifies ping
2. Connects to Qdrant — verifies collections API
3. Tests embedding endpoint — sends probe text, validates 1024-dim response
4. Creates three Qdrant collections (skips if already exist):
   - `isbe_conversations` — 1024-dim, Cosine distance
   - `isbe_facts` — 1024-dim, Cosine distance
   - `isbe_projects` — 1024-dim, Cosine distance
5. Verifies all collections are green
6. Checks Redis namespace state

**Run once:** `node scripts/memory/mem-init.js`

---

### `mem-buffer.js`
Token-free session capture. Runs every 5 minutes via cron.

**What it does:**
1. Reads `sessions.json` to find the active session file path
2. Connects to Redis, checks `memory:sessions:main:meta` for last processed line index
3. If session ID changed (compaction/rotation) — resets line tracker
4. Reads only new lines from the JSONL since last run
5. Filters each entry:
   - Must be `type: "message"` with `role: "user"` or `role: "assistant"`
   - Assistant turns must have `stopReason: "endTurn"` (skips tool-call-only turns)
   - Text must pass noise filter (skips heartbeats, system prompts, `NO_REPLY`, short messages)
6. Pushes valid turns to `memory:sessions:main:turns` Redis LIST
7. Updates `memory:sessions:main:meta` with current line count and timestamp
8. Sets 48h TTL on the turns key

**Why token-free:** runs as a shell exec, never as an LLM turn. No context window consumption.

**Noise patterns filtered:**
- Session startup messages
- `Read HEARTBEAT.md if it exists...`
- `HEARTBEAT_OK`
- `NO_REPLY`
- `[System Message]` blocks
- Any text under 20 characters

**Run:** `node scripts/memory/mem-buffer.js [--agent main]`

---

### `mem-consolidate.js`
The intelligent nightly consolidation engine.

**What it does:**
1. Pulls all turns from Redis buffer (LRANGE 0 -1, then reverses to chronological order)
2. Chunks turns into overlapping context windows (10 turns per chunk, 2-turn overlap)
3. For each chunk:
   - Formats as `[USER] text\n\n[ASSISTANT] text` conversation block
   - Sends to `qwen3:14b` with `think:false` for structured extraction
   - LLM returns JSON: `{ summary, facts[], topics[], importance }`
   - Skips chunks with importance < 3/10
   - Embeds the summary via `qwen3-embedding:0.6b` (1024-dim)
   - Upserts into `isbe_conversations` collection
4. Collects all extracted facts, embeds each individually, stores in `isbe_facts`
5. Clears Redis buffer **only if zero failures** (partial failure = retry tomorrow)
6. Logs full run summary to `/var/log/mem-consolidate.log`

**LLM system prompt (summary):** Extract summary (2-3 sentences), facts (concrete decisions/preferences), topics (2-6 tags), importance (1-10). Focus on technical decisions, project context, user preferences, lessons learned. Ignore system noise.

**Dry run mode:** `node scripts/memory/mem-consolidate.js --dry-run`  
Runs the full LLM summarization pass but does not write to Qdrant or clear Redis.

**Run:** `node scripts/memory/mem-consolidate.js [--dry-run] [--agent main]`

---

### `mem-search.js`
Semantic search interface across all Qdrant collections.

**What it does:**
1. Embeds the query via `qwen3-embedding:0.6b`
2. Searches target collections (all three by default, or filtered by `--collection`)
3. Merges and re-ranks results by score descending
4. Outputs human-readable results with match %, collection type, date, summary/fact text, topics, importance stars

**Output format (conversations):**
```
[67.0% match] [conversations] (2/24/2026)
  📝 Summary text...
  🏷️  topic1, topic2
  Facts:
    • fact 1
    • fact 2
  Importance: ★★★★★★★★★★
```

**Output format (facts):**
```
[76.1% match] [facts] (2/24/2026)
  📌 The fact text
  🏷️  topic1, topic2
  Importance: ★★★★★★★★★★
```

**Options:**
```bash
node mem-search.js "query"
node mem-search.js "query" --collection facts|conversations|projects
node mem-search.js "query" --limit 10
node mem-search.js "query" --min-score 0.6
node mem-search.js "query" --json          # machine-readable output
```

---

### `mem-status.js`
One-command health dashboard across all three memory layers.

**What it checks:**
- Redis: ping, version, memory usage, buffered turn count, last run, last flush
- Daily markdown files: count, most recent files
- Qdrant: connection, all collections with point counts and status
- Mac Mini: reachability, both models loaded

**Run:** `node scripts/memory/mem-status.js`

---

## Cron Jobs

Registered in OpenClaw's cron system (not system crontab):

| Name | Schedule | What it does |
|------|----------|-------------|
| `mem-buffer` | `*/5 * * * *` | Captures session turns to Redis (token-free, isolated session) |
| `mem-consolidate` | `0 3 * * * UTC` | Nightly Redis → Qdrant consolidation (announces result) |

---

## Qdrant Collections Schema

### `isbe_conversations`
```json
{
  "vector": [1024 floats — embedding of summary],
  "payload": {
    "agent": "main",
    "session_id": "uuid",
    "summary": "2-3 sentence description of the conversation",
    "topics": ["tag1", "tag2"],
    "importance": 8,
    "facts": ["fact1", "fact2"],
    "turn_count": 10,
    "from_ts": "2026-02-24T08:42:00Z",
    "to_ts": "2026-02-24T09:05:00Z",
    "created_at": "2026-02-24T09:13:00Z"
  }
}
```

### `isbe_facts`
```json
{
  "vector": [1024 floats — embedding of fact text],
  "payload": {
    "agent": "main",
    "text": "The fact as a sentence",
    "topics": ["tag1", "tag2"],
    "importance": 8,
    "from_ts": "2026-02-24T08:42:00Z",
    "session_id": "uuid",
    "created_at": "2026-02-24T09:13:00Z"
  }
}
```

---

## Security Posture

| Control | Implementation |
|---------|---------------|
| Redis network | Bound to 127.0.0.1 only |
| Redis auth | `requirepass` with 32-byte random hex |
| Qdrant network | Bound to 127.0.0.1 only |
| Qdrant auth | `api-key` with 32-byte random hex |
| Qdrant process | systemd: NoNewPrivileges, PrivateTmp, ProtectSystem=strict |
| Secrets storage | `~/.openclaw/workspace/.secrets/` with chmod 600 |
| Secrets in code | Never hardcoded — loaded from files at runtime |
| Secrets in logs | Never logged — redacted by design |
| External network | Mac Mini calls go over Tailscale (encrypted mesh) only |
| Cron user | Runs as `rico`, not root |

---

## Gotchas & Lessons Learned

### 1. `qwen3:14b` requires `think: false`
Without this flag, the model routes all output to a `thinking` field and returns empty `content`. No error is thrown. The consolidation would silently store nothing. **Always pass `think: false` in the API call.**

### 2. Qdrant `vectors_count` vs `points_count`
`vectors_count` returns 0 until the optimizer indexes the segment (can take minutes). Use `points_count` for accurate real-time counts in status checks.

### 3. Session JSONL has many tool-only assistant turns
In a typical session, ~80% of assistant messages are tool calls (`stopReason: "toolUse"`). Only filter for `stopReason: "endTurn"` to get actual responses.

### 4. Qdrant point IDs must be uint or UUID format
Hex strings (e.g. SHA256 slice) are not valid Qdrant IDs. The consolidation script handles this with a numeric fallback (`index + Date.now()`).

### 5. Redis AOF is off by default
Without `appendonly yes`, a Redis restart loses the entire buffer. Enabled this proactively during setup.

### 6. Session compaction
OpenClaw rotates session JSONL files periodically. The buffer script tracks `session_id` + `line_count` so it detects rotation and resets gracefully rather than re-processing old data or crashing.

---

## Day-to-Day Usage

**Search past memory:**
```bash
node ~/.openclaw/workspace/scripts/memory/mem-search.js "your topic"
```

**Force immediate capture (don't wait 5 min):**
```bash
node ~/.openclaw/workspace/scripts/memory/mem-buffer.js
```

**Force immediate consolidation (don't wait for 3 AM):**
```bash
node ~/.openclaw/workspace/scripts/memory/mem-consolidate.js
```

**Preview what would be consolidated (no writes):**
```bash
node ~/.openclaw/workspace/scripts/memory/mem-consolidate.js --dry-run
```

**Check system health:**
```bash
node ~/.openclaw/workspace/scripts/memory/mem-status.js
```

---

## Future Ideas

- **`isbe_projects` collection** — currently created but not yet populated. Add a `save project` command that captures current project state (name, stack, status, key decisions) into this collection.
- **`mem-search` integration with `memory_search` tool** — today both systems run independently. Could bridge them so a single `memory_search` query checks both sqlite-vec and Qdrant.
- **Importance decay** — lower the effective importance of old facts over time so recent context ranks higher.
- **Per-agent memory** — Carmack, Picasso, and Chief each have their own session dirs. Could buffer and consolidate their sessions separately into agent-specific collections.
- **Memory deduplication** — before storing a fact, check if a semantically similar one already exists (score > 0.95) and skip or merge.
