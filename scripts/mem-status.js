'use strict';
/**
 * mem-status.js — Health check for all three memory layers
 * Usage: node mem-status.js
 */

process.chdir('/home/rico/.openclaw/workspace');
const { CONFIG, createRedisClient, createQdrantClient } = require('./config');

async function main() {
  console.log('\n🧠 ISBE Memory System — Status\n' + '═'.repeat(50));

  let overallOk = true;

  // ── Redis ─────────────────────────────────────────────────────────────────
  console.log('\n📦 Layer 1: Redis Hot Buffer');
  try {
    const redis    = await createRedisClient();
    const pong     = await redis.ping();
    const info     = await redis.info('server');
    const version  = info.match(/redis_version:([^\r\n]+)/)?.[1]?.trim() || '?';
    const memory   = await redis.info('memory');
    const usedMem  = memory.match(/used_memory_human:([^\r\n]+)/)?.[1]?.trim() || '?';

    const turnsKey = CONFIG.memory.turnsKey();
    const metaKey  = CONFIG.memory.metaKey();
    const turns    = await redis.lLen(turnsKey);
    const meta     = await redis.hGetAll(metaKey);

    console.log(`  ✅ Connected (Redis v${version})`);
    console.log(`  💾 Memory used: ${usedMem}`);
    console.log(`  📝 Buffered turns: ${turns}`);
    if (meta.last_run)   console.log(`  🕐 Last buffer run: ${meta.last_run}`);
    if (meta.last_flush) console.log(`  🔄 Last flush to Qdrant: ${meta.last_flush}`);
    if (meta.session_id) console.log(`  🔑 Session: ${meta.session_id.slice(0, 8)}...`);

    await redis.quit();
  } catch (err) {
    console.log(`  ❌ Redis error: ${err.message}`);
    overallOk = false;
  }

  // ── Daily Markdown ────────────────────────────────────────────────────────
  const fs   = require('fs');
  const path = require('path');
  console.log('\n📄 Layer 2: Daily Markdown Files');
  try {
    const memDir = path.join(CONFIG.memory.workspace, 'memory');
    const files  = fs.readdirSync(memDir)
      .filter(f => f.match(/^\d{4}-\d{2}-\d{2}\.md$/))
      .sort()
      .reverse();
    console.log(`  ✅ ${files.length} daily log files`);
    if (files[0]) console.log(`  📅 Latest: ${files[0]}`);
    if (files[1]) console.log(`  📅 Previous: ${files[1]}`);
  } catch (err) {
    console.log(`  ❌ Error reading memory dir: ${err.message}`);
    overallOk = false;
  }

  // ── Qdrant ────────────────────────────────────────────────────────────────
  console.log('\n🔍 Layer 3: Qdrant Vector Database');
  try {
    const qdrant = createQdrantClient();
    const list   = await qdrant.getCollections();
    console.log(`  ✅ Connected (${list.collections.length} collections)`);
    for (const col of list.collections) {
      const info = await qdrant.getCollection(col.name);
      const name = col.name.replace('isbe_', '');
      const vecs = info.points_count || info.vectors_count || 0;
      const stat = info.status || '?';
      console.log(`  📚 ${name}: ${vecs} vectors [${stat}]`);
    }
  } catch (err) {
    console.log(`  ❌ Qdrant error: ${err.message}`);
    overallOk = false;
  }

  // ── Embedding endpoint ────────────────────────────────────────────────────
  console.log('\n⚡ Mac Mini (Embedding + LLM)');
  try {
    const res = await fetch(`${CONFIG.ollama.host}/api/tags`, {
      signal: AbortSignal.timeout(5000),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data   = await res.json();
    const models = (data.models || []).map(m => m.name);
    const hasEmbed = models.some(m => m.includes('qwen3-embedding'));
    const hasLLM   = models.some(m => m.includes('qwen3:14b'));
    console.log(`  ✅ Reachable (${models.length} models loaded)`);
    console.log(`  ${hasEmbed ? '✅' : '❌'} qwen3-embedding:0.6b — ${hasEmbed ? 'ready' : 'NOT LOADED'}`);
    console.log(`  ${hasLLM   ? '✅' : '❌'} qwen3:14b — ${hasLLM ? 'ready' : 'NOT LOADED'}`);
  } catch (err) {
    console.log(`  ❌ Mac Mini unreachable: ${err.message}`);
    overallOk = false;
  }

  // ── Summary ───────────────────────────────────────────────────────────────
  console.log('\n' + '═'.repeat(50));
  console.log(`Overall: ${overallOk ? '✅ All systems operational' : '⚠️  One or more issues detected'}`);
  console.log('');
}

main().catch(err => {
  console.error('Status check error:', err.message);
  process.exit(1);
});
