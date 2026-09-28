# Optional OAuth development client

Run from the MCP repository:

```sh
make oauth-client
# Or target a configured test installation:
make oauth-client MCP_URL=https://your-deployment.example/mcp/private
```

Requires Node.js 22.19 or newer and npm. The pinned official Inspector owns
metadata discovery, public client registration, state, PKCE, callback, token
exchange and MCP requests. The wrapper only provides a catalog and isolated
process state. It does not run with the production server or implement login.

1. Open the loopback URL printed at startup. Keep its session token private.
2. Select MyScoutee and connect. Inspector registers its callback automatically;
   do not create a manual client key for this automatic path.
3. MyScoutee opens its existing sign-in and consent UI. In dev use the normal
   demo login. With Firebase configured, use the deployment's normal login.
4. Check the displayed profile/group, approve, and return to Inspector.
5. Inspect tools and invoke an allowed read operation. Record its actual result.
6. Revoke this run's OAuth permission in MyScoutee and verify access fails.

Inspector listens on loopback port 6274 (sandbox 6275). Its callbacks are
`http://127.0.0.1:6274/oauth/callback` or `http://localhost:6274/oauth/callback`,
matching the browser host. HTTP targets are allowed only on loopback; remote
installations require HTTPS. There is no dummy callback or pre-injected grant.

During fixture-backed QA, first stage the versioned backend checkpoint and
launch its managed browsers with the repository launcher. The current minimal
`mcp-oauth` fixture covers development of block 7 only; it does not stage the
other API-group checks or authorize a full-QA PASS.

Stop with Ctrl+C. The wrapper removes its temporary Inspector state; revoke
the server-side grant and reset the test fixture separately. Browser site data
is managed by the checkpoint browser launcher. Startup success alone does not
prove login, callback or authenticated private-tool execution.
