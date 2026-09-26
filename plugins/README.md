# Plugin packages (unpublished)

`myscoutee/` is the shared skill, metadata and MCP configuration. `build.py`
creates separate target packages without duplicating the MCP implementation or
help content. No command here installs, publishes or changes a marketplace.

```sh
# From the MCP repository root: build both local Docker/stdio packages.
make plugin
make plugin PLUGIN_TARGET=claude
make plugin PLUGIN_TARGET=openai

# Remote anonymous-help packages: replace with your deployed public endpoint.
make plugin MCP_URL=https://your-deployment.example/mcp
```

Outputs: `build/plugins/{openai,claude}/{local,remote-help}/myscoutee/` and
`build/myscoutee-plugin-{target}-{variant}-{version}.zip`. The version comes from
the common manifest. Build does not bundle credentials or the server image.

| Target | Manifest | Distribution |
| --- | --- | --- |
| OpenAI | Root `plugin.json` + `mcp.json`, Codex compatibility overlay | ChatGPT/Codex plugin submission; public MCP submissions require a deployed HTTPS endpoint |
| Claude | `.claude-plugin/plugin.json` + `.mcp.json` | Claude Code plugin directory/package; subsequent marketplace publication is separate |

The local package needs Docker, the locally built `myscoutee-mcp:1.3.0` image and
the API URL/key/client-ID environment variables described in the main README.
The remote package currently exposes anonymous help only. Personal remote
operations still require the unimplemented per-user OAuth flow. A Claude Code
plugin ZIP does not imply claude.ai connector-directory approval.

Both packages include the existing MyScoutee app logo in `assets/logo.png`.
OpenAI's `logo` and `composerIcon` fields reference it. Claude's documented plugin
manifest has no logo field; use the bundled image wherever the publication UI
accepts a logo rather than adding an unsupported manifest field.

For local validation, use the target's plugin validator. For OpenAI, register the
real MCP endpoint and follow the submission flow; do not put fabricated
`plugin_asdk_app` IDs into the package. Live client installation and store approval
are not covered by the package build.

Sources checked 2026-09-26:
- [OpenAI plugin packaging](https://developers.openai.com/plugins/build/plugins)
- [Claude plugin manifest](https://code.claude.com/docs/en/plugins-reference)
- [Claude marketplace distribution](https://code.claude.com/docs/en/plugin-marketplaces)
