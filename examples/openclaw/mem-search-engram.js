#!/usr/bin/env node
'use strict';
/**
 * OpenClaw ENGRAM search adapter.
 *
 * Drop this into an OpenClaw workspace as scripts/memory/mem-search.js, or call it
 * from your existing memory_search fallback path. It uses engramctl so auth,
 * citations, governance filters, and pagination stay consistent with ENGRAM.
 */

const { spawnSync } = require('child_process');

const ENGRAMCTL = process.env.ENGRAMCTL_BIN || 'engramctl';
const ENGRAM_CONFIG = process.env.ENGRAM_CONFIG || './engram.yaml';
const args = process.argv.slice(2);

function flag(name, fallback = '') {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : fallback;
}
function has(name) { return args.includes(`--${name}`); }

const query = args.filter((arg, i) => {
  if (arg.startsWith('--')) return false;
  const prev = args[i - 1];
  return !['--limit', '--min-score', '--min-confidence', '--status', '--cursor'].includes(prev);
}).join(' ').trim();

if (!query) {
  console.error('Usage: mem-search-engram.js "query" [--limit N] [--min-confidence N] [--json]');
  process.exit(2);
}

const ctlArgs = [
  'search',
  '--config', ENGRAM_CONFIG,
  '--q', query,
  '--limit', flag('limit', '5'),
  '--status', flag('status', 'accepted'),
  '--min-confidence', flag('min-confidence', flag('min-score', '0')),
];
if (flag('cursor')) ctlArgs.push('--cursor', flag('cursor'));
if (has('include-events')) ctlArgs.push('--include-events');

const res = spawnSync(ENGRAMCTL, ctlArgs, { encoding: 'utf8', maxBuffer: 20 * 1024 * 1024 });
if (res.stdout) process.stdout.write(res.stdout);
if (res.stderr) process.stderr.write(res.stderr);
process.exit(res.status ?? 1);
