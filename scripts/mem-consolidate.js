'use strict';
/**
 * mem-consolidate.js — Nightly Redis → LLM summarize → embed → Qdrant
 *
 * Pulls all buffered turns from Redis, groups them into context chunks,
 * runs each through qwen3:14b for summarization + fact extraction,
 * embeds via qwen3-embedding:0.6b, and upserts into Qdrant.
 * Clears Redis buffer after successful flush.
 *
 * Usage: node mem-consolidate.js [--dry-run] [--agent main]
 * Dry run: summarizes but does not write to Qdrant or clear Redis.
 */

process.chdir('/home/rico/.openclaw/workspace');
const crypto = require('crypto');
const { CONFIG, createRedisClient, createQdrantClient, embed, summarize, log } = require('./config');

const LOG   = CONFIG.memory.consolidateLog;
const info  = (msg, d) => log(LOG, 'INFO',  msg, d);
const warn  = (msg, d) => log(LOG, 'WARN',  msg, d);
const error = (msg, d) => log(LOG, 'ERROR', msg, d);
const ok    = (msg, d) => log(LOG, 'OK',    msg, d);

// ── Args ──────────────────────────────────────────────────────────────────────
const args   = process.argv.slice(2);
const AGENT  = args.includes('--agent')   ? args[args.indexOf('--agent') + 1]   : 'main';
const DRY    = args.includes('--dry-run');

// ── Constants ─────────────────────────────────────────────────────────────────
const TURNS_PER_CHUNK    = 10;   // How many turns per LLM context window
const MIN_IMPORTANCE     = 3;    // Skip chunks scored below this
const MAX_FACTS_PER_CHUNK = 8;   // Cap facts stored per chunk

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Stable ID for a chunk based on content hash */
function chunkId(text) {
  return crypto.createHash('sha256').update(text).digest('hex').slice(0, 16);
}

/** Format turns array into a readable conversation block for the LLM */
function formatChunk(turns) {
  return turns.map(t => `[${t.role.toUpperCase()}] ${t.text}`).join('\n\n');
}

/** Split flat turns array into overlapping context windows */
function chunkTurns(turns, size = TURNS_PER_CHUNK, overlap = 2) {
  const chunks = [];
  for (let i = 0; i < turns.length; i += size - overlap) {
    const slice = turns.slice(i, i + size);
    if (slice.length >= 2) chunks.push(slice);  // need at least 2 turns
    if (i + size >= turns.length) break;
  }
  return chunks;
}

