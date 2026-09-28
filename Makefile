.PHONY: test build image plugin contract-check oauth-client
GO ?= go

test:
	$(GO) test -race ./...
build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w' -o build/myscoutee-mcp ./cmd/myscoutee-mcp
image:
	docker build -t myscoutee-mcp:1.3.0 .
plugin:
	python3 plugins/build.py --target $(or $(PLUGIN_TARGET),all) $(if $(MCP_URL),--url '$(MCP_URL)',)
contract-check:
	python3 tools/sync-api-contract.py --check

# Optional test client, deliberately separate from the production Go server.
oauth-client:
	cd tools/oauth-client && npm ci --ignore-scripts --no-audit --no-fund
	cd tools/oauth-client && npm start -- $(or $(MCP_URL),http://localhost/mcp/private)
