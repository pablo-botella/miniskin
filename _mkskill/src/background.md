---
mkskill:
  pos: 160
  in: readme
---

## Background

miniskin originated as a helper for `go generate` in projects that needed fine-grained control over which assets are embedded and how they are registered. It is not a static site generator — it is an asset assembler. It does not replace tools like Hugo or Jekyll; it operates at a different layer, producing Go source files that compile into the binary.