// ── Main ──────────────────────────────────────────────────────────────────────
async function main() {
  info(`=== mem-consolidate.js starting ===`, { agent: AGENT, dry_run: DRY });

  let redis, qdrant;

  // ── 1. Connect ────────────────────────────────────────────────────────────
  try {
    redis  = await createRedisClient();
    qdrant = createQdrantClient();
    await redis.ping();
    ok('Connections established');
  } catch (err) {
    error('Connection failed', { message: err.message });
    process.exit(1);
  }

  // ── 2. Pull turns from Redis ──────────────────────────────────────────────
  const turnsKey = CONFIG.memory.turnsKey(AGENT);
  const metaKey  = CONFIG.memory.metaKey(AGENT);
  const rawCount = await redis.lLen(turnsKey);

  if (rawCount === 0) {
    info('Redis buffer empty — nothing to consolidate');
    await redis.quit();
    process.exit(0);
  }

  info(`Pulling ${rawCount} turns from Redis buffer`);

  // LRANGE 0 -1 returns all elements; Redis LIST is LIFO so we reverse
  const rawItems = await redis.lRange(turnsKey, 0, -1);
  const turns = rawItems
    .map(item => { try { return JSON.parse(item); } catch { return null; } })
    .filter(Boolean)
    .reverse();  // oldest first

  info(`Parsed ${turns.length} valid turns`);

  // ── 3. Chunk into context windows ─────────────────────────────────────────
  const chunks = chunkTurns(turns);
  info(`Chunked into ${chunks.length} context windows`, { turns_per_chunk: TURNS_PER_CHUNK });

  // ── 4. Process each chunk ─────────────────────────────────────────────────
  let stored = 0, skipped = 0, failed = 0;
  const allFacts = [];

  for (let i = 0; i < chunks.length; i++) {
    const chunk = chunks[i];
    const chunkText = formatChunk(chunk);
    const cid = chunkId(chunkText);
    const firstTs = chunk[0].timestamp;
    const lastTs  = chunk[chunk.length - 1].timestamp;

    info(`Processing chunk ${i + 1}/${chunks.length}`, {
      id: cid,
      turns: chunk.length,
      from: firstTs,
    });

    // ── 4a. LLM Summarization ───────────────────────────────────────────────
    let extracted;
    try {
      extracted = await summarize(chunkText);
    } catch (err) {
      error(`Summarization failed for chunk ${i + 1}`, { message: err.message });
      failed++;
      continue;
    }

    info(`Summarized chunk ${i + 1}`, {
      importance: extracted.importance,
      topics:     extracted.topics,
      facts_count: extracted.facts.length,
    });

    // Skip low-importance chunks
    if (extracted.importance < MIN_IMPORTANCE) {
      warn(`Skipping low-importance chunk`, {
        id: cid,
        importance: extracted.importance,
        summary_preview: extracted.summary.slice(0, 80),
      });
      skipped++;
      continue;
    }

    if (DRY) {
      ok(`[DRY RUN] Would store chunk`, {
        id: cid,
        importance: extracted.importance,
        summary: extracted.summary.slice(0, 100),
        facts: extracted.facts.slice(0, 3),
      });
      stored++;
      continue;
    }

    // ── 4b. Embed the summary ───────────────────────────────────────────────
    let vector;
    try {
      vector = await embed(extracted.summary);
    } catch (err) {
      error(`Embedding failed for chunk ${i + 1}`, { message: err.message });
      failed++;
      continue;
    }

    // ── 4c. Upsert into isbe_conversations ─────────────────────────────────
    try {
      await qdrant.upsert(CONFIG.memory.collections.conversations, {
        wait: true,
        points: [{
          id:     cid,  // stable content hash ID (hex → Qdrant wants UUID format)
          vector,
          payload: {
            agent:        AGENT,
            session_id:   chunk[0].session_id || '',
            summary:      extracted.summary,
            topics:       extracted.topics,
            importance:   extracted.importance,
            facts:        extracted.facts.slice(0, MAX_FACTS_PER_CHUNK),
            turn_count:   chunk.length,
            from_ts:      firstTs,
            to_ts:        lastTs,
            created_at:   new Date().toISOString(),
          },
        }],
      });
      stored++;
    } catch (err) {
      // Qdrant requires UUID-format IDs — use name-based UUID v5
      try {
        // Fallback: use numeric ID from index
        await qdrant.upsert(CONFIG.memory.collections.conversations, {
          wait: true,
          points: [{
            id:     i + Date.now(),
            vector,
            payload: {
              agent:        AGENT,
              chunk_hash:   cid,
              session_id:   chunk[0].session_id || '',
              summary:      extracted.summary,
              topics:       extracted.topics,
              importance:   extracted.importance,
              facts:        extracted.facts.slice(0, MAX_FACTS_PER_CHUNK),
              turn_count:   chunk.length,
              from_ts:      firstTs,
              to_ts:        lastTs,
              created_at:   new Date().toISOString(),
            },
          }],
        });
        stored++;
      } catch (err2) {
        error(`Qdrant upsert failed for chunk ${i + 1}`, { message: err2.message });
        failed++;
        continue;
      }
    }

    // Collect facts for batch fact storage
    if (extracted.facts.length > 0) {
      allFacts.push(...extracted.facts.slice(0, MAX_FACTS_PER_CHUNK).map(f => ({
        text: f,
        topics: extracted.topics,
        importance: extracted.importance,
        from_ts: firstTs,
        session_id: chunk[0].session_id || '',
      })));
    }
  }

  // ── 5. Store extracted facts into isbe_facts ──────────────────────────────
  if (!DRY && allFacts.length > 0) {
    info(`Storing ${allFacts.length} extracted facts`);
    let factsStored = 0;

    for (let i = 0; i < allFacts.length; i++) {
      const fact = allFacts[i];
      try {
        const factVector = await embed(fact.text);
        await qdrant.upsert(CONFIG.memory.collections.facts, {
          wait: true,
          points: [{
            id:     i + Date.now() + 1_000_000,
            vector: factVector,
            payload: {
              agent:      AGENT,
              text:       fact.text,
              topics:     fact.topics,
              importance: fact.importance,
              from_ts:    fact.from_ts,
              session_id: fact.session_id,
              created_at: new Date().toISOString(),
            },
          }],
        });
        factsStored++;
      } catch (err) {
        warn(`Failed to store fact`, { fact: fact.text.slice(0, 60), err: err.message });
      }
    }
    ok(`Facts stored`, { count: factsStored });
  }

  // ── 6. Clear Redis buffer (only on full success) ──────────────────────────
  if (!DRY && failed === 0) {
    await redis.del(turnsKey);
    await redis.hSet(metaKey, {
      last_flush:  new Date().toISOString(),
      last_stored: String(stored),
    });
    ok('Redis buffer cleared after successful consolidation');
  } else if (!DRY && failed > 0) {
    warn('Partial failures — Redis buffer NOT cleared (will retry tomorrow)', { failed });
  }

  // ── 7. Summary ────────────────────────────────────────────────────────────
  ok('=== Consolidation complete ===', {
    chunks_total:   chunks.length,
    stored,
    skipped,
    failed,
    facts_collected: allFacts.length,
    dry_run: DRY,
  });

  await redis.quit();
}

main().catch(async (err) => {
  error('Unhandled error in mem-consolidate.js', { message: err.message, stack: err.stack });
  process.exit(1);
});
