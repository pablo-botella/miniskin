# miniskin

## miniskin

Build-time template assembler for Go projects. Processes `*.miniskin.xml` catalogs,
resolves percent tags, applies skins (layouts), and generates Go source code with `//go:embed` directives. Supports Mockup-Driven Development (MDD).

## Pipeline

`Run()` executes the following steps in order:

0. **Resolve externals** — materialise files declared in `<external>` blocks from their origins: copy from a `<local>` source or download+verify from a `<remote>` source (local wins). Committed origin defaults + per-developer `miniskin-origin.xml` override
1. **Analyze dependencies** — build export/import graph, error on circular references
2. **Export mockups** — `mockup-export` directives write content to files on disk
3. **Update imports** — refresh inline content in `mockup-import` blocks
4. **Build embed** — process resource items, resolve variables, apply skins

The **build embed** step itself runs each bucket twice: a dry pre-pass to populate any `doc-block-begin/end` buffers (no output is written, `doc-block-toc/content` emit nothing), then the regular pass with the buffers fully populated. This makes doc-block buffers visible bucket-wide regardless of item order.

`BuildEmbed()` invoked on its own also runs externals first, so item sources that depend on copied files exist before assembly.

Code generation (`GenerateAll`) runs after `Run()` to produce `//go:embed` Go files.

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
| `mockup-import:path [indent:N\|Ntab]` | Insert file contents (mockup mode only). `indent:4`: 4 spaces per line, `indent:2tab`: 2 tabs per line. Value accepts quotes: `indent:"8tab"`, `indent="8tab"`. A leading UTF-8 BOM in the imported file is stripped. The inline content may carry its own percent tags: the block is closed by its matching `end-mockup-import` |
| `end-mockup-import` | Close mockup-import block (mandatory — generic `end` is not valid here) |
| `include:path [minify:type[:0\|1\|2]]` | Include file (double tags only, resolved recursively). Absolute (`/path`) relative to bucket src; relative to current file otherwise. A leading UTF-8 BOM is stripped. Quote paths with spaces: `include:"/my css/a.css" minify:css`. `minify:type:N` runs the resolved result through the minifier with the same levels as `@minify` (`0` = none, `1` = safe, `2` = aggressive; `minify:type` alone = `1`); types: `css`, `js`, `html`, `json`, `svg`, `xml`. No guarantees: the included file should contain only valid content of that type — anything else (e.g. runtime `{{...}}` in CSS/JS, content inside comments) may be mangled or removed, and a minifier error stops the build. In line-mode: entire line containing the tag is removed (including surrounding comments like `/* */`) |
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

## Front-matter

YAML-like block delimited by `---` at the top of source files:

```
---
skin: default
title: Sign In
@minify:1
@eol:lf
---
<div>content</div>
```

`skin` triggers layout application. Other keys become template variables.
Lines starting with `@` are directives (processed by miniskin, not passed as variables).

### Directives

| Directive | Values | Description |
|---|---|---|
| `@minify` | `0` (default), `1`, `2` | Minify output via tdewolff/minify, selected by output extension (html/css/js/json/svg/xml). `0` = none, `1` = safe (conservative options), `2` = aggressive (maximum minification). Other types pass through unchanged; an unknown level is an error. |
| `@eol` | `lf`, `crlf`, `cr` | Convert line endings (default: no conversion) |

## XML structure

**Root** (`contentPath/*.miniskin.xml`):

```xml
<miniskin skin-dir="_skin" log="miniskin.log">
  <globals>
    <var name="appName" value="MyApp" />
  </globals>
  <escape ext="*.html,*.html.tmpl" as="html" />
  <escape ext="*.js,*.js.tmpl" as="js" />
  <bucket-list filename="generated_embed.go" module="content" import="pkg/content" project-root="..">
    <bucket src="app" dst="/modules/app/generated_assets.go"
            module-name="app" template="miniskin::mux" />
  </bucket-list>
</miniskin>
```

### `<bucket-list>` attributes

