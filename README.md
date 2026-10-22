# Shared MCP adapter

One Go implementation for MCP HTTP/stdio transport, anonymous live help,
per-connection OAuth isolation, input validation, live backend capability checks
and MCP Events forwarding. The consuming application owns identity, permissions,
business logic and product configuration.

## Two application-owned files

Each consuming repository supplies `config/mcp/product.json` and
`config/mcp/contract.json`. No product identity, instructions or API catalog is
compiled into the shared adapter.

`product.json` requires `name`, `title`, `version`, `helpResourcePrefix`,
`instructions` and `authorizationScope`. The help prefix is an absolute URI
ending in `/`; `en` and `hu` select the existing shared help protocol.

`contract.json` is an array of operations with `name`, `method`, `path`,
`description`, `schema` and `parameters`. These are the existing backend API input
schemas, not a new business API. Optional `guidance` appends operation-specific
instructions; `destructiveHint` describes effects; `authorizedGroupArgument`
checks a configured argument against the handshake's authorized invitation groups.
No operation name selects special behavior in the adapter.

The selected catalog is always intersected with the authenticated backend's
current operations. Configuring a tool never grants permission. Missing or invalid
configuration fails startup; there is no fallback to another application's data.
Files are loaded once per process, then reused without per-request file reads.
Use `[]` as the catalog for a help-only product.

## Build and run

Go 1.26 is required. Tests use synthetic fixtures and need no application checkout.

```sh
make test
make build
MCP_CONFIG_DIR=/absolute/path/to/application/config/mcp \
  MYSCOUTEE_API_BASE_URL=https://your-deployment.example/api \
  build/myscoutee-mcp
```

The established `MYSCOUTEE_*` environment names and integration wire protocol are
retained for client compatibility. `MYSCOUTEE_MCP_TRANSPORT` selects `stdio`
(default) or `http`; `MYSCOUTEE_MCP_LISTEN` defaults to `127.0.0.1:8080`.
HTTP refuses process-wide personal keys. Local stdio may use
`MYSCOUTEE_API_TOKEN` and `MYSCOUTEE_CLIENT_ID`, configured outside chat.
`MYSCOUTEE_PUBLIC_API_BASE_URL` supplies the public OAuth metadata origin.

The shared Dockerfile takes the two files through a named build context and
packages them with the common binary. No consuming repository copies Go sources
or maintains its own Dockerfile.

```sh
docker build --build-context mcp-config=/absolute/path/to/application/config/mcp \
  -t application-mcp:version .
```

Compose consumers set `build.context` to their pinned `mcp` submodule and
`build.additional_contexts.mcp-config` to their own `config/mcp` directory.
The image sets `MCP_CONFIG_DIR=/etc/mcp`; installed and KVM deployments need no
source/config mounts. The image contains only the binary, CA certificates and the
two selected JSON files, and runs unprivileged.

The standalone `compose.dev.yaml`, `compose.e2e.yaml` and `compose.prod.yaml`
likewise require `MCP_CONFIG_DIR` when building. Use the same `-f` selection for
later lifecycle commands. These files are not launched by consuming projects.

## Update an application's contract

The application owns its operation selection and annotations. The common schema
generator takes an explicit OpenAPI source and destination, preserving selected
operation metadata. Python 3 and PyYAML are required.

```sh
python3 tools/sync-api-contract.py \
  --source /path/to/openapi.yaml --contract /path/to/config/mcp/contract.json
python3 tools/sync-api-contract.py --check \
  --source /path/to/openapi.yaml --contract /path/to/config/mcp/contract.json
```

## Protocol boundaries

Anonymous `/mcp` reads current help from the backend's `/help/active` API.
Personal `/mcp/private` binds every request to a backend-verified OAuth token.
Discovery and invocation check current capabilities; no authorization cache,
caller-selected identity, direct database access or write probes are used.

Events use the same OAuth identity and forward the backend's current definitions
and subscription decisions. They add no local business rules or permission grants.
The backend remains responsible for delivery, revocation and resource ownership.

Source tests do not qualify an external AI host or a deployed application flow.
