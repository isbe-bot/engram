'use strict';
/**
 * mem-init.js — One-time setup: create Qdrant collections and verify connectivity.
 * Safe to re-run (idempotent — skips existing collections).
 * Usage: node mem-init.js
 */

process.chdir('/home/rico/.openclaw/workspace');
const { CONFIG, createRedisClient, createQdrantClient, embed, log } = require('./config');

const LOG = CONFIG.memory.consolidateLog;
const info  = (msg, d) => log(LOG, 'INFO',  msg, d);
const error = (msg, d) => log(LOG, 'ERROR', msg, d);
const ok    = (msg, d) => log(LOG, 'OK',    msg, d);

async function main() {
  info('=== mem-init.js starting ===');
  let redis, qdrant;

  // ── 1. Redis ──────────────────────────────────────────────────────────────
  info('Connecting to Redis...');
  try {
    redis = await createRedisClient();
    await redis.ping();
    ok('Redis connected', { host: CONFIG.redis.host, port: CONFIG.redis.port });
  } catch (err) {
    error('Redis connection failed', { message: err.message });
    process.exit(1);
  }

  // ── 2. Qdrant ─────────────────────────────────────────────────────────────
  info('Connecting to Qdrant...');
  try {
    qdrant = createQdrantClient();
    const health = await qdrant.api('service').healthz();
    ok('Qdrant connected', { url: CONFIG.qdrant.url });
  } catch (err) {
    // healthz may not exist in all versions — try collections list instead
    try {
      qdrant = createQdrantClient();
      await qdrant.getCollections();
      ok('Qdrant connected', { url: CONFIG.qdrant.url });
    } catch (err2) {
      error('Qdrant connection failed', { message: err2.message });
      await redis.quit();
      process.exit(1);
    }
  }

  // ── 3. Test embedding ─────────────────────────────────────────────────────
  info('Testing embedding endpoint...', { model: CONFIG.ollama.embedModel });
  try {
    const testEmbed = await embed('test memory system initialization');
    ok('Embedding working', { dims: testEmbed.length, model: CONFIG.ollama.embedModel });
  } catch (err) {
    error('Embedding test failed', { message: err.message });
    await redis.quit();
    process.exit(1);
  }

  // ── 4. Create Qdrant collections ──────────────────────────────────────────
  const collections = [
    {
      name: CONFIG.memory.collections.conversations,
      desc: 'Summarized conversation chunks with context',
    },
    {
      name: CONFIG.memory.collections.facts,
      desc: 'Extracted facts, preferences, and decisions',
    },
    {
      name: CONFIG.memory.collections.projects,
      desc: 'Project context snapshots',
    },
  ];

  const existingList = await qdrant.getCollections();
  const existing = new Set(existingList.collections.map(c => c.name));

  for (const col of collections) {
    if (existing.has(col.name)) {
      info(`Collection already exists — skipping: ${col.name}`);
      continue;
    }
    info(`Creating collection: ${col.name}`);
    try {
      await qdrant.createCollection(col.name, {
        vectors: {
          size:     CONFIG.ollama.embedDims,  // 1024
          distance: 'Cosine',
        },
        optimizers_config: {
          default_segment_number: 2,
        },
        replication_factor: 1,
      });
      ok(`Collection created: ${col.name}`, { dims: CONFIG.ollama.embedDims, distance: 'Cosine' });
    } catch (err) {
      error(`Failed to create collection: ${col.name}`, { message: err.message });
      await redis.quit();
      process.exit(1);
    }
  }

  // ── 5. Verify all collections ─────────────────────────────────────────────
  info('Verifying collections...');
  const finalList = await qdrant.getCollections();
  for (const col of finalList.collections) {
    const info2 = await qdrant.getCollection(col.name);
    ok(`Collection verified: ${col.name}`, {
      status: info2.status,
      vectors_count: info2.vectors_count || 0,
    });
  }

  // ── 6. Redis keys check ───────────────────────────────────────────────────
  info('Checking Redis memory namespace...');
  const turnsKey = CONFIG.memory.turnsKey();
  const metaKey  = CONFIG.memory.metaKey();
  const turnsLen = await redis.lLen(turnsKey);
  const metaData = await redis.hGetAll(metaKey);
  ok('Redis namespace ready', {
    turnsKey,
    buffered_turns: turnsLen,
    meta: metaData,
  });

  await redis.quit();
  ok('=== mem-init.js complete — all systems verified ===');
}

main().catch(async (err) => {
  error('Unhandled error in mem-init.js', { message: err.message, stack: err.stack });
  process.exit(1);
});
