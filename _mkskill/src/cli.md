---
mkskill:
  pos: 130
  in: readme
---

## CLI

```
miniskin <command> [flags]

Commands:
  run                    Mockup update + Build + Generate code
  generate               Build embed assets + Generate Go code
  -generate-claude-skill  -generate-agent-docs  -generate-readme
                         Write/install the composed docs (mkskill family)
  mockup update          Export mockup pieces + Refresh imports
  mockup negative        Transform a mockup file into a negative template
  deps                   Show dependency map and processing order
  combine <dir>          Combine subdirectory XMLs into one
  split <file>           Split nested resource-lists into separate XMLs

Flags:
  -content string        path to content directory (default ".")
  -modules string        path to modules directory (default ".")
  -v                     verbose output (dependency analysis, processing order)
  -vv                    debug output (all internal details)
  -silent                suppress all output

Mockup negative flags:
  -src string            source mockup file (required)
  -dst string            destination negative template file (required)

Generate family flags:
  -dst string            destination path   -global   install skill under ~/.claude
  -force                 overwrite existing destination file
```

Examples:

```
miniskin run
miniskin run -v
miniskin generate
miniskin -generate-claude-skill -global -force
miniskin -generate-agent-docs -dst CONVENTIONS.md -force
miniskin mockup update
miniskin mockup negative -src mockup_login.html -dst login_negative.html
miniskin deps
miniskin combine content/app
miniskin split content/app/app.miniskin.xml
```

