# MyScoutee MCP

Go MCP adapter using the official MCP SDK. The Docker image is a static binary
and CA certificates, without a JVM. Server and unpublished plugin live in this
repository, with separate build targets. Nothing is published by a build.

## Capabilities

| Connection | Tools | Identity |
| --- | --- | --- |
| Remote HTTP | `list_help_topics`, `read_help_topic`, `search_help`; HU/EN help resources | Anonymous, read-only help |
| Local stdio without a key | Same help tools | Anonymous |
| Local stdio with a key | Help plus the API-authorized subset of the 40 supported operations | Existing integration API key and its client ID |

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

## Access packs (MSC-119 / 1.6.0)

The existing shield popup offers Blocked / Write / Read / Full. The main MCP link
starts Blocked unless an explicit preference was saved; generating a client keeps
its existing behavior and does not enable that main link. New client packs remain
Write. Main and client packs cap each other. Confirming a selector only stages the
parent draft; the parent tick saves all changed packs in one settings request.

Write allows the existing organizing commands and their necessary results. Read
adds permitted personal/workspace queries. Full adds OAuth MCP Events; it grants
no additional human-decision or administrative writes. Anonymous help is separate.

Admin/operator profiles expose read-only role tools: adminOverview,
operatorMeasurements and operatorRevenueReports (paged, 1–100). The backend checks
the current role and Read/Full pack on discovery and invocation. No private member
profiles, moderation actions, deployments, payments or configuration commands are
introduced. Existing desktop admin-key overview access is retained and can be blocked.


## MCP Events (MSC-116 / 1.6.0)

The OAuth `/mcp/private` endpoint implements MCP 2.0 (`2026-07-28`) discovery
and `events/list`, `events/subscribe`, `events/unsubscribe` using SDK custom-method
extension points. It uses the same identity as tools. DCR registers client metadata;
each person signs in and consents to their own profile. The publisher's account
is not shared. Authorization code + S256 PKCE, resource binding, rotating refresh
tokens and existing revocation remain with the backend OAuth service.

Events require **Full** on the main MCP link and client. Member definitions are
`event.changed`, `event.encounters` and `asset.changed`, with required `resourceId`
(the API's event/asset `externalId`). Only organizer-owned events and base-profile
owned assets qualify. Payloads contain compact status summaries; encounter results
use revision/total and the existing paged `encounters` tool. These methods are
protocol subscriptions, not extra AI write tools. Static-key stdio and anonymous
help do not advertise Events. Embedded review UI is still a separate follow-up.

The backend persists subscriptions and reuses its webhook delivery queue. Callbacks
must use public HTTPS on port 443, echo a signed challenge and accept Standard
Webhooks signatures. Signing keys are encrypted with the deployment's existing
configuration key; include that key in the existing protected backup procedure.
Verification is cached for five minutes per principal/callback/key. Refresh rotates
secrets with a two-minute dual-signature overlap. Default/max lifetime is one day;
shorter positive `ttlMs` values are honored. No replay: cursor is null. Expired or
unsubscribed generations cannot deliver old queued events. Revocation, Full downgrade,
group membership and resource ownership are checked before delivery.

Delivery is at least once: IDs/body survive retries, timestamps/signatures are
fresh. The existing three-attempt schedule applies; 410 disables the subscription,
413/permanent 4xx do not retry. The common virtual-thread workers retain a concurrency
semaphore. Events additionally defaults to 4 starts/sec, burst 4, 8 concurrent
callbacks and 2 simultaneous verifications. The token bucket is per backend process;
multiple replicas multiply that deployment rate. Configure
`INTEGRATION_EVENTS_REQUESTS_PER_SECOND`, `INTEGRATION_EVENTS_BURST` and
`INTEGRATION_EVENTS_MAX_CONCURRENT` in the normal backend environment.

Subscription limit is 100/profile. Queue admission ceilings are 1000 Events rows
per profile and 10000 per database; concurrent publishers can occupy previously
admitted slots. Overflow does not fail the original app action: it records a gap,
reported as `truncated: true` on refresh. Events deliveries expire after seven days;
subscriptions are retained seven days after expiry. Startup uses the existing Mongo
index initializer. API `/watch` remains its separate, compatible callback protocol.

Runtime ChatGPT callback/lifecycle QA and load testing are **NOT RUN**; see the
backend's `guides/qa/checklists/api/MCP-EVENTS-001.md` and this repository's QA notes.

Product identity/version/help guidance live in `internal/server/product.go`;
`internal/api/contract.json` is the generated supported API catalog. A derivative
application selects those build inputs while reusing the same transport and live
capability checks. Neither input grants access: the backend remains authoritative.

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

Development defaults to the existing local backend at `http://localhost/api`.
Inside the container, `host.docker.internal` reaches the host, including on Linux
through the configured host-gateway mapping. Start the standalone dev service with:

```sh
# Development: no automatic restart; also the default for plain docker compose.
docker compose -f compose.dev.yaml up -d --build
```

To use another development backend, set the optional `MYSCOUTEE_DEV_API_BASE_URL`
to its API root ending in `/api`; it also becomes the public API address.
Use HTTPS for remote deployments. Anonymous help may use HTTP on the private
Compose network; personal credentials require HTTPS except on loopback.

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
| Dev | `MCP_DEV_IMAGE` (`1.6.0-dev`) | `MYSCOUTEE_DEV_API_BASE_URL` (optional; local API by default) | 8091 | `no` |
| E2E | `MCP_E2E_IMAGE` (`1.6.0-e2e`) | `MYSCOUTEE_E2E_API_BASE_URL` | 18091 | `no` |
| Prod | `MCP_IMAGE` (`1.6.0-prod`) | `MYSCOUTEE_API_BASE_URL` | 8091 | `unless-stopped` |

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
The [publication handoff](plugins/PUBLICATION_HANDOFF.md) records completed dev
preparation, exact review prompts, source/ZIP checks and the remaining deployment,
policy, host-QA and publisher steps. The generated compatibility files are kept
in sync with `make plugin-sync`; every package build checks the actual ZIP.
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

### MSC-119 role Events and host availability

Full/Full role connections discover `admin.attention` and/or `operator.attention`,
according to their current roles, with `resourceId: node`. These summarize the
existing MSC-118 registration, report/feedback and release notifications as a kind.
The client may read authorized statistics afterwards. They reuse the durable queue,
signing, TTL, limits and current-role checks before delivery; there is no polling
loop or extra role-control endpoint. Full→Read stops Events and retains queries.

ChatGPT documents Events in Work chats on web and desktop Cloud. Ordinary MCP
support is distinct: Claude supports a Free custom connector, but its documentation
does not establish compatibility with this Events extension. Provider plan and
workspace restrictions apply; MyScoutee does not check AI subscriptions.

Sources: https://developers.openai.com/plugins/build/mcp-events and
https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp
