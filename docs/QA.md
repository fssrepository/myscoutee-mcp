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
  → Affiliate → Client keys instructions; remote personal access is explicitly
  marked unavailable rather than pointing to an unimplemented MCP switch.
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
  contain no secrets. Remote variants advertise help only. Live client/store
  validation remains NOT RUN; building does not publish or install.

Remote OAuth personal access and the MCP connections table remain unimplemented;
do not mark their cases passed from local API-key tests.
