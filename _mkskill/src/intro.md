---
mkskill:
  pos: 10
  in: readme
---

# miniskin

[![Go Reference](https://pkg.go.dev/badge/github.com/pablo-botella/miniskin.svg)](https://pkg.go.dev/github.com/pablo-botella/miniskin)

- Miniskin is a build-time template assembler for Go projects.
- Supports Mockup-Driven Development (MDD): mockup HTML files can serve directly as source files — they render standalone in the browser and are processed at build time, eliminating the need to maintain separate mockup and source files.
- It processes content files using an explicit asset catalog defined in `*.miniskin.xml` files and generates Go source code with `//go:embed` directives.
- It is designed to run during development as part of `go generate`, not at runtime.
- The tool is content-agnostic: files are treated as opaque text except for optional front-matter and percent-tag directives.
- The parser is implemented as a finite-state machine, ensuring deterministic single-pass processing.
- Percent-tag syntax (`<% ... %>`) coexists with Go template syntax (`{{ ... }}`) without conflicts.


