---
mkskill:
  pos: 40
---

## Flags

Common (any command):

| Flag | Default | Description |
|---|---|---|
| `-content` | `.` | Path to the content directory |
| `-modules` | `.` | Path to the modules directory |
| `-v` | | Verbose output (dependency analysis, processing order) |
| `-vv` | | Debug output (all internal details) |
| `-silent` | | Suppress all output |

`mockup negative`:

| Flag | Required | Description |
|---|---|---|
| `-src` | yes | Source mockup file |
| `-dst` | yes | Destination negative template file |

`debug`:

| Flag | Default | Description |
|---|---|---|
| `-echo` | off | Byte-faithful rewrite of every catalog (file, or `-` for stdout) |
| `-rx` | off | Claimed-vs-cargo report (file, or `-` for stdout; the default output when neither is chosen) |

`-generate-claude-skill`:

| Flag | Default | Description |
|---|---|---|
| `-dst` | `.claude/skills/miniskin/SKILL.md` | Destination path |
| `-global` | | Install into `~/.claude/skills/miniskin/` instead (mutually exclusive with `-dst`) |
| `-force` | | Overwrite an existing destination file |

`-generate-agent-docs` / `-generate-readme`:

| Flag | Default | Description |
|---|---|---|
| `-dst` | `AGENTS.md` / `README.md` | Destination path |
| `-force` | | Overwrite an existing destination file |