| Attribute | Description |
|---|---|
| `filename`     | embed file path written by `Codegen.GenerateEmbed` (e.g. `generated_embed.go`) |
| `module`       | Go package name for the generated embed file |
| `import`       | Go import path for the generated embed file |
| `template`     | custom embed template (`miniskin::default`, `miniskin::mux`, `miniskin::muxblob`, or a file) |
| `project-root` | path resolved from `contentPath` for resolving bucket `dst`s |
| `mux-include` / `mux-exclude` | cascading mux glob patterns |
| `omit`         | comma/space-separated codegen outputs to skip — `embed` (skip the embed file) and/or `module` (skip per-bucket module files). When both are listed, `filename` and `module` may be omitted entirely. Useful when miniskin is used to assemble assets for non-Go projects. |
| `blob-out`     | output directory for `.blob` sidecar files (relative to `project-root`, default `blob`). See [Blobs](#blobs). |

**Subdirectory** (`subdir/*.miniskin.xml`):

```xml
<miniskin>
  <resource-list urlbase="/assets" skin-dir="skins">
    <item type="static" file="app.css" />
    <item type="html-template,parse" src="page_src.html" file="page.html" key="/page" />
  </resource-list>
  <mockup-list save-mode="append" line-mode="off">
    <var name="policybanner" value="1" />
    <item src="mockup_login.html">
      <var name="title" value="Login" />
    </item>
  </mockup-list>
  <external>
    <external-item origin="closure-ui" id="closure-ui.js" dstfile="./src/app_source.js" />
  </external>
</miniskin>
```

## External / Origin

`<external>` **materialises files** into the project at build time (a file-drop to
`dstfile` — *not* an embedded item). To embed/serve one, list a normal `<item>`
pointing at the `dstfile`; externals run at step 0, so the file already exists. A
source can be a **local** directory (a sibling repo build) or a **remote** URL (a
pinned release), resolved through named origins.

### Origins: `<local>` and/or `<remote>`, resources by `id`

An `<origin>` has a `<local>` and/or `<remote>` side, each with a `base` and a list
of `<resource>`. A resource's `id` defaults to its `src`.

- The **committed default** (usually the `<remote>`) lives in a normal
  `*.miniskin.xml` (e.g. the root `content.miniskin.xml`).
- The **per-developer override** (usually a `<local>`) lives in
  `miniskin-origin.xml` (content root, **not** committed).

Origins merge by **`name`**, resources by **`id`**; **local always wins** per id.

```xml
<!-- committed content.miniskin.xml: the remote default -->
<origin name="closure-ui">
  <remote base="https://github.com/pablo-botella/closure-ui/releases/download/v0.0.4/">
    <resource id="closure-ui.js" src="closure-ui-min-v0.0.4.js"
              sha256="c00169…b779f" />
  </remote>
</origin>

<!-- gitignored miniskin-origin.xml: the local override (wins) -->
<origin name="closure-ui">
  <local base="C:\HD\F\_sams\closure-ui\release\">
    <resource id="closure-ui.js" src="closure-ui.js" ignorehash="always" />
  </local>
</origin>
```

`<resource>` attributes: `id` (default = `src`), `src` (file under the source's
`base`), `sha256` (optional; inline on a remote = the **contract**), `ignorehash`
(truthy → skip verification, e.g. local dev builds with different bytes).

### `<external-item>` (references by `id`)

Inside `<external>` in any subdirectory `*.miniskin.xml`, resolved at step 0.

| Attribute | Description |
|---|---|
| `origin` | origin `name` |
| `id`     | resource id within the origin (the real filename differs per source, so you reference by id, not path) |
| `dstfile`| destination relative to the directory of the declaring XML |

Resolution:

- **local resource exists** → copied **every build** (dev build changes; no hash),
  mtime+size-aware.
- **local declared but file missing** → error.
- **no local** → **remote**: with `sha256` (contract) verify on download (mismatch →
  error), no-op if `dstfile` already matches; without `sha256`, trust the versioned
  URL, download, and cache the hash (TOFU) — re-fetch only if `dstfile` or the cache
  entry is gone; a changed hash on re-fetch → error.
- a failed fetch (network / non-200) → error.

### Hash cache: `miniskin-resource-hash.xml`

For contract-less remotes only, computed hashes are cached in
`miniskin-resource-hash.xml` (content root, sibling of `miniskin-origin.xml`).
Machine-managed and gitignoreable. **Cleanup = delete it by hand** → next build
re-fetches every contract-less resource; contract resources stay pinned by their
inline `sha256`.

Resource lists can be **chained** (multiple at the same level) and **nested** (with `src` for relative path resolution):

```xml
<miniskin>
  <resource-list urlbase="/assets">
    <item type="static" file="app.css" />
    <resource-list src="login" urlbase="/login">
      <item src="signin_src.html" file="signin.html" />
    </resource-list>
  </resource-list>
</miniskin>
```

Nested resource lists inherit `skin-dir`, `mux-include`, `mux-exclude`, `template-function-map`, and `<escape>` rules from their parent.

## Item attributes

- `file` — output filename (what gets embedded). Not needed for `block` items.
- `src` — source filename (if present, processed through template engine)
- `type` — comma-separated flags: `static`, `html-template`, `response`, `nomux`, `parse`, `block`
- `key` — logical key for asset lookup
- `url` / `alt-url-abs` — URL routing attributes
- `escape` — override default escape type for this item (`html`, `js`, `url`, `sql`, etc.)
- `template-function-map` — Go expression returning `template.FuncMap`; injected via `.Funcs(expr)` before `.Parse()` for `parse` items (cascades from `<bucket>` → `<resource-list>` → `<item>`)

### `type="block"` items (process, no output)

A `block` item is processed for its **side effects only** — populating `doc-block`
buffers — and writes no output file, so it needs no `file`:

```xml
<item type="block" src="./_doc_block_b1.list" />
```

The item's `src` is resolved in the doc-block pre-pass (so any `doc-block-begin/end`
and `include-notes:` inside it run and fill the buffers), then the item is dropped
before writing, existence-validation, and embedding. A separate item then emits the
buffer it built (`doc-block-toc` / `doc-block-content`). Buffers are bucket-global,
so the consumer can live in any item of the same bucket. This is the clean way to
assemble a doc buffer from many partials without generating a throwaway file — an
ordinary item always writes its `file`.

