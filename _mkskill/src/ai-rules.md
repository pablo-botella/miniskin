---
mkskill:
  pos: 240
  in: ai*
---

## Path resolution

`/` and `\` are treated as equivalent separators in all paths. The rule is uniform:

- **`/` or `\` prefix** → relative to `bucketSrc` (`contentPath + bucket.Src`)
  - `include:/utils/helper.js` → `bucketSrc/utils/helper.js`
  - `mockup-export:/login/page.html` → `bucketSrc/login/page.html`
  - `mockup-import:/shared/header.html` → `bucketSrc/shared/header.html`
- **No prefix** → relative to the current source file's directory
  - `include:helper.js` → same directory as the file containing the include
- **`bucket.Src`** → relative to `contentPath` (both `/app` and `app` resolve the same)
- **`skin-dir`** → relative to `bucketSrc` (fallback: `contentPath` when no bucket context)
- **`dst`** in `<bucket>`: relative to `project-root` (not bucket src)
  - `project-root` is set on `<bucket-list>` and is relative to `contentPath`
  - Example: `project-root=".."` with `dst="/modules/app/gen.go"` → `contentPath/../modules/app/gen.go`
- **`external-item.id`** → resource id in the named origin; the resource's `src` is resolved against its source `base` (local dir path, or remote URL). Local wins over remote per id.
- **`external-item.dstfile`** → relative to the directory of the declaring `*.miniskin.xml` (same as `<item src>`)
- **`miniskin-origin.xml`** (per-dev override) and **`miniskin-resource-hash.xml`** (remote hash cache) → fixed names at `contentPath` root, next to the root `*.miniskin.xml`

## Validation

- During build embed, all output files are validated to exist on disk before code generation
- Missing files produce an error with item name, absolute path, and XML origin:
  ```
  item "app.css" not found at: /abs/path/content/app/app.css
      (declared in /abs/path/content/app/app.miniskin.xml line 7)
  ```

## Key behaviors

- `mockup-export` inside `if:mockup` is silently skipped in normal mode (no error)
- `mockup-import` inside `mockup-export` works (imported content becomes part of export)
- Export block dependencies resolved at block level: if export A imports a path produced by export B (same or different file), B is processed first
- `mockup-export` inside `mockup-import` is ignored (raw text, not parsed)
- In mockup mode: variables, includes, echo, note pass through literally (not resolved)
- Conditionals in mockup mode check existence only (defined = true)
- **Inside `mockup-export` blocks**: all conditionals (`if:`, `else`, `endif`, etc.) pass through literally to the export buffer — nothing is excluded. FSM block stack stays balanced for correct nesting.
- `mockup=1` is auto-injected in mockup mode
- `end` is universal closer for if and mockup-export blocks; mockup-import blocks MUST close with `end-mockup-import` (generic `end` does not truncate the inline content and errors when detectable)
- Specific closers: `end-if` (if only), `end-mockup-export` (export only), `end-mockup-import` (import, mandatory)
- `TransformNegative` replaces export...end blocks with import...end-mockup-import blocks
- Resource lists can be chained (multiple at the same level) and nested (with `src` for relative path resolution)
- Skin directory cascades: `<miniskin>` → `<bucket>` → `<resource-list>` → nested `<resource-list>` (default: `_skin`)
- Mux-include/mux-exclude cascades: `<miniskin>` → `<bucket-list>` → `<bucket>` → `<resource-list>` → nested `<resource-list>` (default: `mux-include="*"`, `mux-exclude=""`)
- Template-function-map cascades: `<bucket>` → `<resource-list>` → `<item>`. Injects `template.FuncMap` into parsed templates via `.Funcs(expr)` before `.Parse()`
- Escape rules cascade through the same hierarchy, including nested resource-lists
- Items not matching `mux-include` or matching `mux-exclude` get `nomux` added automatically
- Explicit `nomux` in item type always takes precedence
- Save-mode cascades: `<mockup-list>` → `<item>` → tag-level mode
- Variable merge order: globals → mockup-list vars → item vars → front-matter vars
- First write to a file in a session always truncates; subsequent writes respect mode
- `refreshImports` is idempotent: single tags promoted to blocks, existing blocks get content replaced
- **`line-mode`** (default: on): when `mockup-import`, `mockup-export`, or `include` tags appear inside a line with surrounding content (e.g. `/* <%%include:file.js%%> */`), the entire line is consumed — content before the tag is truncated, content after is discarded. Disable with `<mockup-list line-mode="off">`
- **JS-comment wrapper** (`/*<% … %>*/` and `/*<%% … %%>*/`): two extra tag syntaxes (5 and 6) recognised by the FSM. The `/*` and `*/` are part of the delimiter and consumed with the tag — they do **not** appear in the output. Apertura and closure are independent: a tag opened with `/*<%` may close with `%>` (the `*/` is not consumed) and vice versa. Useful for embedding tags inside `.js` / `.css` so the file remains valid when read raw (e.g. during mockup development). Apertura matches only `/*<%` — `/*<!` is **not** treated as an HTML-comment wrapper.
- **`<external>` blocks** run as pipeline step 0 — before deps, mockups, and build. Both `Run()` and `BuildEmbed()` resolve externals first so items that reference a copied file see it on disk. `miniskin-origin.xml` is optional; missing file is fine when no `<external>` blocks exist anywhere.
- **External copy** is mtime-aware: dst is rewritten only when it differs from src in size or mtime; on copy, the source mtime is propagated to dst (so a second run is a no-op).
- **External errors are hard**: missing origin, origin without `<local>`, or missing source file all abort the pipeline with absolute paths and the declaring XML.
- **`doc-block` buffers**: `doc-block-begin:NAME` redirects the output buffer to a new in-memory builder, `doc-block-end:NAME` pops the frame and stores the captured content in `ms.docBuffer[NAME]`. The captured region produces no output where the markers stand. `doc-block-toc:NAME` and `doc-block-content:NAME` later read from `ms.docBuffer`. Scope is **bucket-global**: any item in the bucket can read or write any name. A pre-pass over each bucket's items (with `dryRun` and `collectingDocBlocks` set) populates `ms.docBuffer` before the regular pass runs, so capture/emit order across items doesn't matter. `ms.docBuffer` is reset at the start of each bucket. `doc-block-toc/content` referencing an unknown buffer is an error during the regular pass.
