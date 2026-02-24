'use strict';
/**
 * config.js — Shared config and client factory for the Jarvis Memory System
 * Loads credentials from .secrets files, exports ready-to-use clients.
 * Never hardcodes secrets. Never logs secrets.
 */

const fs = require('fs');
const path = require('path');
const { createClient } = require('redis');
const { QdrantClient } = require('@qdrant/js-client-rest');

// ─── Paths ────────────────────────────────────────────────────────────────────
const WORKSPACE       = '/home/rico/.openclaw/workspace';
const SECRETS_DIR     = path.join(WORKSPACE, '.secrets');
const SESSION_DIR     = '/home/rico/.openclaw/agents/main/sessions';
const SESSION_INDEX   = path.join(SESSION_DIR, 'sessions.json');
const SCRIPTS_DIR     = path.join(WORKSPACE, 'scripts', 'memory');
const LOG_DIR         = '/var/log';

// ─── Secret loader ────────────────────────────────────────────────────────────
function loadSecret(name) {
  const file = path.join(SECRETS_DIR, name);
  if (!fs.existsSync(file)) {
    throw new Error(`Secret file not found: ${file}`);
  }
  return fs.readFileSync(file, 'utf8').trim();
}

// ─── Infrastructure config ────────────────────────────────────────────────────
const CONFIG = {
  redis: {
    host: process.env.REDIS_HOST || '127.0.0.1',
    port: parseInt(process.env.REDIS_PORT || '6379'),
    get password() { return loadSecret('redis_password'); },
  },
  qdrant: {
    url: process.env.QDRANT_URL || 'http://127.0.0.1:6333',
    get apiKey() { return loadSecret('qdrant_api_key'); },
  },
  ollama: {
    host: process.env.OLLAMA_HOST || 'http://100.73.153.121:11434',
    embedModel: process.env.EMBED_MODEL || 'qwen3-embedding:0.6b',
    llmModel:   process.env.LLM_MODEL   || 'qwen3:14b',
    embedDims:  parseInt(process.env.EMBED_DIMS || '1024'),
  },
  memory: {
    sessionDir:   SESSION_DIR,
    sessionIndex: SESSION_INDEX,
    workspace:    WORKSPACE,
    scriptsDir:   SCRIPTS_DIR,
    logDir:       LOG_DIR,
    consolidateLog: path.join(LOG_DIR, 'mem-consolidate.log'),
    bufferLog:      path.join(LOG_DIR, 'mem-buffer.log'),
    // Redis keys
    turnsKey:   (agentId = 'main') => `memory:sessions:${agentId}:turns`,
    metaKey:    (agentId = 'main') => `memory:sessions:${agentId}:meta`,
    // Qdrant collections
    collections: {
      conversations: 'isbe_conversations',
      facts:         'isbe_facts',
      projects:      'isbe_projects',
    },
  },
};

// ─── Redis client factory ─────────────────────────────────────────────────────
async function createRedisClient() {
  const client = createClient({
    socket: { host: CONFIG.redis.host, port: CONFIG.redis.port },
    password: CONFIG.redis.password,
  });
  client.on('error', (err) => {
    // Avoid logging the password in error messages
    console.error('[redis] Connection error:', err.message);
  });
  await client.connect();
  return client;
}

// ─── Qdrant client factory ────────────────────────────────────────────────────
function createQdrantClient() {
  return new QdrantClient({
    url: CONFIG.qdrant.url,
    apiKey: CONFIG.qdrant.apiKey,
    checkCompatibility: false,  // client 1.17 vs server 1.13 — minor ver diff only
  });
}

// ─── Ollama helpers ───────────────────────────────────────────────────────────

/**
 * Embed text using qwen3-embedding:0.6b on Mac Mini.
 * Returns float[] of length 1024.
 */
async function embed(text) {
  const res = await fetch(`${CONFIG.ollama.host}/api/embeddings`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      model: CONFIG.ollama.embedModel,
      prompt: text,
    }),
    signal: AbortSignal.timeout(30_000),
  });
  if (!res.ok) throw new Error(`Embedding failed: ${res.status} ${res.statusText}`);
  const data = await res.json();
  const embedding = data.embedding;
  if (!Array.isArray(embedding) || embedding.length !== CONFIG.ollama.embedDims) {
    throw new Error(`Unexpected embedding dims: got ${embedding?.length}, expected ${CONFIG.ollama.embedDims}`);
  }
  return embedding;
}

/**
 * Summarize/extract structured data from conversation text using qwen3:14b.
 * Returns { summary, facts[], topics[], importance } or null on failure.
 * IMPORTANT: must use think:false — model returns empty content otherwise.
 */
async function summarize(conversationText) {
  const systemPrompt = `You are a memory consolidation assistant for an AI called ISBE.
Extract structured information from conversation excerpts.
Return ONLY valid JSON with exactly these keys:
- summary: string (2-3 sentence description of what was discussed)
- facts: array of strings (concrete facts, decisions, preferences learned)
- topics: array of strings (main topic tags, 2-6 tags)
- importance: integer 1-10 (10 = critical info to remember, 1 = trivial chitchat)

Focus on: technical decisions, project context, user preferences, problems solved, lessons learned.
Ignore: system prompts, heartbeat messages, tool call noise.`;

  const res = await fetch(`${CONFIG.ollama.host}/api/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      model: CONFIG.ollama.llmModel,
      messages: [
        { role: 'system', content: systemPrompt },
        { role: 'user',   content: conversationText },
      ],
      stream: false,
      think:  false,   // CRITICAL: without this, content is empty
      options: { temperature: 0.2, num_predict: 600 },
    }),
    signal: AbortSignal.timeout(120_000),
  });
  if (!res.ok) throw new Error(`LLM call failed: ${res.status} ${res.statusText}`);
  const data = await res.json();
  const content = data.message?.content || '';

  // Extract JSON from response (may be wrapped in markdown)
  const match = content.match(/\{[\s\S]*\}/);
  if (!match) throw new Error(`No JSON in LLM response: ${content.slice(0, 200)}`);
  const parsed = JSON.parse(match[0]);

  // Validate required fields
  if (!parsed.summary || !Array.isArray(parsed.facts) || !Array.isArray(parsed.topics) || !parsed.importance) {
    throw new Error(`Missing required fields in LLM response: ${JSON.stringify(parsed).slice(0, 200)}`);
  }
  return parsed;
}

// ─── Logging helper ───────────────────────────────────────────────────────────
function log(logFile, level, message, data = null) {
  const ts = new Date().toISOString();
  const line = `[${ts}] [${level.toUpperCase()}] ${message}${data ? ' ' + JSON.stringify(data) : ''}`;
  console.log(line);
  try {
    fs.appendFileSync(logFile, line + '\n');
  } catch (_) {
    // Log dir may not be writable — stdout is always there
  }
}

module.exports = { CONFIG, createRedisClient, createQdrantClient, embed, summarize, log };