## Response items (`type="response"`)

A `response` item serves a **canned HTTP response** instead of file content: a
redirect (3xx + `Location`), a bare status (404, 410, …), or an error page with a
static body. The item points to a raw `.http` file — a status line, optional
headers, a blank line, and an optional body:

```
301 Moved Permanently
Location: https://www.example.com/products

```

```
404 Not Found
Content-Type: text/html; charset=utf-8
Cache-Control: no-store

<h1>Not found</h1>
```

Declared like any other item; the route comes from `key` (use it — without `key`
the route would be the `.http` filename):

```xml
<item type="response" file="old-page.http" key="/old-page/" />
```

Mechanics (mux template only):

- The `.http` is embedded exactly like a `static` asset — same `//go:embed`, same
  on-disk validation. No separate build step, no Go core code.
- Generated code calls `serveResponse(mux, route, embeddedBytes)`. The response is
  parsed **once, at registration**: first line → status code (the reason phrase is
  ignored, net/http derives it from the code); header lines until the blank line →
  response headers (any header, repeats preserved via `Add`); the rest → body.
- Bytes are fixed at build time (like `static`). For per-environment values, use a
  build-time percent tag with `src` (e.g. `Location: https://<%%domain%%>/x`).
  Anything dynamic/per-request belongs in your own `mux.HandleFunc`, not here.
- Headers you declare are sent verbatim; net/http still adds framing headers
  (`Date`, `Content-Length`) and sniffs `Content-Type` only when you omit it.

## Blobs

