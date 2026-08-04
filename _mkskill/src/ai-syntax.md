---
mkskill:
  pos: 210
  in: ai*
---

## Percent-tag syntaxes (6 forms)

| Syntax | Behavior |
|---|---|
| `<%var%>` | value, escaped per `<escape>` rules |
| `<%%var%%>` | value, never escaped |
| `<!--%var%-->` | same as `<%>`, hidden in browser (HTML comment wrapper) |
| `<!--%%var%%-->` | same as `<%%>`, hidden in browser (HTML comment wrapper) |
| `/*<%var%>*/` | same as `<%>`, hidden as a JS / CSS comment |
| `/*<%%var%%>*/` | same as `<%%>`, hidden as a JS / CSS comment |

Single-percent tags (`<%`, `<!--%`, `/*<%`) apply the default escape configured via `<escape>` in XML. Double-percent tags (`<%%`, `<!--%%`, `/*<%%`) never escape. Default escape is none unless configured.

The JS-comment wrapper (`/*<%`, `%>*/`) is recognised at apertura and closure independently — they need not be balanced. Use it inside `.js` / `.css` source so the embedded tag is also a valid block comment when the file is loaded raw (e.g. during mockup development). Only `/*<%` matches; `/*<!` is **not** treated as a wrapper for HTML comments.

## Directives

| Directive | Description |
|---|---|
| `if:var` / `if-not:var` | Conditional (check if var is defined and non-empty) |
| `elseif:var` / `elseif-not:var` | Chained conditional |
| `else` | Fallback branch |
| `endif` / `end-if` / `end` | Close conditional block |
| `mockup-export:path [append\|overwrite] [ltrim\|rtrim\|trim]` | Extract content to file (mockup mode only). `ltrim`: dedent, `rtrim`: strip trailing whitespace, `trim`: both |
| `end-mockup-export` / `end` | Close mockup-export block |
| `mockup-import:path [indent:N\|Ntab]` | Insert file contents (mockup mode only). `indent:4`: 4 spaces per line, `indent:2tab`: 2 tabs per line. Value accepts quotes: `indent:"8tab"`, `indent="8tab"` |
| `end-mockup-import` | Close mockup-import block (mandatory — generic `end` is not valid here) |
| `include:path` | Include file (double tags only, resolved recursively). Absolute (`/path`) relative to bucket src; relative to current file otherwise. In line-mode: entire line containing the tag is removed (including surrounding comments like `/* */`) |
| `include-notes:path` | Include file (double tags only). Returns only the bodies of `note:` tags found in the file, dedented and joined by a blank line. Used to assemble per-component documentation into a single Markdown file. Same path resolution as `include:` |
| `doc-block-begin:NAME` / `doc-block-end:NAME` | Capture content between the markers into a labeled, in-memory buffer (`ms.docBuffer[NAME]`). The captured region is **not** emitted in place — only stored. After capture, the buffer can be replayed elsewhere via `doc-block-content` or summarised via `doc-block-toc`. Buffer scope is bucket-global: a buffer captured in one resource-list item is visible from other items in the same bucket |
| `doc-block-content:NAME` | Emit the literal contents of `doc_buffer[NAME]`. Errors if the buffer was never defined in this bucket |
| `doc-block-toc:NAME` | Emit a nested unordered list of the H1 (`#`) and H2 (`##`) headers found in `doc_buffer[NAME]`, with GitHub-compatible anchors (lowercase, alphanumerics, dashes; duplicates suffixed `-1`, `-2`, …). Code-fenced regions are skipped. Errors if the buffer was never defined |
| `note:text` | Discarded silently |
| `echo:text` | Emit text (uses default escape) |

## Escape types

Explicit escape prefix overrides default:
`<%url:var%>`, `<%%js:var%%>`, `<!--%sql:var%-->`, etc.

Prefixes: `html`, `xml`, `url`, `js`, `css`, `json`, `sql`, `sqlt` (sql + LIKE `_%` escaping)

Echo with escape prefix: `<%js:echo:it's "ok"%>` → `it\'s \"ok\"`. Plain `<%echo:text%>` uses configured default escape.

Default escape is none. Configurable per file extension: `<escape ext="*.html" as="html" />` in any XML block. Use `<escape ext="*" as="html" />` at `<miniskin>` level to escape everything as HTML.
Item-level override: `<item escape="sql" .../>`. Cascades like skin-dir and mux-include.
