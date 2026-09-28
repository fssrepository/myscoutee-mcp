#!/usr/bin/env node
// The official Inspector owns discovery, DCR, state, PKCE, callback and MCP.
// This wrapper only isolates the optional QA client's lifetime and state.
import { spawn } from 'node:child_process';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const input = process.argv[2] ?? 'http://localhost/mcp/private';
const url = new URL(input);
if (url.username || url.password || url.hash || url.search ||
    (url.protocol !== 'https:' && !(url.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname)))) {
  throw new Error('Use an HTTPS MCP URL, or HTTP on loopback for development.');
}
const state = await mkdtemp(join(tmpdir(), 'myscoutee-oauth-client-'));
const catalog = join(state, 'catalog.json');
await writeFile(catalog, JSON.stringify({ mcpServers: { MyScoutee: {
  type: 'http', url: url.href, protocolEra: 'legacy'
} } }), { mode: 0o600 });
const launcher = fileURLToPath(new URL('./node_modules/@modelcontextprotocol/inspector/clients/launcher/build/index.js', import.meta.url));
const child = spawn(process.execPath, [launcher, '--web', '--catalog', catalog], {
  stdio: 'inherit',
  env: { ...process.env, HOST: '127.0.0.1', CLIENT_PORT: '6274',
    MCP_AUTO_OPEN_ENABLED: 'false', MCP_STORAGE_DIR: state,
    MCP_CATALOG_PATH: join(state, 'catalog.json'), DANGEROUSLY_OMIT_AUTH: 'false',
    MCP_CLIENT_CONFIG_PATH: join(state, 'client.json'),
    MCP_INSPECTOR_SECRET_STORE: 'memory' }
});
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => child.kill(signal));
child.once('error', async error => {
  await rm(state, { recursive: true, force: true });
  console.error(error.message); process.exitCode = 1;
});
child.once('exit', async code => {
  await rm(state, { recursive: true, force: true });
  process.exitCode = code ?? 0;
});