A **blob** packs many items into a single external `.blob` sidecar file, read at
runtime, instead of embedding each as a Go `var` (`go:embed`). Use it for large
asset volumes you do not want baked into the binary. Blobs need the
`miniskin::muxblob` preset and the companion module
[`github.com/pablo-botella/mskblob`](https://pkg.go.dev/github.com/pablo-botella/mskblob).

An item is packed into a blob (instead of embedded) when its `<resource-list>`
carries `blob-name`. These `<resource-list>` attributes (all cascade to nested
resource-lists):

| Attribute | Description |
|---|---|
| `blob-name` | file name of the `.blob` this item is packed into (e.g. `prod-img.blob`). Several resource-lists with the same `blob-name` pack together; a different name is a different file. |
| `blob-attach` | comma/space-separated list of where to wire the blob into the generated package: `mux`, `assets`, `templates`. **Multi-valued** — one blob can be several at once; each entry lands wherever its `type`/`key` qualifies. Empty = blob stored but not auto-wired (wire it yourself via `Blob(id)`). An unknown token is a build error. |
| `preserve-blob-if-id` | pin the blob's guid: if the on-disk `.blob` already carries exactly this guid, it is trusted as-is (cache hit — only the header is read, no repack). A mismatch or missing blob is an error. |

`.blob` output directory comes from `blob-out` on `<bucket-list>` (default `blob`).

### The three attach modes

```xml
<miniskin>

  <!-- mux: entries served over HTTP, streamed lazily from the .blob.
       Served at Base + URL, where Base = urlbase + "/". -->
  <resource-list urlbase="/img" blob-name="prod-img.blob" blob-attach="mux">
    <item type="static" file="hero.jpg" />
    <item type="static" file="logo.png" />
  </resource-list>

  <!-- assets: entries NOT served; reachable via GetAsset(key). Needs key=. -->
  <resource-list blob-name="data.blob" blob-attach="assets">
    <item type="static" file="prices.json" key="/data/prices" />
  </resource-list>

  <!-- templates: entries parsed to *template.Template, via GetParsedTemplate(key).
       Needs key= and an html-template/parse type. -->
  <resource-list blob-name="mail.blob" blob-attach="templates">
    <item type="html-template,parse" file="welcome.html" key="/mail/welcome" />
  </resource-list>

  <!-- multi-attach: one blob wired into several structures at once;
       each entry lands by its restype/key. -->
  <resource-list urlbase="/all" blob-name="all.blob" blob-attach="mux,assets,templates">
    <item type="static" file="hero.jpg" />
    <item type="html-template,parse" file="page.html" key="/all/page" />
  </resource-list>

</miniskin>
```

Sharp edges:

- For `mux`, each item's route must fall **under** the base (`urlbase` + `/`); a
  `key` that does not start with `urlbase` is a `packBlobs` error.
- For `assets`/`templates`, entries are found by `key` — an item without `key` is
  skipped by those branches.

### Manual init (`miniskin::muxblob`)

`::mux` is embedded-only, so it can init automatically. A blob lives **outside**
the binary, and miniskin cannot know its deploy path — so the application owns
loading. The generated package gives you the file name and guid as consts; you
load and register:

```go
mux := http.NewServeMux()

// 1) embedded routes (static / response / html-template handlers) — if present
app.RegisterEmbededRoutes(mux, tmplHandlers)

// 2) embedded `parse` templates — only generated if there are parse items
app.InitBucket(func(err error) bool { log.Print(err); return true })

// 3) blobs — the app builds the deploy path from the generated File const
b, err := mskblob.Load(filepath.Join(blobDir, app.ProdImgBlobFile), app.ProdImgBlobID)
if err != nil { log.Fatal(err) } // Load verifies the guid matches
app.RegisterMskBlob(mux, b, func(err error) bool { log.Print(err); return true })
```

- `mskblob.Load(path, id)` validates the guid on open — a stale/mismatched `.blob`
  fails here, not at serve time.
- `RegisterMskBlob` switches on the blob's guid to wire its entries (per the attach
  modes). An unknown guid goes to `onError` (`"unregistered blob id …"`).
- `OnError` returns `true` to continue, `false` to stop; `nil` = skip silently.
- The `mux` parameter is present only when some blob's attach includes `mux`.

## Built-in templates (presets)

