---
mkskill:
  pos: 40
  in: readme
---

## Why miniskin?

Embedding assets in Go projects typically involves scanning directories with `//go:embed` patterns or scattering embed directives across packages. Registering those assets in an HTTP mux requires manual wiring that drifts as assets are added or removed. Layouts (headers, footers, navigation) get duplicated across templates and diverge over time.

miniskin addresses these problems with an explicit XML catalog that declares exactly which files are embedded, reusable skin (layout) files that enforce structural consistency, and a code generator that produces the embed declarations and asset registration functions automatically.

