---
mkskill:
  pos: 30
---

## Commands

| Command | Description |
|---|---|
| `run` | Mockup update + build embed assets + generate Go code (full pipeline) |
| `generate` | Build embed assets + generate Go code (no mockup pass) |
| `debug` | Cargo parse of every catalog: `-echo` byte-faithful rewrite, `-rx` claimed-vs-cargo report |
| `-generate-claude-skill` | Write/install the composed `SKILL.md` (`-global` → `~/.claude/skills/miniskin/`) |
| `-generate-agent-docs` | Write the composed `AGENTS.md` (Cursor, Aider, Windsurf, …) |
| `-generate-readme` | Write the composed `README.md` |
| `mockup update` | Export mockup pieces and refresh `mockup-import` blocks |
| `mockup clean` | Empty the inline content of `mockup-import` blocks |
| `mockup negative` | Transform a mockup file into a negative template |
| `deps` | Print the dependency map and processing order |
| `combine <dir>` | Combine subdirectory `*.miniskin.xml` files into one |
| `split <file>` | Split nested resource-lists into separate XML files |

