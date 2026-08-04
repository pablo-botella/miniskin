---
mkskill:
  pos: 150
  in: readme
---

## AI assistance

The `_mkskill/src/` directory contains miniskin's documentation in modular
sections — the canonical source. mkskill composes every view from them
(this README, `AGENTS.md` and the Claude skill), and the binary carries the
composed docs embedded, so it hands them out by itself.

### Claude Code

```
miniskin -generate-claude-skill -global
```

Installs `~/.claude/skills/miniskin/SKILL.md` (available from every
project). Without `-global` it writes the project-local
`.claude/skills/miniskin/SKILL.md`; `-dst` picks any other path, `-force`
overwrites.

### Other agents (Cursor, Aider, Windsurf, AGENTS.md, …)

```
miniskin -generate-agent-docs
```

Writes `AGENTS.md` in the current directory — the same content without any
tool-specific frontmatter. Point it elsewhere with `-dst`:

```
miniskin -generate-agent-docs -dst .cursor/rules/miniskin.mdc -force
miniskin -generate-agent-docs -dst CONVENTIONS.md -force
```

Suitable for any tool that accepts plain Markdown context.
