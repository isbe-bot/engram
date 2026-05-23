import { spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const DEFAULT_ENDPOINT = 'http://127.0.0.1:8787';
const DEFAULT_CONFIG_PATH = '~/.config/engram/engram.yaml';
const DEFAULT_ENGRAMCTL = 'engramctl';

export function resolveEngramConfig(raw = {}) {
  const cfg = raw && typeof raw === 'object' ? raw : {};
  return {
    endpoint: trim(cfg.endpoint) || trim(process.env.ENGRAM_ENDPOINT) || trim(process.env.ENGRAM_URL) || DEFAULT_ENDPOINT,
    apiKey: trim(cfg.apiKey) || trim(process.env.ENGRAM_API_KEY) || trim(process.env.ENGRAM_READ_API_KEY) || trim(process.env.ENGRAM_ADMIN_API_KEY) || '',
    configPath: expandHome(trim(cfg.configPath) || trim(process.env.ENGRAM_CONFIG) || DEFAULT_CONFIG_PATH),
    engramctl: trim(cfg.engramctl) || trim(process.env.ENGRAMCTL_BIN) || DEFAULT_ENGRAMCTL,
    fallbackToCli: cfg.fallbackToCli !== false,
    timeoutMs: positiveInt(cfg.timeoutMs, 5000),
    defaultLimit: positiveInt(cfg.defaultLimit, 5),
    defaultStatus: trim(cfg.defaultStatus) || 'accepted',
    defaultMinConfidence: nonNegativeNumber(cfg.defaultMinConfidence, 0),
  };
}

export function createEngramClient(rawConfig = {}) {
  const config = resolveEngramConfig(rawConfig);
  return {
    config,
    async health() {
      const errors = [];
      if (config.endpoint) {
        try {
          return { transport: 'http', ...(await httpJSON(config, '/v1/health')) };
        } catch (err) {
          errors.push(`http: ${formatError(err)}`);
        }
      }
      if (config.fallbackToCli) {
        try {
          const status = runEngramctlJSON(config, ['health', '--config', config.configPath]);
          return { transport: 'cli', ...status };
        } catch (err) {
          errors.push(`cli: ${formatError(err)}`);
        }
      }
      throw new Error(`ENGRAM unavailable (${errors.join('; ') || 'no transport configured'})`);
    },
    async search(params = {}) {
      const query = trim(params.query ?? params.q);
      if (!query) throw new Error('query is required');
      const limit = positiveInt(params.limit ?? params.maxResults, config.defaultLimit);
      const status = trim(params.status) || config.defaultStatus;
      const minConfidence = nonNegativeNumber(params.minConfidence ?? params.min_score, config.defaultMinConfidence);
      const includeEvents = Boolean(params.includeEvents);
      const cursor = trim(params.cursor);
      const errors = [];

      if (config.endpoint) {
        const qs = new URLSearchParams({ q: query, limit: String(limit) });
        if (status) qs.set('status', status);
        if (minConfidence > 0) qs.set('min_confidence', String(minConfidence));
        if (cursor) qs.set('cursor', cursor);
        if (includeEvents) qs.set('include_events', 'true');
        try {
          const result = await httpJSON(config, `/v1/memory/search?${qs.toString()}`);
          return normalizeSearchResponse(result);
        } catch (err) {
          errors.push(`http: ${formatError(err)}`);
        }
      }

      if (config.fallbackToCli) {
        try {
          const args = ['search', '--config', config.configPath, '--q', query, '--limit', String(limit)];
          if (status) args.push('--status', status);
          if (minConfidence > 0) args.push('--min-confidence', String(minConfidence));
          if (cursor) args.push('--cursor', cursor);
          if (includeEvents) args.push('--include-events');
          return normalizeSearchResponse(runEngramctlJSON(config, args));
        } catch (err) {
          errors.push(`cli: ${formatError(err)}`);
        }
      }
      throw new Error(`ENGRAM search failed (${errors.join('; ')})`);
    },
    async get(objectID) {
      const id = trim(objectID);
      if (!id) throw new Error('object_id is required');
      const errors = [];
      if (config.endpoint) {
        try {
          const result = await httpJSON(config, `/v1/memory/${encodeURIComponent(id)}`);
          return result.memory ?? result;
        } catch (err) {
          errors.push(`http: ${formatError(err)}`);
        }
      }
      if (config.fallbackToCli) {
        try {
          const result = runEngramctlJSON(config, ['get', '--config', config.configPath, '--id', id]);
          return result.memory ?? result;
        } catch (err) {
          errors.push(`cli: ${formatError(err)}`);
        }
      }
      throw new Error(`ENGRAM get failed (${errors.join('; ')})`);
    },
    async curate(input = {}) {
      const payload = {
        object_id: trim(input.object_id ?? input.objectID),
        type: trim(input.type),
        content: trim(input.content),
        source_refs: Array.isArray(input.source_refs) ? input.source_refs : Array.isArray(input.sourceRefs) ? input.sourceRefs : [],
        confidence: input.confidence,
        classification: input.classification,
        scope: input.scope,
        schema_version: input.schema_version ?? input.schemaVersion,
        envelope: input.envelope ?? {
          actor_id: trim(input.actor_id) || 'openclaw-plugin',
          mutation_id: trim(input.mutation_id) || `openclaw-${Date.now()}`,
          signature: trim(input.signature) || 'openclaw-plugin',
        },
      };
      if (!payload.object_id) delete payload.object_id;
      if (!payload.schema_version) delete payload.schema_version;
      if (!payload.classification) delete payload.classification;
      if (!payload.scope) delete payload.scope;
      if (payload.confidence === undefined || payload.confidence === null) delete payload.confidence;

      const errors = [];
      if (config.endpoint) {
        try {
          const result = await httpJSON(config, '/v1/memory/curate', { method: 'POST', body: payload });
          return result.memory ?? result;
        } catch (err) {
          errors.push(`http: ${formatError(err)}`);
        }
      }
      if (config.fallbackToCli) {
        try {
          const result = runEngramctlJSON(config, ['curate', '--config', config.configPath, '--json', JSON.stringify(payload)]);
          return result.memory ?? result;
        } catch (err) {
          errors.push(`cli: ${formatError(err)}`);
        }
      }
      throw new Error(`ENGRAM curate failed (${errors.join('; ')})`);
    },
  };
}

export function toCorpusSearchResult(row) {
  const id = trim(row.object_id ?? row.event_id ?? row.id);
  const sourceRefs = Array.isArray(row.source_refs) ? row.source_refs : [];
  const citation = Array.isArray(row.citations) && row.citations.length > 0
    ? row.citations.map((c) => c.path || c.id).filter(Boolean).join('; ')
    : sourceRefs.join('; ');
  const score = Number(row.rank_score ?? row.semantic_score ?? row.confidence ?? row.score ?? 0);
  return {
    corpus: 'engram',
    path: id ? `engram://${row.event_id ? 'event' : 'memory'}/${id}` : 'engram://result',
    title: id || row.type || row.event_type || 'ENGRAM result',
    kind: row.event_id ? 'event' : (row.type || 'memory_object'),
    score: Number.isFinite(score) ? score : 0,
    snippet: trim(row.content) || trim(row.event_type) || JSON.stringify(row.data ?? row).slice(0, 1000),
    id,
    citation,
    source: 'ENGRAM',
    provenanceLabel: row.provenance_hash || citation || 'ENGRAM',
    sourceType: row.event_id ? 'event' : 'memory_object',
    sourcePath: sourceRefs[0] || row.retrieval_source || 'engram',
    updatedAt: row.updated_at || row.created_at || row.occurred_at,
  };
}

export function toCorpusGetResult(memory) {
  const id = trim(memory.object_id ?? memory.id);
  return {
    corpus: 'engram',
    path: id ? `engram://memory/${id}` : 'engram://memory',
    title: id || memory.type || 'ENGRAM memory',
    kind: memory.type || 'memory_object',
    content: JSON.stringify(memory, null, 2),
    fromLine: 1,
    lineCount: JSON.stringify(memory, null, 2).split('\n').length,
    id,
    provenanceLabel: memory.provenance_hash || '',
    sourceType: 'memory_object',
    sourcePath: Array.isArray(memory.source_refs) ? memory.source_refs[0] : undefined,
    updatedAt: memory.updated_at || memory.created_at,
  };
}

async function httpJSON(config, urlPath, opts = {}) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), config.timeoutMs);
  const headers = { accept: 'application/json', ...(opts.headers || {}) };
  if (config.apiKey) headers.authorization = `Bearer ${config.apiKey}`;
  let body;
  if (opts.body !== undefined) {
    headers['content-type'] = 'application/json';
    body = JSON.stringify(opts.body);
  }
  try {
    const res = await fetch(new URL(urlPath, config.endpoint), { method: opts.method || 'GET', headers, body, signal: controller.signal });
    const text = await res.text();
    let json = {};
    if (text) {
      try { json = JSON.parse(text); } catch { json = { raw: text }; }
    }
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}: ${json.error || json.message || text}`);
    return json;
  } finally {
    clearTimeout(timer);
  }
}

function runEngramctlJSON(config, args) {
  if (!config.engramctl || (config.engramctl.includes('/') && !existsSync(config.engramctl))) {
    throw new Error(`engramctl not found: ${config.engramctl}`);
  }
  const res = spawnSync(config.engramctl, args, { encoding: 'utf8', maxBuffer: 20 * 1024 * 1024 });
  if (res.status !== 0) throw new Error((res.stderr || res.stdout || `engramctl exited ${res.status}`).trim());
  try {
    return JSON.parse(res.stdout || '{}');
  } catch (err) {
    throw new Error(`invalid engramctl JSON: ${err.message}; stdout=${res.stdout.slice(0, 500)}`);
  }
}

function normalizeSearchResponse(result) {
  const rows = Array.isArray(result.results) ? result.results : [];
  return { ...result, results: rows };
}

function trim(value) { return typeof value === 'string' ? value.trim() : ''; }
function positiveInt(value, fallback) {
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : fallback;
}
function nonNegativeNumber(value, fallback) {
  const n = Number(value);
  return Number.isFinite(n) && n >= 0 ? n : fallback;
}
function expandHome(value) {
  if (!value) return value;
  return value === '~' ? os.homedir() : value.startsWith('~/') ? path.join(os.homedir(), value.slice(2)) : value;
}
function formatError(err) { return err && err.message ? err.message : String(err); }