- `miniskin::default` — generic `Asset` type with `Get()`, `RegisterRoutes()`, `GetParsedTemplate()`
- `miniskin::mux` — **legacy preset, automatic init, NO blob support.** `RegisterRoutes(mux *http.ServeMux, tmplHandlers)` with exact-path matching; `serveResponse` for `response` items. Everything is embedded in the binary, so it needs no external load step. If a bucket using `::mux` has `blob-name` items, the `.blob` is still packed but `::mux` ignores it.
- `miniskin::muxblob` — **everything `::mux` does, plus external blobs; requires manual init** (it must load the `.blob` files at runtime). The favoured preset when you use blobs. See [Blobs](#blobs) for the init sequence.

Custom templates via file path: `template="my_template.tmpl"`

### Generated sections are conditional (modes)

Both mux presets emit each capability only when the bucket needs it — unused sections (and their imports) are omitted entirely. A bucket with no `response` items gets no `serveResponse`/`parseResponse`; one with no blobs gets nothing from `mskblob`. So your manual init only calls what was generated.

| Mode | Generated when the bucket has… | Emits | Runtime init |
|---|---|---|---|
| Embedded routes | a `static`/`html-template`/`response` item (non-`nomux`) | `RegisterEmbededRoutes`, `serveStatic` | `RegisterEmbededRoutes(mux, tmplHandlers)` |
| Served templates | an `html-template` (non-`nomux`) | `Templates` map | consumed by your handler layer |
| Assets (`nomux`) | a `nomux` item **or** a blob whose attach includes `assets` | `Asset`, `assets`, `GetAsset`, `AllAssets` | — (populated at build / in `RegisterMskBlob`) |
| Parse templates | a `parse` item | `InitBucket`, `parsedTemplates`, `GetParsedTemplate` | `InitBucket(onError)` |
| Response | a `response` item (non-`nomux`) | `serveResponse`, `parseResponse` | via `RegisterEmbededRoutes` |
| Blobs (base) | any blob | `OnError`, `blobs`, `Blob(id)`, `RegisterMskBlob`, `<Base>File`/`<Base>ID` consts | `mskblob.Load` + `RegisterMskBlob` |
| Blob→mux | a blob whose attach **includes** `mux` | `serveStaticBlob` + `mux` param in `RegisterMskBlob` | inside `RegisterMskBlob` |
| Blob→assets | a blob whose attach **includes** `assets` | the `assets` branch of `RegisterMskBlob` | inside `RegisterMskBlob` |
| Blob→templates | a blob whose attach **includes** `templates` | the `templates` branch of `RegisterMskBlob` | inside `RegisterMskBlob` |

## Go API

```go
// One-liner: full pipeline + code generation
miniskin.MiniskinRun(contentPath, modulesPath)
miniskin.MiniskinRun(contentPath, modulesPath, miniskin.VerbosityVerbose)

// Step by step
ms := miniskin.MiniskinNew(contentPath, modulesPath)
ms.SetVerbosity(miniskin.VerbosityVerbose)

// Dependency analysis
dm, _ := ms.AnalyzeDeps()
order, _ := dm.ProcessingOrder()
if dm.HasCycles() { /* error */ }

// Full pipeline
result, _ := ms.Run()

// Code generation
cg := miniskin.CodegenNew(contentPath, modulesPath)
cg.GenerateAll(result)

// Mockup update only (export + refresh imports)
miniskin.MiniskinMockupUpdate(contentPath, modulesPath)

// Mockup clean (empty inlined content of mockup-import blocks)
miniskin.MiniskinMockupClean(contentPath, modulesPath)

// Generate only (build + codegen, no mockups)
miniskin.MiniskinGenerate(contentPath, modulesPath)

// Single-file negative transform
neg := miniskin.TransformNegative(content)

// Combine subdirectory XMLs into one
miniskin.CombineDir("/path/to/app")

// Split nested resource-lists back into separate XMLs
miniskin.SplitXML("/path/to/app/app.miniskin.xml")
```

The AI docs are no longer a library API: the binary carries them composed
(by mkskill) and hands them out itself — see the generate commands below.

## Doc-block buffers

Capture a region into a named buffer, then emit a generated table of contents and the captured content from elsewhere:

```
<%% doc-block-begin: components %%>
<%% include-notes:btn_grid.js %%>
<%% include-notes:clock_display.js %%>
<%% include-notes:credential_password.js %%>
<%% doc-block-end: components %%>

# Contents

<%% doc-block-toc: components %%>

---

<%% doc-block-content: components %%>
```

`doc-block-toc` walks the captured markdown for `#` and `##` headers and emits a nested list with GitHub-compatible anchors (duplicates get `-1`, `-2`, …). `doc-block-content` re-emits the captured text verbatim. Buffers are scoped to the bucket, so capture in one resource-list item and emit in another works regardless of item order — a pre-pass populates `ms.docBuffer` before the regular pass runs.

## Install

```
go install github.com/pablo-botella/miniskin/cmd/miniskin@latest    # CLI binary
go get github.com/pablo-botella/miniskin                            # library (go:generate / programmatic)
```

## CLI

```
miniskin run                                       # full pipeline (contentPath = ".")
miniskin run -content content                      # full pipeline with explicit contentPath
miniskin run -v                                    # verbose
miniskin generate                                  # build + codegen only
miniskin -generate-claude-skill -global            # install the Claude Code skill (~/.claude/skills)
miniskin -generate-agent-docs                      # write AGENTS.md (Cursor, Aider, Windsurf, etc.)
miniskin -generate-readme                          # write README.md
miniskin mockup update                             # export + refresh imports
miniskin mockup clean                              # empty inlined content of import blocks
miniskin mockup negative -src m.html -dst n.html   # single file negative
miniskin deps                                      # show dependency map
miniskin combine content/app                       # combine XMLs into one
miniskin split content/app/app.miniskin.xml         # split back into subdirectories
miniskin blob-header blob/prod-img.blob            # inspect a .blob header (magic, version, guid, entries, CRC)
```

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
