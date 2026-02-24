# ENGRAM

> Persistent semantic memory for OpenClaw AI agents.

An engram is the physical trace a memory leaves in the brain. This system leaves a permanent, searchable trace of every meaningful AI conversation — so your agent actually remembers.

## Architecture

```
OpenClaw Session JSONL
        │
        ▼ (every 5 min, token-free)
  mem-buffer.js ──► Redis Hot Buffer (48h TTL)
        │
        ▼ (3:00 AM daily)
  mem-consolidate.js
        ├─► qwen3:14b ──► summary + facts + topics + importance score
        └─► qwen3-embedding:0.6b ──► 1024-dim vectors
                │
                ▼
          Qdrant Collections
          ├── isbe_conversations
          ├── isbe_facts
          └── isbe_projects
                │
                ▼
          mem-search.js ──► semantic query results
```

## Prerequisites

| Service | Purpose | Install |
|---------|---------|---------|
| Redis | Hot buffer | `sudo apt install redis-server` |
| Qdrant | Vector database | [Binary release](https://github.com/qdrant/qdrant/releases) |
| Ollama | Embeddings + LLM | [ollama.ai](https://ollama.ai) |

**Ollama models needed:**
```bash
ollama pull qwen3-embedding:0.6b   # 1024-dim embeddings
ollama pull qwen3:14b              # summarization (or any capable model)
```

## Installation

```bash
git clone https://github.com/isbe-bot/engram.git
cd engram
npm install
```

**Create secrets:**
```bash
mkdir -p .secrets
echo "your-redis-password" > .secrets/redis_password
echo "your-qdrant-api-key" > .secrets/qdrant_api_key
chmod 600 .secrets/*
```

**Configure environment** (optional overrides):
```bash
cp .memory_env.template .memory_env
# Edit .memory_env with your host addresses
```

**Initialize Qdrant collections:**
```bash
npm run init
```

## Usage

```bash
# Search memory
npm run search -- "your query here"
node scripts/mem-search.js "Redis Qdrant architecture"

# Capture session turns to buffer
npm run buffer

# Consolidate buffer → Qdrant (normally runs via cron at 3 AM)
npm run consolidate

# Preview consolidation without writing
node scripts/mem-consolidate.js --dry-run

# System health check
npm run status
```

## Cron Setup

```bash
# Capture turns every 5 minutes (token-free)
*/5 * * * * node /path/to/engram/scripts/mem-buffer.js

# Nightly consolidation at 3 AM UTC
0 3 * * * node /path/to/engram/scripts/mem-consolidate.js
```

## Configuration

All config in `scripts/config.js`. Override via environment variables or `.memory_env`:

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_HOST` | `127.0.0.1` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `QDRANT_URL` | `http://127.0.0.1:6333` | Qdrant URL |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama host |
| `EMBED_MODEL` | `qwen3-embedding:0.6b` | Embedding model |
| `LLM_MODEL` | `qwen3:14b` | Summarization model |
| `EMBED_DIMS` | `1024` | Vector dimensions |

## Security

- Redis and Qdrant bound to `127.0.0.1` by default — no network exposure
- Secrets loaded from files at runtime, never hardcoded or logged
- Qdrant systemd service hardened with `NoNewPrivileges`, `PrivateTmp`, `ProtectSystem`

## Full Documentation

See [`docs/ENGRAM.md`](docs/ENGRAM.md) for complete architecture, gotchas, and build notes.

## Built With

- [Redis](https://redis.io) — hot buffer
- [Qdrant](https://qdrant.tech) — vector database
- [Ollama](https://ollama.ai) — local LLM + embeddings
- [OpenClaw](https://openclaw.ai) — AI agent platform

## License

MIT — built by [ISBE](https://github.com/isbe-bot) for [OpenClaw](https://openclaw.ai).
