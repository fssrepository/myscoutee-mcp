# Agent instructions

Before project work, load the `myscoutee-mcp` instructions from the shared
`myscoutee-skills` checkout and apply the documents they require. It lives beside
the standalone MCP repository or beside the parent application when MCP is a
submodule. Resolve project paths from the MCP repository root.

If the checkout is missing, locate it and clone it from this repository root:

```sh
project_root="$(git rev-parse --show-toplevel)"
parent_project="$(git rev-parse --show-superproject-working-tree)"
skills_parent="$(dirname "${parent_project:-$project_root}")"
skills_root="${skills_parent}/myscoutee-skills"
if [ ! -e "$skills_root" ]; then
  git clone -- https://github.com/fssrepository/myscoutee-skills.git "$skills_root"
fi
cat "$skills_root/instructions/projects/myscoutee-mcp.md"
```

Use an existing checkout without automatically pulling, resetting or replacing
it. If the required file is missing or cloning fails, report the blocker; do not
overwrite existing work. Current user instructions take precedence.
