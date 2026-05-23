#!/usr/bin/env node
import { createEngramClient } from '../src/engram-client.js';

const query = process.argv.slice(2).join(' ').trim() || 'OpenClaw memory';
const client = createEngramClient({});

try {
  const health = await client.health();
  const search = await client.search({ query, limit: 3 });
  console.log(JSON.stringify({ status: 'ok', health, count: search.results.length, results: search.results }, null, 2));
} catch (err) {
  console.error(err && err.stack ? err.stack : String(err));
  process.exit(1);
}
