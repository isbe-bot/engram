'use strict';
/**
 * mem-search.js — Semantic search across Qdrant memory collections
 *
 * Embeds a query, searches all Qdrant collections, returns ranked results.
 * Usage:
 *   node mem-search.js "your query here"
 *   node mem-search.js "your query" --collection conversations
 *   node mem-search.js "your query" --limit 10 --min-score 0.6
 *   node mem-search.js "your query" --json   (machine-readable output)
 */

process.chdir('/home/rico/.openclaw/workspace');
const { CONFIG, createQdrantClient, embed, log } = require('./config');

const LOG   = CONFIG.memory.consolidateLog;
const info  = (msg, d) => log(LOG, 'INFO',  msg, d);
const error = (msg, d) => log(LOG, 'ERROR', msg, d);

// ── Args ──────────────────────────────────────────────────────────────────────
const args = process.argv.slice(2);

// Extract query (first non-flag arg)
const queryArgs = args.filter(a => !a.startsWith('--'));
const QUERY = queryArgs.join(' ').trim();

function getFlag(name, defaultVal) {
  const i = args.indexOf(`--${name}`);
  return i !== -1 ? args[i + 1] : defaultVal;
}
function hasFlag(name) { return args.includes(`--${name}`); }

const COLLECTION = getFlag('collection', null);   // null = all collections
const LIMIT      = parseInt(getFlag('limit', '5'));
const MIN_SCORE  = parseFloat(getFlag('min-score', '0.5'));
const JSON_OUT   = hasFlag('json');

if (!QUERY) {
  console.error('Usage: node mem-search.js "your query" [--collection conversations|facts|projects] [--limit N] [--min-score N] [--json]');
  process.exit(1);
}

// ── Search ────────────────────────────────────────────────────────────────────
async function searchCollection(qdrant, collectionName, vector, limit, minScore) {
  try {
    const results = await qdrant.search(collectionName, {
      vector,
      limit,
      score_threshold: minScore,
      with_payload: true,
    });
    return results.map(r => ({ ...r, collection: collectionName }));
  } catch (err) {
    // Collection may not exist yet
    if (err.message?.includes('Not found') || err.message?.includes('doesn\'t exist')) {
      return [];
    }
    throw err;
  }
}

async function main() {
  const qdrant = createQdrantClient();

  // Embed the query
  let queryVector;
  try {
    queryVector = await embed(QUERY);
  } catch (err) {
    console.error('Embedding failed:', err.message);
    process.exit(1);
  }

  // Determine which collections to search
  const collectionMap = CONFIG.memory.collections;
  const targets = COLLECTION
    ? [collectionMap[COLLECTION] || COLLECTION]
    : Object.values(collectionMap);

  // Search all target collections in parallel
  const resultArrays = await Promise.all(
    targets.map(name => searchCollection(qdrant, name, queryVector, LIMIT, MIN_SCORE))
  );

  // Merge and sort by score descending
  const allResults = resultArrays
    .flat()
    .sort((a, b) => b.score - a.score)
    .slice(0, LIMIT);

  if (JSON_OUT) {
    console.log(JSON.stringify({ query: QUERY, results: allResults }, null, 2));
    return;
  }

  // ── Human-readable output ─────────────────────────────────────────────────
  if (allResults.length === 0) {
    console.log(`\n🔍 No memories found for: "${QUERY}"\n`);
    console.log('Tip: Run mem-consolidate.js to flush the Redis buffer to Qdrant first.');
    return;
  }

  console.log(`\n🧠 Memory Search: "${QUERY}"`);
  console.log(`Found ${allResults.length} result(s)\n`);
  console.log('─'.repeat(60));

  for (const result of allResults) {
    const p = result.payload || {};
    const score = (result.score * 100).toFixed(1);
    const col   = result.collection.replace('isbe_', '');
    const ts    = p.from_ts || p.created_at || '';
    const date  = ts ? new Date(ts).toLocaleDateString() : '';

    console.log(`\n[${score}% match] [${col}] ${date ? `(${date})` : ''}`);

    if (col === 'facts') {
      console.log(`  📌 ${p.text || ''}`);
      if (p.topics?.length) console.log(`  🏷️  ${p.topics.join(', ')}`);
    } else {
      console.log(`  📝 ${p.summary || ''}`);
      if (p.topics?.length)  console.log(`  🏷️  ${p.topics.join(', ')}`);
      if (p.facts?.length) {
        console.log(`  Facts:`);
        p.facts.slice(0, 3).forEach(f => console.log(`    • ${f}`));
      }
    }
    console.log(`  Importance: ${'★'.repeat(Math.min(p.importance || 1, 10))}`);
    console.log('─'.repeat(60));
  }
  console.log('');
}

main().catch(err => {
  console.error('Search error:', err.message);
  process.exit(1);
});
