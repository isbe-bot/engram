import { Type } from '@sinclair/typebox';
import { definePluginEntry } from 'openclaw/plugin-sdk/plugin-entry';
import { createEngramClient, resolveEngramConfig, toCorpusGetResult, toCorpusSearchResult } from './src/engram-client.js';

const SearchSchema = Type.Object({
  query: Type.String({ minLength: 1 }),
  limit: Type.Optional(Type.Number({ minimum: 1, maximum: 50 })),
  status: Type.Optional(Type.String()),
  minConfidence: Type.Optional(Type.Number({ minimum: 0, maximum: 1 })),
  includeEvents: Type.Optional(Type.Boolean()),
});

const GetSchema = Type.Object({ object_id: Type.String({ minLength: 1 }) });

const CurateSchema = Type.Object({
  object_id: Type.Optional(Type.String()),
  type: Type.String({ minLength: 1 }),
  content: Type.String({ minLength: 1 }),
  source_refs: Type.Optional(Type.Array(Type.String())),
  confidence: Type.Optional(Type.Number({ minimum: 0, maximum: 1 })),
  classification: Type.Optional(Type.String()),
  scope: Type.Optional(Type.String()),
  actor_id: Type.Optional(Type.String()),
  mutation_id: Type.Optional(Type.String()),
  signature: Type.Optional(Type.String()),
});

export default definePluginEntry({
  id: 'engram',
  name: 'ENGRAM Memory',
  description: 'First-class OpenClaw integration for ENGRAM governed memory.',
  register(api) {
    const config = resolveEngramConfig(api.pluginConfig);
    const client = createEngramClient(config);

    api.registerMemoryPromptSupplement(() => [
      'ENGRAM memory is available as a governed, cited long-term memory corpus.',
      'Use memory_search for normal recall; ENGRAM results may appear with engram:// paths and provenance hashes.',
      'Use engram_search / engram_get / engram_curate when you need explicit ENGRAM operations.',
    ]);

    api.registerMemoryCorpusSupplement({
      async search(params) {
        const result = await client.search({
          query: params.query,
          limit: params.maxResults ?? config.defaultLimit,
          status: config.defaultStatus,
          minConfidence: config.defaultMinConfidence,
        });
        return result.results.map(toCorpusSearchResult);
      },
      async get(params) {
        const lookup = String(params.lookup || '').replace(/^engram:\/\/memory\//, '');
        if (!lookup) return null;
        const memory = await client.get(lookup);
        return toCorpusGetResult(memory);
      },
    });

    api.registerTool({
      name: 'engram_status',
      label: 'ENGRAM Status',
      description: 'Check ENGRAM daemon health using HTTP first and engramctl fallback when enabled.',
      parameters: Type.Object({}),
      async execute() {
        const health = await client.health();
        return { content: [{ type: 'text', text: JSON.stringify(health, null, 2) }], details: health };
      },
    });

    api.registerTool({
      name: 'engram_search',
      label: 'ENGRAM Search',
      description: 'Search governed ENGRAM memory with citations, quality filters, and safe CLI fallback.',
      parameters: SearchSchema,
      async execute(_toolCallId, params) {
        const result = await client.search(params);
        return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }], details: result };
      },
    });

    api.registerTool({
      name: 'engram_get',
      label: 'ENGRAM Get',
      description: 'Read one ENGRAM memory object by object_id.',
      parameters: GetSchema,
      async execute(_toolCallId, params) {
        const memory = await client.get(params.object_id);
        return { content: [{ type: 'text', text: JSON.stringify(memory, null, 2) }], details: memory };
      },
    });

    api.registerTool({
      name: 'engram_curate',
      label: 'ENGRAM Curate',
      description: 'Write a governed ENGRAM memory object. Requires a write/admin ENGRAM token or CLI config.',
      parameters: CurateSchema,
      async execute(_toolCallId, params) {
        const memory = await client.curate(params);
        return { content: [{ type: 'text', text: JSON.stringify(memory, null, 2) }], details: memory };
      },
    }, { optional: true });
  },
});
