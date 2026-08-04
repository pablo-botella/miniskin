---
mkskill:
  pos: 10
---

# miniskin (CLI)

[![Go Reference](https://pkg.go.dev/badge/github.com/pablo-botella/miniskin/cmd/miniskin.svg)](https://pkg.go.dev/github.com/pablo-botella/miniskin/cmd/miniskin)

Command-line interface for the [miniskin](https://pkg.go.dev/github.com/pablo-botella/miniskin) build-time template assembler.

miniskin started as a library helper for `//go:generate`. The CLI was added later for interactive operations that don't really fit a `go:generate` step — inspect dependencies, transform mockups into negative templates, combine and split XML catalogs, regenerate AI agent docs — and as a no-code shortcut for the `go:generate` integration itself.

