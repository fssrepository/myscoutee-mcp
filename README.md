# MyScoutee MCP

Go MCP adapter using the official MCP SDK. The Docker image is a static binary
and CA certificates, without a JVM. Server and unpublished plugin live in this
repository, with separate build targets. Nothing is published by a build.

## Capabilities

| Connection | Tools | Identity |
| --- | --- | --- |
| Remote HTTP | `list_help_topics`, `read_help_topic`, `search_help`; HU/EN help resources | Anonymous, read-only help |
| Local stdio without a key | Same help tools | Anonymous |
| Local stdio with a key | Help plus the API-authorized subset of `event`, `encounters`, `createAssets`, `createEvents`, `createCampaigns` (Work only), `inviteParticipants`, `inviteGroupParticipants` | Existing integration API key and its client ID |

Help is fetched on demand from `GET /api/help/active?lang=hu|en`, the application's
existing active-revision API. There is **no help copy in this repository**.
Responses contain section IDs, titles, summaries, readable content, source URL,
language and revision. A missing/unavailable revision is an error, never a cached
fallback. Resource URIs are `myscoutee://help/en` and `myscoutee://help/hu`.

The common help seed is maintained in the backend repository, in its existing
Mongo JSON files and frontend local defaults. Mongo startup uses the existing
importer. Operator-edited active revisions are read too.

API schemas are generated from `myscoutee-client/openapi.yaml`. Permissions,
profile/group ownership, invitation attribution and write rules stay in the
existing backend. No per-call group header, caller-selected identity, direct
Mongo access or duplicate business logic. Assets require base-profile access;
events retain their existing API profile scope. Use a separate key/client ID per
local client, respecting the API's client binding. Multipart local-file uploads
and webhook configuration are not exposed as MCP tools in this version.

Personal discovery calls the existing authenticated `POST /integrations/v1/connect`.
Its response identifies the issuing profile/group, available operation IDs and
administered invitation groups. The MCP intersects that list with its implemented
tools; group keys do not advertise asset creation. No write endpoint is probed.
Capabilities are read again for each tool list and personal call, without an
authorization cache. Discovery is private and immediately stale. If the API is
unavailable, rejects the key or lacks capability metadata, only anonymous help is
listed; private calls fail closed. Resource ownership, lifecycle and per-resource
grants still apply at execution. Tool availability is not a promise that every
resource ID or payload will be accepted.

## Build and test

Requires Go 1.26 and Docker for an image build:

```sh
make test
make build
make image
```

`make contract-check` verifies the generated API schema against the sibling
`myscoutee-client` checkout (Python 3 + PyYAML). After a canonical API change, run
`python3 tools/sync-api-contract.py` and review the generated contract. Normal Go
and Docker builds do not require that sibling checkout.

## Remote help

Set the selected mode's API URL variable to the existing API root ending in `/api` (HTTPS;
anonymous help may use HTTP on the private Compose network; personal credentials
require HTTPS except on loopback). Select the Compose environment explicitly:

```sh
# Development: no automatic restart; also the default for plain docker compose.
MYSCOUTEE_DEV_API_BASE_URL=https://your-dev-deployment.example/api \
  docker compose -f compose.dev.yaml up -d --build
```

For E2E, point explicitly at the backend under test:

```sh
MYSCOUTEE_E2E_API_BASE_URL=https://your-e2e-deployment.example/api \
  docker compose -f compose.e2e.yaml up -d --build
```

For production:

```sh
MYSCOUTEE_API_BASE_URL=https://your-deployment.example/api \
  docker compose -f compose.prod.yaml up -d --build
```

| Mode | Image override | API URL variable | Default host port | Restart |
| --- | --- | --- | --- | --- |
| Dev | `MCP_DEV_IMAGE` (`1.3.0-dev`) | `MYSCOUTEE_DEV_API_BASE_URL` | 8091 | `no` |
| E2E | `MCP_E2E_IMAGE` (`1.3.0-e2e`) | `MYSCOUTEE_E2E_API_BASE_URL` | 18091 | `no` |
| Prod | `MCP_IMAGE` (`1.3.0-prod`) | `MYSCOUTEE_API_BASE_URL` | 8091 | `unless-stopped` |

Each mode has a separate Compose project. The default project remains
`myscoutee-mcp` for existing development usage; E2E and production use
`myscoutee-mcp-e2e` and `myscoutee-mcp-prod`. `MCP_DEV_PORT`, `MCP_E2E_PORT` and
`MCP_PROD_PORT` override their respective loopback ports. Set different ports
when running dev and production on the same host. All modes reuse
`compose.common.yaml` for service settings. A production image/API variable
does not silently select the development or E2E upstream.
Use the same `-f` selection for subsequent `logs`, `stop` and `down` commands.
The backend's integrated dev/E2E/production stacks retain their own Compose
entrypoints; these files run the standalone MCP service.

The host ports bind only to `127.0.0.1`. Proxy `/mcp` through the deployment's
existing HTTPS reverse proxy, preserving headers and the path. `/healthz` is a
process health endpoint; it does not claim upstream API health. The service runs
unprivileged, with a read-only filesystem, dropped capabilities and a 128 MiB
memory limit. These are deployment defaults, not a benchmark guarantee.

For a direct binary deployment set `MYSCOUTEE_MCP_TRANSPORT=http`; the default
listen address is `127.0.0.1:8080`. `MYSCOUTEE_MCP_LISTEN` overrides the bind address.

## Local API tools

The client starts its own stdio process. Set these environment variables in the
local MCP client configuration, **never in a chat or committed file**:

