---
mkskill:
  pos: 220
  in: ai*
---

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
