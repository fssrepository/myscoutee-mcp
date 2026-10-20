.PHONY: test build image plugin plugin-sync plugin-check contract-check oauth-client
GO ?= go
MYSCOUTEE_VERSION ?= $(shell python3 -c 'import json; print(json.load(open("plugins/myscoutee/plugin.json"))["version"])')
export MYSCOUTEE_VERSION

test:
	$(GO) test -race ./...
build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w' -o build/myscoutee-mcp ./cmd/myscoutee-mcp
image:
	docker build -t myscoutee-mcp:$(MYSCOUTEE_VERSION) .
plugin:
	python3 plugins/build.py --target $(or $(PLUGIN_TARGET),all) $(if $(MCP_URL),--url '$(MCP_URL)',)
plugin-sync:
	python3 plugins/build.py --sync
plugin-check:
	python3 plugins/build.py --check-archive '$(PLUGIN_ZIP)' $(if $(SUBMISSION),--submission,)
contract-check:
	python3 tools/sync-api-contract.py --check

# Optional test client, deliberately separate from the production Go server.
oauth-client:
	cd tools/oauth-client && npm ci --ignore-scripts --no-audit --no-fund
	cd tools/oauth-client && npm start -- $(or $(MCP_URL),http://localhost/mcp/private)
