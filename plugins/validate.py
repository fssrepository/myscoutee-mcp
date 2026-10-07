"""Offline checks on the actual plugin ZIP. Does not certify live URLs or review."""
import ipaddress
import json
import re
import stat
import struct
import zipfile
from pathlib import PurePosixPath
from urllib.parse import urlsplit


def https_url(value):
    if not isinstance(value, str) or len(value) > 1024:
        return False
    try:
        url = urlsplit(value)
        host = url.hostname or ""
        if (url.scheme != "https" or not host or url.username or url.password
                or re.search(r"\s|[<>]", value)
                or any(part in {"example", "localhost", "invalid", "test"} for part in host.split("."))
                or host.endswith((".local", ".internal"))
                or host in {"example.com", "example.org", "example.net"}
                or "your-" in host):
            return False
        try:
            return ipaddress.ip_address(host).is_global
        except ValueError:
            return "." in host
    except ValueError:
        return False


def validate_archive(archive, submission=False):
    errors = []
    def need(condition, message):
        if not condition:
            errors.append(message)
    try:
        with zipfile.ZipFile(archive) as bundle:
            entries = bundle.infolist()
            need(len({x.filename for x in entries}) == len(entries), "ZIP contains duplicate paths")
            files = {}
            roots = set()
            for entry in entries:
                path = PurePosixPath(entry.filename)
                need(not path.is_absolute() and ".." not in path.parts, "ZIP contains an unsafe path")
                need(not stat.S_ISLNK(entry.external_attr >> 16), "ZIP contains a symlink")
                roots.add(path.parts[0])
                if not entry.is_dir():
                    files[str(PurePosixPath(*path.parts[1:]))] = bundle.read(entry)
            need(len(roots) == 1, "ZIP must contain one plugin directory")
    except (OSError, ValueError, zipfile.BadZipFile) as exc:
        return [f"Cannot read ZIP: {exc}"]

    def read(name):
        try:
            return json.loads(files[name])
        except (KeyError, ValueError):
            errors.append(f"Missing/invalid JSON: {name}")
            return {}

    portable = "plugin.json" in files
    manifest = read("plugin.json" if portable else ".claude-plugin/plugin.json")
    name, version = manifest.get("name", ""), manifest.get("version", "")
    need(bool(re.fullmatch(r"[a-z][a-z0-9-]{0,63}", name)) and roots == {name}, "Invalid plugin name/root")
    need(bool(re.fullmatch(r"\d+\.\d+\.\d+(?:-[\w.-]+)?", version)), "Invalid semantic version")
    need(bool(manifest.get("description")), "Missing description")
    need(any(p.startswith("skills/") and p.endswith("/SKILL.md") for p in files), "Missing skill")
    extension = manifest.get("extensions", {}).get("com.openai", {})
    ui = extension.get("interface", {})

    if portable:
        overlay = read(".codex-plugin/plugin.json")
        for key in ("name", "version", "description", "author", "repository"):
            need(overlay.get(key) == manifest.get(key), f"Compatibility manifest mismatch: {key}")
        for key in ("interface", "review", "publication"):
            need(overlay.get(key) == extension.get(key), f"Compatibility manifest mismatch: {key}")
        for key, limit in (("displayName", 30), ("shortDescription", 30), ("longDescription", 4000), ("developerName", 80)):
            value = ui.get(key)
            need(isinstance(value, str) and 0 < len(value) <= limit, f"Invalid {key} (maximum {limit})")
        prompts = ui.get("defaultPrompt", [])
        prompts = [prompts] if isinstance(prompts, str) else prompts
        need(isinstance(prompts, list) and 1 <= len(prompts) <= 3, "Require 1–3 default prompts")
        if isinstance(prompts, list):
            need(all(isinstance(p, str) and p.strip() and len(p) <= 128 and "\n" not in p for p in prompts), "Invalid default prompt")
            need(len({" ".join(str(p).split()) for p in prompts}) == len(prompts), "Duplicate default prompts")
        for key in ("websiteURL", "supportURL", "privacyPolicyURL", "termsOfServiceURL"):
            if submission or key in ui:
                need(https_url(ui.get(key)), f"Missing/invalid public {key}")
        for key, minimum in (("logo", 256), ("composerIcon", 48)):
            ref = ui.get(key, "")
            path = PurePosixPath(ref)
            data = files.get(str(path), b"")
            need(not path.is_absolute() and ".." not in path.parts and bool(data), f"Missing/unsafe {key} asset")
            png = data.startswith(b"\x89PNG\r\n\x1a\n") and len(data) >= 24
            need(png and len(data) <= 5 * 1024 * 1024, f"{key} must be PNG up to 5 MiB")
            if png:
                width, height = struct.unpack(">II", data[16:24])
                need(width == height and minimum <= width <= 4096, f"Invalid {key} dimensions")
        review = extension.get("review", {})
        cases = review.get("test_cases", {})
        for kind, count in (("positive", 5), ("negative", 3)):
            items = cases.get(kind, [])
            need(isinstance(items, list) and len(items) == count, f"Require {count} {kind} review cases")
            for item in items:
                required = ("description", "prompt", "tools_triggered", "expected_behavior") if kind == "positive" else ("description", "prompt")
                need(isinstance(item, dict) and all(isinstance(item.get(k), str) and item[k].strip() for k in required), f"Incomplete {kind} review case")
        if submission:
            need(https_url(review.get("demo_recording_url")), "Missing/invalid demo recording URL (a script is not a recording)")
            publication = extension.get("publication", {})
            need(bool(publication.get("release_notes")), "Missing release notes")
            need("countries" in publication, "Country targeting has not been decided")
            need("commerce" in review, "Commerce declaration has not been confirmed")

    mcp = read("mcp.json" if portable else ".mcp.json")
    servers = mcp.get("mcpServers", {})
    need(len(servers) == 1 and name in servers, "Require one MCP server matching the plugin name")
    if portable:
        compatible = read(".mcp.json").get("mcpServers", {})
        # Legacy clients call the same Streamable HTTP transport "http".
        for server in compatible.values():
            if server.get("type") == "http":
                server["type"] = "streamable-http"
        need(compatible == servers, "Compatibility MCP configuration differs")
    for server in servers.values():
        if "command" in server:
            need(not submission, "Public submission needs a verified remote HTTPS MCP endpoint")
            need(f"myscoutee-mcp:{version}" in server.get("args", []), "Local Docker image version differs from manifest")
        else:
            need(https_url(server.get("url")), "Remote MCP URL is missing, private or a placeholder")
            need(not urlsplit(server.get("url", "")).query and not urlsplit(server.get("url", "")).fragment, "MCP URL must not contain query/fragment")
    # Public upload restrictions apply across shadowed/compatibility manifests too.
    for filename in ("plugin.json", ".codex-plugin/plugin.json", ".claude-plugin/plugin.json"):
        if filename not in files:
            continue
        data = read(filename)
        need(data.get("apps") is None and data.get("extensions", {}).get("com.openai", {}).get("apps") is None, f"App bindings cannot be uploaded: {filename}")
        need(not data.get("hooks"), f"Lifecycle hooks cannot be uploaded: {filename}")
    need(".app.json" not in files and not any(p.startswith("hooks/") for p in files), "App bindings/hooks found in package")
    if submission and not portable:
        errors.append("Claude directory review requires the GitHub bundle/connector handoff; local ZIP validity is not submission approval")
    return errors
