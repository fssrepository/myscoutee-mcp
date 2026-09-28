#!/usr/bin/env node
// The official Inspector owns discovery, DCR, state, PKCE, callback and MCP.
// This wrapper only isolates the optional QA client's lifetime and state.
import { spawn } from 'node:child_process';
import { mkdtemp, rm, writeFile, readFile, readdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';

const input = process.argv[2] ?? 'http://localhost/mcp/private';
const url = new URL(input);
if (url.username || url.password || url.hash || url.search ||
    (url.protocol !== 'https:' && !(url.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname)))) {
  throw new Error('Use an HTTPS MCP URL, or HTTP on loopback for development.');
}
// Inspector 2.8.0 forces a 1280px root. Keep its original client and OAuth
// implementation; apply only our versioned responsive presentation override.
const assets = fileURLToPath(new URL('./node_modules/@modelcontextprotocol/inspector/clients/web/dist/assets/', import.meta.url));
const marker = '/* MyScoutee responsive Inspector */';
const responsive = await readFile(new URL('./responsive.css', import.meta.url), 'utf8');
const styles = (await readdir(assets)).filter(name => /^index-.*\.css$/.test(name));
if (styles.length !== 1) throw new Error('Inspector stylesheet changed; review the responsive override.');
const stylesheet = join(assets, styles[0]);
const original = (await readFile(stylesheet, 'utf8')).split(marker)[0].trimEnd();
await writeFile(stylesheet, `${original}\n${marker}\n${responsive}`);
const indexPath = join(assets, '..', 'index.html');
const index = await readFile(indexPath, 'utf8');
const revision = createHash('sha256').update(responsive).digest('hex').slice(0, 12);
await writeFile(indexPath, index.replace(/(href="\/assets\/index-[^"?]+\.css)(?:\?[^" ]*)?"/g,
  `$1?myscoutee-responsive=${revision}"`));
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
