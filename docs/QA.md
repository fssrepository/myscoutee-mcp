# MCP manual checks — NOT RUN

Extend the existing API and help regression micros; this is not a PASS ledger.

- Open the app help in HU/EN, call the MCP topic list, then read the same topic:
  section content and active revision match; all 17 seeded topics are present.
- Activate an operator-edited help revision. The next MCP read reflects it without
  copying seeds or restarting. Empty/missing help reports an error.
- Connect the anonymous HTTP endpoint from ChatGPT/Claude where custom MCP is
  available: help works; personal API tools are absent.
- Without any credentials, inspect initialization, tools/list and resources/list:
  they explicitly describe anonymous help without login or an API key. Ask for
  help and verify the client can call it without prompting for credentials.
- In HU/EN app help, AI integration is the second topic. Verify the profile editor
  → Affiliate → Client keys → AI / MCP connections instructions. Verify the
  callback, client-ID, one-time-secret and consent instructions in each client.
- Start stdio with a dedicated API key/client ID. Event/encounter queries observe
  the same ownership/group restrictions as the API. Cross-group/foreign IDs fail.
- Request event/participant creation in another group: discovery explains that
  the connection needs that group's profile key. Neither the prompt nor the app's
  active-group selector changes the key's scope. Discovery's profile/group must
  match the API handshake; only its authorized operation subset is listed. Select a correctly
  configured connection outside chat; never paste credentials into the chat.
- Base-profile asset creation works; group-profile asset creation remains rejected.
- Create event/group participant invites with the existing API permissions; links
  retain affiliate attribution and existing registration/join behavior.
- Revoke the key, then retry: the next personal operation fails; no stale access.
- API 401/403 responses remain MCP errors without retries, alternate identity or
  leaked backend response details. Attempts to override token, client, user or
  group through tool arguments are rejected before reaching the API.
- Change the API policy or group-admin membership and list tools again: the
  relevant tool or invitation scope disappears without restarting MCP. An API
  without capability metadata must not cause all personal tools to be advertised.
- Start HTTP with a private key configured: startup must fail, never expose it.
- Check partial batch results and unknown write outcomes: no automatic blind retry.
- Install a privately built plugin and ask for help: it reads the active guide and
  gives focused steps. It must not claim that undocumented/private tools exist.
- Build OpenAI and Claude targets independently: common skill and original
  MyScoutee logo are identical, platform manifests/config paths are valid, ZIPs
  contain no secrets. Remote-help advertises help only; remote-private requires OAuth and advertises
  only the API-authorized operation subset. Live client/store
  validation remains NOT RUN; building does not publish or install.

## Personal remote OAuth and connections — NOT RUN

- In HU/EN and local/HTTP modes create an MCP connection below API keys. Check
  name, callback, URL, client ID, one-time secret and expiry; reload hides the
  secret and retains the row. Other profiles cannot see it. Three active slots
  are separate from API/admin keys. Revoke then create again.
- Local/demo clearly says remote access is unavailable; reload still retains
  metadata. No demo credentials or session-mode headers grant real API access.
- In ChatGPT and Claude advanced connector setup, use the exact callback URL
  and client credentials with `/mcp/private`. Missing login uses normal login
  and returns to consent. Confirm the displayed profile/group. Cancel returns
  denial to the validated callback; approve returns state and issuer intact.
- Test bad callback, foreign account, wrong resource, expired/reused code,
  missing/wrong S256 verifier and wrong client secret: no grant or private data.
- Two users connected simultaneously receive their own discovery and results.
  No credential reaches anonymous help or a different upstream URL.
- Keep an enrolled browser session for the owner, then call MCP without a browser
  session header. It works; browser logout does not revoke the separate OAuth
  grant. Normal app routes still enforce their browser-session checks.
- Refresh an expired access token once; reuse the old refresh token: the family
  is rejected. Revoke the app row while connected: next discovery and call fail.
- Change API permissions/group membership: the next tools/list changes and
  unauthorized writes remain blocked. Switch the app’s active group: the
  connection retains its issuing profile. Check base assets and group events.
- Create/revoke updates the visible table without reload. A failed revoke keeps
  the row retryable. Change profiles during a pending create: no old secret or
  row appears in the new profile. Mobile layout and consent Back/X need real QA.

Automated OAuth/unit/transport checks do not mark these live client cases PASS.

## MCP Events — NOT RUN

MSC-116's stable execution checklist is in the backend repository at
`guides/qa/checklists/api/MCP-EVENTS-001.md`. It covers MCP 2.0 discovery, shared DCR
registration with independent user grants, signed challenge/rotation, restart and
expiry, resource authorization/revocation, SSRF/DNS rebinding, bounded retries,
virtual-thread concurrency, callback rate throttling, queue overflow and ChatGPT
Work/Cloud receipt. Go HTTP adapter and Java protocol regression sources were added;
source compilation does not mark execution or live plugin interoperability PASS.
