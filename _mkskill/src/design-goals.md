---
mkskill:
  pos: 45
  in: readme
---

## Design goals

1. **Explicit asset catalog** — every embedded file is declared in XML; nothing is implicitly scanned
2. **Minimal embedded payload** — only declared assets are included in the binary
3. **Build-time integration** — runs via `go generate`, produces deterministic output
4. **No syntax conflict** — percent-tag syntax (`<% %>`) passes Go `{{ }}` templates through untouched
5. **Content-agnostic processing** — files are opaque text; percent tags and front-matter are the only interpreted structures
6. **Layout and content separation** — skins provide reusable layouts; content files declare which skin to apply via front-matter

