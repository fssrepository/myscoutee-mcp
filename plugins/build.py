#!/usr/bin/env python3
"""Package shared plugin sources for OpenAI or Claude. Never installs or publishes."""
import argparse
import json
import shutil
import zipfile
from copy import deepcopy
from pathlib import Path
from urllib.parse import urlsplit

from validate import validate_archive


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")


def compatibility(manifest, mcp):
    """The portable manifests own identity, presentation and connection settings."""
    codex = {key: deepcopy(manifest[key]) for key in
             ("name", "version", "description", "author", "repository")}
    codex.update(deepcopy(manifest["extensions"]["com.openai"]))
    codex.update(skills="./skills/", mcpServers="./.mcp.json")
    return {".codex-plugin/plugin.json": codex,
            ".mcp.json": {"mcpServers": deepcopy(mcp["mcpServers"])}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", choices=("all", "openai", "claude"), default="all")
    parser.add_argument("--url", help="Public HTTPS MCP URL; omit for local Docker stdio")
    parser.add_argument("--sync", action="store_true", help="Regenerate tracked compatibility manifests only")
    parser.add_argument("--check-archive", type=Path, help="Validate an existing ZIP without building")
    parser.add_argument("--submission", action="store_true", help="Also require public-submission metadata and remote HTTPS")
    args = parser.parse_args()
    if args.check_archive:
        problems = validate_archive(args.check_archive, submission=args.submission)
        if problems:
            parser.exit(1, "\n".join(problems) + "\n")
        print("PASS: archive structure and metadata (live checks remain separate)")
        return
    if args.sync and (args.url or args.submission):
        parser.error("--sync only regenerates source compatibility files")
    if args.url:
        url = urlsplit(args.url)
        if (url.scheme != "https" or not url.hostname or url.username or url.password
                or url.query or url.fragment):
            parser.error("MCP URL must use HTTPS without credentials, query or fragment")

    root = Path(__file__).resolve().parents[1]
    source = root / "plugins/myscoutee"
    manifest = json.loads((source / "plugin.json").read_text())
    mcp = json.loads((source / "mcp.json").read_text())
    generated = compatibility(manifest, mcp)
    if args.sync:
        for name, data in generated.items():
            write_json(source / name, data)
        print("Synchronized compatibility manifests from plugin.json and mcp.json")
        return
    for name, expected in generated.items():
        if json.loads((source / name).read_text()) != expected:
            parser.error(f"{name} differs from portable sources; run plugins/build.py --sync")
    variant = ("remote-private" if url.path.rstrip("/").endswith("/mcp/private") else "remote-help") if args.url else "local"
    targets = ("openai", "claude") if args.target == "all" else (args.target,)
    for target in targets:
        output = root / "build/plugins" / target / variant / "myscoutee"
        if output.exists():
            shutil.rmtree(output)
        output.mkdir(parents=True)
        shutil.copytree(source / "skills", output / "skills", symlinks=True)
        shutil.copytree(source / "assets", output / "assets", symlinks=True)
        if target == "openai":
            for name in ("plugin.json", "mcp.json", ".mcp.json"):
                shutil.copyfile(source / name, output / name)
            shutil.copytree(source / ".codex-plugin", output / ".codex-plugin", symlinks=True)
        else:
            claude = {key: manifest[key] for key in
                      ("name", "version", "description", "author", "repository")}
            if variant == "remote-help":
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
            if target == "openai" and variant == "remote-help":
                for name in ("plugin.json", ".codex-plugin/plugin.json"):
                    path = output / name
                    data = json.loads(path.read_text())
                    data["description"] = "Current MyScoutee help in English and Hungarian."
                    interface = data.get("interface") or data["extensions"]["com.openai"]["interface"]
                    interface.update(
                        capabilities=["Read"], shortDescription="Current MyScoutee help.",
                        longDescription="Read the application's active help. This remote connection does not expose personal API operations.",
                        defaultPrompt=["Explain how MyScoutee groups work.", "Hogyan kérhetek közvetlen chatengedélyt?"])
                    extension = data if name.startswith(".codex") else data["extensions"]["com.openai"]
                    extension["review"] = json.loads((root / "plugins/review/remote-help.json").read_text())
                    write_json(path, data)

        archive = root / "build" / f"myscoutee-plugin-{target}-{variant}-{manifest['version']}.zip"
        if any(path.is_symlink() for path in output.rglob("*")):
            parser.error("Symlinks cannot be packaged")
        with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as bundle:
            for path in sorted(output.rglob("*")):
                if path.is_file():
                    bundle.write(path, Path("myscoutee") / path.relative_to(output))
        problems = validate_archive(archive, submission=args.submission)
        if problems:
            archive.unlink()
            parser.exit(1, "\n".join(problems) + "\n")
        print(archive)


if __name__ == "__main__":
    main()
