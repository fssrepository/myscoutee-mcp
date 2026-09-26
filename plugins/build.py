#!/usr/bin/env python3
"""Package shared plugin sources for OpenAI or Claude. Never installs or publishes."""
import argparse
import json
import shutil
import zipfile
from pathlib import Path
from urllib.parse import urlsplit


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", choices=("all", "openai", "claude"), default="all")
    parser.add_argument("--url", help="Public HTTPS MCP URL; omit for local Docker stdio")
    args = parser.parse_args()
    if args.url:
        url = urlsplit(args.url)
        if (url.scheme != "https" or not url.hostname or url.username or url.password
                or url.query or url.fragment):
            parser.error("MCP URL must use HTTPS without credentials, query or fragment")

    root = Path(__file__).resolve().parents[1]
    source = root / "plugins/myscoutee"
    manifest = json.loads((source / "plugin.json").read_text())
    variant = "remote-help" if args.url else "local"
    targets = ("openai", "claude") if args.target == "all" else (args.target,)
    for target in targets:
        output = root / "build/plugins" / target / variant / "myscoutee"
        if output.exists():
            shutil.rmtree(output)
        output.mkdir(parents=True)
        shutil.copytree(source / "skills", output / "skills")
        shutil.copytree(source / "assets", output / "assets")
        if target == "openai":
            for name in ("plugin.json", "mcp.json", ".mcp.json"):
                shutil.copyfile(source / name, output / name)
            shutil.copytree(source / ".codex-plugin", output / ".codex-plugin")
        else:
            claude = {key: manifest[key] for key in
                      ("name", "version", "description", "author", "repository")}
            if args.url:
                claude["description"] = "Current MyScoutee help in English and Hungarian."
            write_json(output / ".claude-plugin/plugin.json", claude)
            shutil.copyfile(source / ".mcp.json", output / ".mcp.json")

        if args.url:
            write_json(output / ".mcp.json", {"mcpServers": {
                "myscoutee": {"type": "http", "url": args.url}}})
            if target == "openai":
                write_json(output / "mcp.json", {
                    "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
                    "mcpServers": {"myscoutee": {"type": "streamable-http", "url": args.url}}})
                for name in ("plugin.json", ".codex-plugin/plugin.json"):
                    path = output / name
                    data = json.loads(path.read_text())
                    data["description"] = "Current MyScoutee help in English and Hungarian."
                    interface = data.get("interface") or data["extensions"]["com.openai"]["interface"]
                    interface.update(
                        capabilities=["Read"], shortDescription="Current MyScoutee help.",
                        longDescription="Read the application's active help. This remote connection does not expose personal API operations.",
                        defaultPrompt=["Explain how MyScoutee groups work.", "Hogyan kérhetek közvetlen chatengedélyt?"])
                    write_json(path, data)

        archive = root / "build" / f"myscoutee-plugin-{target}-{variant}-{manifest['version']}.zip"
        with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as bundle:
            for path in sorted(output.rglob("*")):
                if path.is_file():
                    bundle.write(path, Path("myscoutee") / path.relative_to(output))
        print(archive)


if __name__ == "__main__":
    main()
