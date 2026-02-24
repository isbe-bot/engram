'use strict';
/**
 * mem-buffer.js — Session JSONL → Redis hot buffer
 *
 * Reads the current OpenClaw main-agent session JSONL, extracts meaningful
 * user/assistant text turns (skipping tool-only and system noise), and
 * pushes new turns to the Redis buffer. Tracks progress so it never
 * duplicates. Token-free — designed to run from cron every 5 minutes.
 *
 * Usage: node mem-buffer.js [--agent main]
 */

process.chdir('/home/rico/.openclaw/workspace');
const fs   = require('fs');
const path = require('path');
const { CONFIG, createRedisClient, log } = require('./config');

const LOG    = CONFIG.memory.bufferLog;
const info   = (msg, d) => log(LOG, 'INFO',  msg, d);
const warn   = (msg, d) => log(LOG, 'WARN',  msg, d);
const error  = (msg, d) => log(LOG, 'ERROR', msg, d);
const ok     = (msg, d) => log(LOG, 'OK',    msg, d);

// ── Args ──────────────────────────────────────────────────────────────────────
const args   = process.argv.slice(2);
const AGENT  = args.includes('--agent') ? args[args.indexOf('--agent') + 1] : 'main';

// ── Filters ───────────────────────────────────────────────────────────────────
const NOISE_PATTERNS = [
  /^A new session was started via \/new/i,
  /^Read HEARTBEAT\.md if it exists/i,
  /^\s*HEARTBEAT_OK\s*$/,
  /^\s*NO_REPLY\s*$/,
  /^System:\s*\[/,
  /^\[System Message\]/,
];

function isNoise(text) {
  if (!text || text.trim().length < 20) return true;
  return NOISE_PATTERNS.some(re => re.test(text.trim()));
}

// ── JSONL parser ──────────────────────────────────────────────────────────────
/**
 * Extract text from message content blocks.
 * Returns empty string for tool-only turns.
 */
function extractText(content) {
  if (!content) return '';
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) return '';
  return content
    .filter(b => b.type === 'text')
    .map(b => b.text || '')
    .join('\n')
    .trim();
}

/**
 * Parse a session JSONL file and return meaningful turns as objects.
 * Only returns assistant turns with stopReason === 'endTurn' (final responses).
 * Returns array of: { role, text, timestamp, id }
 */
function parseSessionTurns(jsonlPath) {
  if (!fs.existsSync(jsonlPath)) return [];

  const lines = fs.readFileSync(jsonlPath, 'utf8').split('\n').filter(Boolean);
  const turns = [];

  for (const line of lines) {
    let entry;
    try { entry = JSON.parse(line); } catch { continue; }
    if (entry.type !== 'message') continue;

    const msg  = entry.message;
    const role = msg?.role;
    if (!['user', 'assistant'].includes(role)) continue;

    // For assistant: only final responses (endTurn), skip tool-only turns
    if (role === 'assistant' && msg.stopReason !== 'endTurn') continue;

    const text = extractText(msg.content);
    if (isNoise(text)) continue;

    turns.push({
      role,
      text,
      timestamp: entry.timestamp || new Date().toISOString(),
      id:        entry.id || '',
    });
  }

  return turns;
}

// ── Main ──────────────────────────────────────────────────────────────────────
async function main() {
  info(`mem-buffer starting`, { agent: AGENT });

  // ── 1. Find active session file ───────────────────────────────────────────
  const indexPath = CONFIG.memory.sessionIndex
    .replace('/main/', `/${AGENT}/`);

  if (!fs.existsSync(indexPath)) {
    warn('Session index not found', { path: indexPath });
    process.exit(0);
  }

  const index     = JSON.parse(fs.readFileSync(indexPath, 'utf8'));
  const sessionKey = Object.keys(index).find(k => k.includes(`:${AGENT}:`));
  if (!sessionKey) {
    warn('No active session found in index', { agent: AGENT });
    process.exit(0);
  }

  const sessionMeta = index[sessionKey];
  const sessionFile  = sessionMeta.sessionFile;
  const sessionId    = sessionMeta.sessionId;

  if (!sessionFile || !fs.existsSync(sessionFile)) {
    warn('Session file not found', { sessionFile });
    process.exit(0);
  }

  // ── 2. Connect Redis ──────────────────────────────────────────────────────
  let redis;
  try {
    redis = await createRedisClient();
  } catch (err) {
    error('Redis connection failed', { message: err.message });
    process.exit(1);
  }

  const turnsKey = CONFIG.memory.turnsKey(AGENT);
  const metaKey  = CONFIG.memory.metaKey(AGENT);

  // ── 3. Check last processed state ────────────────────────────────────────
  const storedMeta     = await redis.hGetAll(metaKey);
  const lastSessionId  = storedMeta.session_id  || '';
  const lastLineCount  = parseInt(storedMeta.line_count || '0');

  // If session changed (compaction/rotation), reset tracking
  if (lastSessionId && lastSessionId !== sessionId) {
    info('Session rotated — resetting line tracker', {
      old: lastSessionId,
      new: sessionId,
    });
    await redis.hSet(metaKey, { session_id: sessionId, line_count: '0' });
  }

  // ── 4. Parse all turns from session file ─────────────────────────────────
  const rawLines    = fs.readFileSync(sessionFile, 'utf8').split('\n').filter(Boolean);
  const currentLine = rawLines.length;
  const startFrom   = (lastSessionId === sessionId) ? lastLineCount : 0;

  if (currentLine <= startFrom) {
    info('No new lines since last run', { currentLine, startFrom });
    await redis.quit();
    process.exit(0);
  }

  // Parse only new lines
  const newLines = rawLines.slice(startFrom);
  const newTurns = [];

  for (const line of newLines) {
    let entry;
    try { entry = JSON.parse(line); } catch { continue; }
    if (entry.type !== 'message') continue;

    const msg  = entry.message;
    const role = msg?.role;
    if (!['user', 'assistant'].includes(role)) continue;
    if (role === 'assistant' && msg.stopReason !== 'endTurn') continue;

    const text = extractText(msg.content);
    if (isNoise(text)) continue;

    newTurns.push({
      role,
      text,
      timestamp: entry.timestamp || new Date().toISOString(),
      id:        entry.id || '',
      session_id: sessionId,
    });
  }

  if (newTurns.length === 0) {
    info('No new meaningful turns found', { newLines: newLines.length, sessionId });
    await redis.hSet(metaKey, {
      session_id:  sessionId,
      line_count:  String(currentLine),
      last_run:    new Date().toISOString(),
    });
    await redis.quit();
    process.exit(0);
  }

  // ── 5. Push to Redis ──────────────────────────────────────────────────────
  for (const turn of newTurns) {
    await redis.lPush(turnsKey, JSON.stringify(turn));
    // Set TTL of 48h on each entry (Redis list TTL is per-key, not per-element)
  }

  // Set 48h TTL on the list key
  await redis.expire(turnsKey, 48 * 60 * 60);

  // Update meta
  await redis.hSet(metaKey, {
    session_id:  sessionId,
    line_count:  String(currentLine),
    last_run:    new Date().toISOString(),
    agent:       AGENT,
  });

  const totalBuffered = await redis.lLen(turnsKey);

  ok('Buffer updated', {
    new_turns:     newTurns.length,
    total_buffered: totalBuffered,
    session_id:    sessionId.slice(0, 8) + '...',
  });

  await redis.quit();
}

main().catch(async (err) => {
  error('Unhandled error in mem-buffer.js', { message: err.message });
  process.exit(1);
});