- `MYSCOUTEE_API_BASE_URL`: HTTPS API root ending in `/api`.
- `MYSCOUTEE_API_TOKEN`: existing profile integration key.
- `MYSCOUTEE_CLIENT_ID`: that key's stable client UUID.

Run `build/myscoutee-mcp` or the plugin's Docker command. With no key, only help is
advertised. Supplying only part of the identity fails startup. Public HTTP refuses
a process-wide private key: it cannot expose one user's operations to everybody.

## Remote personal connections

Use the deployed HTTPS `/mcp/private` endpoint. The anonymous `/mcp` endpoint
continues to provide help without credentials.

1. Copy the MCP URL from Profile editor → Affiliate → AI / MCP connections
   into the external client's connection settings and choose Connect.
2. Sign in through the normal MyScoutee login, check the displayed profile/group,
   and approve access. An existing signed-in session can go directly to approval.
   Public dynamic registration uses S256 PKCE and needs no copied client secret.
3. Return to the client and use its authenticated connection. The approved
   profile stays fixed; changing groups in the app does not retarget it.
4. Revoke that connection under **OAuth permissions** in the main app popup.
   Access and refresh tokens stop working. Connections expire after 90 days.

For a client requiring predefined credentials, **Advanced** contains the manual
MCP client-key table and Generate action. Generate immediately adds the client
ID and one-time secret to the table; copy them into the client configuration.
There is no name/callback form. Deployment configuration
`config.mcp-manual-redirect-uris` supplies exact trusted callback URLs for newly
generated predefined clients. An empty list permits no callback; it does not
accept arbitrary destinations. Existing clients with a registered callback retain
that exact restriction. Dev preconfigures only the two loopback Inspector callbacks. Those
keys have their own revoke actions and are separate from automatically registered
OAuth permissions. Never paste credentials into chat.

Google/Firebase/email are installation-configured MyScoutee sign-in methods.
The external client receives MyScoutee OAuth tokens, never the Google/Firebase
login token. The existing dev backend supports demo sign-in when explicitly
enabled; browser-only IndexedDB local mode cannot serve external OAuth.

The integration-key owner manages profile-bound MCP permissions (separate `mcp`
scope). Public registration stores client metadata without granting profile access.
Authorization codes are one-use, five-minute, S256 PKCE-bound records. The token
endpoint supports public `none`, `client_secret_basic` and `client_secret_post`,
resource-bound one-hour access tokens and rotating refresh tokens; replay revokes
the refresh family. Only token hashes persist. Both discovery and execution use
the existing API authentication, membership and operation checks. Ordinary API
keys are not remote OAuth bearer tokens. Tokens never reach help requests.

Nginx routes protected-resource and authorization-server metadata, bounded
`/api/mcp/oauth/register`, `/api/mcp/oauth/token`, `/mcp/private`, and the existing
SPA authorization route `/mcp/authorize`. The configured integration public origin
and `MYSCOUTEE_PUBLIC_API_BASE_URL` must describe the same installation. Caller
Host headers cannot choose the issuer. CIMD and a token-revocation protocol endpoint
are not advertised; users revoke permissions through the app.

### Optional external development client

Run `make oauth-client` from this repository (Node.js 22.19+). This starts the
pinned official MCP Inspector separately from the production MCP server, with
`http://localhost/mcp/private` as its default target. For another installation,
use `make oauth-client MCP_URL=https://your-deployment.example/mcp/private`.
Follow the local URL printed by Inspector; its URL includes a session secret,
so do not publish the startup log. Select MyScoutee and connect. See
[client setup and cleanup](tools/oauth-client/README.md).

Inspector verification does not prove a ChatGPT or Claude browser conversation.
Actual host acceptance is a separate post-deployment smoke test. Backend QA
keeps its own isolated protocol/provider checks and evidence.

Protocol references: [MCP authorization](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization)
and [official Inspector](https://modelcontextprotocol.io/docs/tools/inspector).

## MCP Inspector

With the local dev stack running, run `make oauth-client` from this repository.
Open the loopback URL printed in the terminal, then switch **MyScoutee** on to
sign in and approve access. Stop with `Ctrl+C`.
For another installation: `make oauth-client MCP_URL=https://your-host/mcp/private`.

## Unpublished plugin

`plugins/myscoutee` contains the portable manifest, Codex compatibility manifest,
MCP configuration and a skill that directs the assistant to the live help first.
It does not contain a copied guide, credentials or invented registration IDs.

```sh
make plugin                             # local Docker/stdio package
make plugin PLUGIN_TARGET=claude        # Claude package only
make plugin MCP_URL=https://your-deployment.example/mcp  # remote help package
```

ZIPs are written to `build/`. `/mcp` builds a Read-only remote-help variant;
`/mcp/private` builds a remote-private variant requiring OAuth configuration.
The shared source builds separate OpenAI and Claude manifests with the existing
MyScoutee logo. See [plugin build and distribution](plugins/README.md).
Use the real deployed HTTPS URL when preparing that variant. Building neither
installs the plugin nor publishes it to a store/workspace. Registering a remote
connection and account-dependent developer-mode testing remain deployment steps.

## Verification boundaries

Automated tests cover actual MCP HTTP initialization/discovery/tools/resources,
active-revision changes, language/search, failed help reads, private-tool exclusion,
API identity/path handling, input-schema rejection, redirects and credential-safe
errors. Docker initialization/discovery is also smoke-tested during development.
Full deployed MyScoutee backend integration and user-operated mobile/ChatGPT/Claude
checks are separate NOT RUN cases; see `docs/QA.md`.
