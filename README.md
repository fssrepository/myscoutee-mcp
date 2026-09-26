# MyScoutee MCP

Go MCP adapter using the official MCP SDK. The Docker image is a static binary
and CA certificates, without a JVM. Server and unpublished plugin live in this
repository, with separate build targets. Nothing is published by a build.

## Capabilities

| Connection | Tools | Identity |
| --- | --- | --- |
| Remote HTTP | `list_help_topics`, `read_help_topic`, `search_help`; HU/EN help resources | Anonymous, read-only help |
| Local stdio without a key | Same help tools | Anonymous |
| Local stdio with a key | Help plus the API-authorized subset of `event`, `encounters`, `createAssets`, `createEvents`, `inviteParticipants`, `inviteGroupParticipants` | Existing integration API key and its client ID |

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

Set `MYSCOUTEE_API_BASE_URL` to the existing API root ending in `/api` (HTTPS;
anonymous help may use HTTP on the private Compose network; personal credentials
require HTTPS except on loopback). Start the optional Compose service:

```sh
MYSCOUTEE_API_BASE_URL=https://your-deployment.example/api docker compose up -d
```

The host port binds only to `127.0.0.1:8091`. Proxy `/mcp` through the deployment's
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

**Remote personal ChatGPT/Claude operations are not implemented yet.** They need
per-user OAuth, grant/revocation management and the app's MCP-connections table.
An ordinary integration key is not an OAuth access token and is not passed through
as one. The HTTP package currently connects for anonymous help only. No remote
private-operation or live ChatGPT/Claude QA pass is claimed.

## Unpublished plugin

`plugins/myscoutee` contains the portable manifest, Codex compatibility manifest,
MCP configuration and a skill that directs the assistant to the live help first.
It does not contain a copied guide, credentials or invented registration IDs.

```sh
make plugin                             # local Docker/stdio package
make plugin PLUGIN_TARGET=claude        # Claude package only
make plugin MCP_URL=https://your-deployment.example/mcp  # remote help package
```

ZIPs are written to `build/`. The remote variant advertises Read capability only.
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
Full MyScoutee backend integration, OAuth, and user-operated mobile/ChatGPT/Claude
checks are separate NOT RUN cases; see `docs/QA.md`.
