import { strict as assert } from 'node:assert';
import http from 'node:http';
import { chmod, mkdtemp, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { createEngramClient, resolveEngramConfig, toCorpusSearchResult } from '../src/engram-client.js';

test('resolveEngramConfig expands defaults safely', () => {
  const cfg = resolveEngramConfig({ endpoint: ' http://localhost:1 ', fallbackToCli: false, timeoutMs: 50 });
  assert.equal(cfg.endpoint, 'http://localhost:1');
  assert.equal(cfg.fallbackToCli, false);
  assert.equal(cfg.timeoutMs, 50);
  assert.match(cfg.configPath, /engram\.yaml$/);
});

test('HTTP search/get/curate path maps ENGRAM API responses', async (t) => {
  const server = http.createServer(async (req, res) => {
    res.setHeader('content-type', 'application/json');
    if (req.url.startsWith('/v1/health')) return res.end(JSON.stringify({ ok: true }));
    if (req.url.startsWith('/v1/memory/search')) {
      assert.equal(req.headers.authorization, 'Bearer read-token');
      return res.end(JSON.stringify({ results: [{ object_id: 'mem-1', type: 'decision', content: 'Use ENGRAM as governed memory.', confidence: 0.9, source_refs: ['spec:test'] }] }));
    }
    if (req.url === '/v1/memory/mem-1') return res.end(JSON.stringify({ memory: { object_id: 'mem-1', content: 'Use ENGRAM as governed memory.' } }));
    if (req.url === '/v1/memory/curate') {
      const chunks = [];
      for await (const chunk of req) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
      assert.equal(body.type, 'decision');
      return res.end(JSON.stringify({ status: 'accepted', memory: { object_id: 'mem-2', ...body } }));
    }
    res.statusCode = 404;
    res.end(JSON.stringify({ error: 'not found' }));
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  t.after(() => server.close());

  const endpoint = `http://127.0.0.1:${server.address().port}`;
  const client = createEngramClient({ endpoint, apiKey: 'read-token', fallbackToCli: false });
  assert.equal((await client.health()).ok, true);
  const search = await client.search({ query: 'governed', limit: 1 });
  assert.equal(search.results[0].object_id, 'mem-1');
  assert.equal((await client.get('mem-1')).object_id, 'mem-1');
  assert.equal((await client.curate({ type: 'decision', content: 'Curated', source_refs: ['chat:test'] })).object_id, 'mem-2');
});

test('toCorpusSearchResult produces OpenClaw corpus shape', () => {
  const row = { object_id: 'mem-1', type: 'fact', content: 'Hello memory', confidence: 0.8, source_refs: ['doc:test'] };
  const result = toCorpusSearchResult(row);
  assert.equal(result.corpus, 'engram');
  assert.equal(result.path, 'engram://memory/mem-1');
  assert.equal(result.snippet, 'Hello memory');
  assert.equal(result.sourcePath, 'doc:test');
});

test('CLI fallback works when HTTP is unavailable', async () => {
  const dir = await mkdtemp(path.join(os.tmpdir(), 'engram-plugin-test-'));
  const fakeCtl = path.join(dir, 'engramctl-fake');
  await writeFile(fakeCtl, `#!/usr/bin/env node
const args = process.argv.slice(2);
if (args[0] === 'health') {
  console.log(JSON.stringify({ ok: true }));
} else if (args[0] === 'search') {
  console.log(JSON.stringify({ results: [{ object_id: 'cli-1', content: 'CLI fallback' }] }));
} else {
  console.error('unexpected args: ' + args.join(' '));
  process.exit(2);
}
`);
  await chmod(fakeCtl, 0o700);

  const client = createEngramClient({ endpoint: 'http://127.0.0.1:1', engramctl: fakeCtl, configPath: '/tmp/none.yaml', fallbackToCli: true, timeoutMs: 100 });
  const health = await client.health();
  assert.equal(health.transport, 'cli');
  const search = await client.search({ query: 'x' });
  assert.equal(search.results[0].object_id, 'cli-1');
});
