# miniskin

[![Go Reference](https://pkg.go.dev/badge/github.com/pablo-botella/miniskin.svg)](https://pkg.go.dev/github.com/pablo-botella/miniskin)

- Miniskin is a build-time template assembler for Go projects.
- Supports Mockup-Driven Development (MDD): mockup HTML files can serve directly as source files — they render standalone in the browser and are processed at build time, eliminating the need to maintain separate mockup and source files.
- It processes content files using an explicit asset catalog defined in `*.miniskin.xml` files and generates Go source code with `//go:embed` directives.
- It is designed to run during development as part of `go generate`, not at runtime.
- The tool is content-agnostic: files are treated as opaque text except for optional front-matter and percent-tag directives.
- The parser is implemented as a finite-state machine, ensuring deterministic single-pass processing.
- Percent-tag syntax (`<% ... %>`) coexists with Go template syntax (`{{ ... }}`) without conflicts.

## Security notice

miniskin performs no input sanitization, no escaping of user-supplied data, and no protection against code injection. It is a build-time tool. Do not expose it to untrusted input.

## Install

CLI (recommended — primary entry point):

```
go install github.com/pablo-botella/miniskin/cmd/miniskin@latest
```

Library (for use with `go:generate` or programmatic invocation):

```
go get github.com/pablo-botella/miniskin
```

## Why miniskin?

Embedding assets in Go projects typically involves scanning directories with `//go:embed` patterns or scattering embed directives across packages. Registering those assets in an HTTP mux requires manual wiring that drifts as assets are added or removed. Layouts (headers, footers, navigation) get duplicated across templates and diverge over time.

miniskin addresses these problems with an explicit XML catalog that declares exactly which files are embedded, reusable skin (layout) files that enforce structural consistency, and a code generator that produces the embed declarations and asset registration functions automatically.

## Design goals

1. **Explicit asset catalog** — every embedded file is declared in XML; nothing is implicitly scanned
2. **Minimal embedded payload** — only declared assets are included in the binary
3. **Build-time integration** — runs via `go generate`, produces deterministic output
4. **No syntax conflict** — percent-tag syntax (`<% %>`) passes Go `{{ }}` templates through untouched
5. **Content-agnostic processing** — files are opaque text; percent tags and front-matter are the only interpreted structures
6. **Layout and content separation** — skins provide reusable layouts; content files declare which skin to apply via front-matter

## Concepts

### Percent tags

Six equivalent syntaxes, resolved at generation time:

| Syntax | Behavior |
|---|---|
| `<%var%>` | value, escaped per `<escape>` rules |
| `<%%var%%>` | value, never escaped |
| `<!--%var%-->` | same as `<%>`, hidden as an HTML comment |
| `<!--%%var%%-->` | same as `<%%>`, hidden as an HTML comment |
| `/*<%var%>*/` | same as `<%>`, hidden as a JS / CSS comment |
| `/*<%%var%%>*/` | same as `<%%>`, hidden as a JS / CSS comment |

Double percent tags also support includes: `<%%include:/path/to/file%%>`

The JS-comment wrapper (`/*<%`, `%>*/`) keeps tags valid inside `.js` /
`.css` files so they read as block comments when loaded raw (useful
during mockup development). Apertura and closure are independent — a
tag opened with `/*<%` may close with `%>` (the `*/` is not consumed)
and vice versa.

### Escape types

All tags default to no escaping. Use `<escape>` in XML to configure default escaping per file extension, or use an explicit escape prefix in any tag syntax:

```
<%url:var%>          URL-encoded value
<%%js:var%%>         JS-escaped value
<!--%sql:var%-->     SQL-escaped (browser-invisible)
<!--%%json:var%%-->  JSON-escaped (browser-invisible)
```

| Prefix | Description | Example input | Example output |
|---|---|---|---|
| `html` | HTML entities | `<b>"hi"</b>` | `&lt;b&gt;&#34;hi&#34;&lt;/b&gt;` |
| `xml` | XML entities (`&apos;` for `'`) | `it's <ok>` | `it&apos;s &lt;ok&gt;` |
| `url` | URL encoding | `hello world` | `hello+world` |
| `js` | JavaScript string escaping | `say "hi"\n` | `say \"hi\"\\n` |
| `css` | CSS hex escaping | `url("x")` | `url\28 \22 x\22 \29 ` |
| `json` | JSON string escaping | `line1\nline2` | `line1\\nline2` |
| `sql` | SQL single-quote doubling | `O'Brien` | `O''Brien` |
| `sqlt` | SQL LIKE (sql + `_%` escaping) | `100% O'B_x` | `100\% O''B\_x` |

The `escape:echo:text` form applies an explicit escape to literal text: `<%js:echo:it's "ok"%>` outputs `it\'s \"ok\"`.

### Default escape rules

The `<escape>` element configures the default escape type based on file extension. It can appear in any XML block (`<miniskin>`, `<bucket-list>`, `<bucket>`, `<resource-list>`) and cascades to children:

```xml
<miniskin>
  <escape ext="*.html,*.html.tmpl" as="html" />
  <escape ext="*.js,*.js.tmpl" as="js" />
  <escape ext="*.css" as="css" />
  <escape ext="*.sql" as="sql" />
  <bucket-list filename="embed.go" module="content">
    <bucket src="app" dst="/gen.go" module-name="app">
      <escape ext="*.json" as="json" />
    </bucket>
  </bucket-list>
</miniskin>
```

When processing a source file, the default escape is determined by matching the file against `<escape>` rules. If no rule matches, no escaping is applied.

Individual items can override with `escape="type"`:

```xml
<item type="static" src="data_src.txt" file="data.txt" escape="sql" />
```

Position of `<escape>` elements within a block is irrelevant. Child rules override parent rules for the same extension pattern.

### Directives

| Directive | Description |
|---|---|
| `if:var` | Include content if var is defined and non-empty |
| `if-not:var` | Include content if var is undefined or empty |
| `elseif:var` | Checked only if all previous branches were false |
| `elseif-not:var` | Negated elseif |
| `else` | Fallback branch |
| `endif` | Close conditional block |
| `end` | Universal closer (works for if and mockup-export) |
| `end-if` | Close if block (specific alias) |
| `end-mockup-export` | Close mockup-export block (specific alias) |
| `end-mockup-import` | Close mockup-import block (mandatory — generic `end` is not valid here) |
| `note:text` | Discarded silently (comment) |
| `echo:text` | Emit text (uses default escape) |
| `include:path [minify:type[:0\|1\|2]]` | Include file contents (double tags only, resolved recursively), optionally minified |
| `include-notes:path` | Include only the bodies of `note:` tags from the file (double tags only). Used to assemble per-component documentation into a single Markdown |
| `doc-block-begin:NAME` / `doc-block-end:NAME` | Capture content between the markers into the named buffer `ms.docBuffer[NAME]`; the captured region is not emitted in place |
| `doc-block-content:NAME` | Emit the captured contents of the named buffer |
| `doc-block-toc:NAME` | Emit a nested unordered list of the H1/H2 headers in the named buffer, with GitHub-compatible anchors |
| `mockup-export:path [mode]` | Extract content to file (mockup mode only) |
| `mockup-import:path [indent:N\|Ntab]` | Insert file contents (mockup mode only) |

All directives work in all four tag syntaxes. `include:` requires double percent tags (`<%%include:path%%>`). Examples:

```html
<!--%%if:mockup%%-->
  <tr><td>Sample Data</td></tr>
<!--%%endif%%-->

<%note: this text is discarded%>

<!--%%echo:<script>alert("literal")</script>%%-->

<!--%%mockup-export: "/app/assets/css/login.css" append%%-->
.login { padding: 20px; }
<!--%%end%%-->
```

### Front-matter

Files with a `src` attribute can have YAML-like front-matter delimited by `---`:

```
---
skin: default
title: Sign In
css: /assets/signin.css
---
<div class="login">
  <h1>{{.AppName}}</h1>
</div>
```

Front-matter variables are available as percent-tag values. The `skin` key is special — it triggers skin application and is not passed as a variable.

### Skins

A skin is an HTML layout file in the skin directory (default `_skin/`) that uses percent tags:

```html
<!DOCTYPE html>
<html>
<head><title><%title%></title></head>
<body>
<%%content%%>
</body>
</html>
```

`<%%content%%>` is replaced with the processed body. Other front-matter variables (like `<%title%>`) are available in any tag syntax. Escaping is determined by the `<escape>` rules declared in the XML.

The skin directory cascades: `<miniskin skin-dir>` → `<bucket skin-dir>` → `<resource-list skin-dir>`. Default is `_skin`.

### Conditionals

```html
<%if:user%>
  <p>Welcome, <%user%></p>
<%elseif:guest%>
  <p>Guest access</p>
<%else%>
  <p>Please sign in</p>
<%endif%>
```

Negated variants with `if-not:` and `elseif-not:`:

```html
<!--%%if-not:production%%-->
  <div class="debug-bar">Debug Mode</div>
<!--%%endif%%-->
```

Blocks can be nested. Undefined variables inside a false branch do not cause errors.

### Includes

Fragment files included via `<%%include:/path%%>`:

- Resolved relative to `contentPath`
- Can contain their own percent tags (resolved before insertion)
- No front-matter, no skin — raw fragments only
- Never written to disk — resolved in memory
- Cycle detection: if A includes B includes A, generation fails
- A leading UTF-8 BOM is stripped
- Paths with spaces must be quoted: `<%%include:"/my css/site.css"%%>`

#### Minified includes

An include can run its resolved result through the minifier (tdewolff/minify):

```html
<style>
<%%include:/css/site.css minify:css%%>
</style>
```

| Flag | Level |
|---|---|
| `minify:type:0` | none |
| `minify:type:1` | safe — conservative options |
| `minify:type` | safe — same as `minify:type:1` |
| `minify:type:2` | aggressive — maximum minification |

The levels are the same as the front-matter `@minify` directive.

Types: `css`, `js`, `html`, `json`, `svg`, `xml`. An unknown type or level is an error.

The safe level keeps CSS2-compatible output (CSS), variable names (JS), number literals (JSON), and quotes, end tags, document tags, default attribute values and special comments (HTML). SVG and XML minify the same at both levels.

This lets you keep a readable, well-formatted source file and still inline it compressed. It is a shredder with **no guarantees**:

- Percent tags and nested includes are resolved first; the minifier sees the final text
- The included file should contain only valid content of that type. Anything else — e.g. runtime `{{...}}` template actions in CSS or JS — may be mangled
- Anything inside comments is removed
- A minifier error stops the build
- In mockup mode includes are not resolved, so nothing is minified

### Doc-block buffers

`doc-block-begin/end` capture a region of resolved content into a labeled, in-memory buffer instead of emitting it where the markers stand. The buffer can later be replayed verbatim with `doc-block-content` or summarised with `doc-block-toc`:

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

- **Scope**: bucket-global. A buffer captured in one resource-list item is visible from any other item in the same bucket. `ms.docBuffer` is reset at the start of each bucket.
- **Order independence**: the build embed step runs each bucket twice — a dry pre-pass that only populates buffers (no output written, `doc-block-toc/content` emit nothing), then the regular pass with the buffers fully populated. Capture and emit can therefore live in different items in any order.
- **TOC format**: `doc-block-toc` walks the captured markdown for `#` and `##` lines, emitting a nested unordered list. Anchors are slugified GitHub-style (lowercase, alphanumerics and hyphens; duplicates get `-1`, `-2`, …). Lines inside fenced code blocks (` ``` ` or `~~~`) are ignored.
- **Errors**: referencing an unknown buffer with `doc-block-toc` or `doc-block-content` is an error during the regular pass.

## Mockup processing

### Mockup mode

Mockup files are HTML files designed to render standalone in a browser. They are declared in a `<mockup-list>` inside subdirectory `*.miniskin.xml` files (not at the root level). Mockup lists do not use a `file` attribute on items — only `src`.

```xml
<mockup-list save-mode="append">
    <var name="policybanner" value="1" />
    <item src="login_mockup.html">
        <var name="title" value="Sign In" />
    </item>
</mockup-list>
```

In mockup mode:

- The variable `mockup` is automatically set to `1`
- **Variables are not resolved** — `<%title%>` passes through literally
- **Conditionals check existence only** — whether a variable is defined, not its value
- **`mockup-export`** extracts content to files (see below)
- The main output is discarded — only mockup-export side effects matter

Variable merge order: globals → mockup-list vars → item vars → front-matter vars. Skins are applied if declared in front-matter.

### mockup-export / mockup-import

The `mockup-export` directive extracts content from a mockup into a separate file:

```html
<!--%%if:mockup%%-->
<!--%%mockup-export: "/app/assets/css/login.css" append%%-->
.login-card { padding: 20px; border: 1px solid #ccc; }
<!--%%end%%-->
<!--%%endif%%-->
```

The `if:mockup` guard ensures the block is silently skipped during normal processing (no error). In a browser, the `<!--%%...%%-->` tags are hidden, so the CSS renders inline.

`mockup-import` reads a file and inserts its content inline. It works as a single tag or as a block tag:

```html
<!--%%mockup-import:/app/assets/css/login.css%%-->
```

As a block tag, the inline content is kept between the import and `end` tags. The `run` command automatically updates this content from the referenced file:

```html
<!--%%mockup-import:/app/assets/css/login.css%%-->
.login-card { padding: 20px; border: 1px solid #ccc; }
<!--%%end%%-->
```

This keeps mockup files self-contained and browser-renderable while the exported files remain the source of truth.

**Indentation:** The `indent:N` flag prepends N spaces to each non-empty line of the imported content. Use `indent:Ntab` for tabs. This is useful when importing fragments into indented contexts:

```html
<div>
    <section>
        <!--%%mockup-import:"/shared/nav.html" indent:8%%-->
    </section>
</div>
```

**BOM:** a leading UTF-8 BOM in the imported file is stripped, so it never lands in the middle of the mockup.

**Percent tags in imported content:** the inline content may carry percent tags of its own (e.g. `<%html:page_class%>`). The block is closed by its matching `end-mockup-import`, not by the next tag, so repeated updates replace the content instead of duplicating it. A generic `end` is only recognised right after the import tag.

**Nesting:** `mockup-import` inside `mockup-export` works normally (the imported content becomes part of the export). `mockup-export` inside `mockup-import` is ignored — imported content is inserted as raw text without parsing.

Quoted paths are supported for filenames with spaces: `mockup-export: "/path with spaces/file.css" append`

**Save-mode cascade:** The write mode for mockup-export follows a cascade: `<mockup-list save-mode>` → `<item save-mode>` → tag-level mode (the optional `append` or `overwrite` after the path).

**touchedFiles behavior:** The first write to a given file in a session always truncates (clean start per execution). Subsequent writes respect the mode: `append` adds content, `overwrite` replaces it.

### Dependency analysis

Mockup files that use `mockup-export` and `mockup-import` form a dependency graph. If file A exports to `/x.css` and file B imports `/x.css`, then B depends on A and A must be processed first.

Dependencies are resolved at the **export block level**, not the file level. This means dependencies within the same file are also handled. If a file contains an export block that imports a path produced by another export block later in the same file, the system detects this and processes the blocks in the correct order (multiple passes over the file if needed).

```html
<!--%%mockup-export:/css/combined.css%%-->
  <!--%%mockup-import:/css/base.css%%-->
<!--%%end%%-->
<!--%%mockup-export:/css/base.css%%-->
  .base { margin: 0; }
<!--%%end%%-->
```

In this example, `base.css` is exported first (no dependencies), then `combined.css` is exported and correctly imports the freshly written `base.css`.

`AnalyzeDeps()` builds the cross-file dependency graph, detects circular dependencies, and computes the correct processing order via topological sort. `Run()` calls this automatically and returns an error if cycles are detected.

```go
ms := miniskin.MiniskinNew(contentPath, modulesPath)
dm, err := ms.AnalyzeDeps()
if err != nil {
    log.Fatal(err)
}
fmt.Print(dm.String())

order, err := dm.ProcessingOrder()
// order: ["a_mockup.html", "b_mockup.html", ...] (dependencies first)
```

### Negative templates

Adding `negative="filename"` to a mockup item generates a reverse template:

```xml
<mockup-list>
    <item src="login_mockup.html" negative="login_negative.html" />
</mockup-list>
```

The transformation replaces all `mockup-export:path...end` blocks with `mockup-import:path...end-mockup-import` blocks. This produces a template that imports the exported files instead of containing them inline.

**Original mockup:**

```html
<!--%%if:mockup%%-->
<!--%%mockup-export:/css/login.css%%-->
.login { padding: 20px; }
<!--%%end%%-->
<!--%%endif%%-->
```

**Generated negative:**

```html
<!--%%if:mockup%%-->
<!--%%mockup-import:/css/login.css%%-->
<!--%%end-mockup-import%%-->
<!--%%endif%%-->
```

Nested `mockup-export` blocks each produce one `mockup-import...end-mockup-import` block. All other content (conditionals, variables, etc.) is preserved.

## External / Origin

Some projects need to consume an artefact produced by a sibling project (e.g. a UI library kept in its own repo) without checking that artefact into the consuming repo's history. `<external>` blocks copy such files into place at build time, sourcing them from a per-developer registry.

### Registry: `miniskin-origin.xml`

The registry lives at `contentPath` root and is **per developer**: it should be `.gitignore`d so each contributor can point origins at their own local clones / build outputs. The file is optional — only required when `<external>` blocks exist somewhere in the project.

```xml
<miniskin>
  <origin name="closure-ui">
    <local>C:\HD\F\_sams\closure-ui</local>
  </origin>
</miniskin>
```

`<local>` is the only supported source: an absolute filesystem path. Each developer is expected to clone and build the sibling project themselves; miniskin only handles the copy. Network sources (`<github-release>`, `<http>`) are intentionally **not** supported — `<local>` already keeps the projects separated, and adding fetch/cache/auth would invite cross-platform pain (TLS roots, proxies, Windows path quirks, token conventions) for no real gain over the current workflow.

### `<external>` block

Lives in any subdirectory `*.miniskin.xml` (anywhere a `<resource-list>` or `<mockup-list>` could appear). Each `<external-item>` describes one copy:

```xml
<external>
  <external-item origin="closure-ui"
                 src="./release/closure_ui.js"
                 dstfile="./src/app_source.js" />
</external>
```

| Attribute | Description |
|---|---|
| `origin`  | name of an entry in `miniskin-origin.xml` |
| `src`     | path inside the origin's `<local>` root |
| `dstfile` | destination relative to the directory of the declaring XML (same convention as `<item src>`) |

Once copied, downstream `<item src="./src/app_source.js">` references the file like any other source.

### Behaviour

- Runs as **step 0** of `Run()` (and as a prelude to `BuildEmbed()`), before dependency analysis, mockup processing, and build embed — so item sources that depend on copied files exist before assembly.
- Copy is **mtime+size-aware**: dst is left untouched when it matches the source; otherwise it is overwritten and the source mtime is propagated to dst. A second run is a no-op.
- Parent directory of `dstfile` is created if missing.
- Errors are hard and include absolute paths plus the declaring XML:
  - origin name not found in the registry
  - origin has no `<local>` (placeholder for future source kinds; currently always an error)
  - source file missing
  - `external-item` missing the `origin` attribute

### Workflow

Each repo decides its own commit/integration cadence:

1. Work on the sibling project, build it, commit when ready in **its** repo.
2. In the consuming project, run `miniskin run` — externals refresh automatically based on file mtime.
3. Commit the copied artefact to the consuming repo when you're ready to ship it.

The `miniskin-origin.xml` registry stays out of git, so each developer (and CI, if desired) can resolve origins differently.

## API

### High-level functions

| Function | Description |
|---|---|
| `MiniskinRun(contentPath, modulesPath, verbosity...)` | Full pipeline: mockup update + build + codegen |
| `MiniskinGenerate(contentPath, modulesPath, verbosity...)` | Build embed assets + codegen (no mockup processing) |
| `MiniskinMockupUpdate(contentPath, modulesPath, verbosity...)` | Deps check + mockup export + refresh imports |
| `TransformNegative(content)` | Transform a single mockup string into a negative template |
| `CombineDir(dir)` | Combine subdirectory XMLs into a single XML with nested resource-lists |
| `SplitXML(xmlPath)` | Split nested resource-lists into separate XMLs per subdirectory |

### Types

| Type | Constructor | Description |
|---|---|---|
| `Miniskin` | `MiniskinNew(contentPath, modulesPath)` | Template processor |
| `Codegen` | `CodegenNew(contentPath, modulesPath)` | Code generator |
| `DepMap` | _(returned by `AnalyzeDeps`)_ | Dependency graph with cycle detection |

`Miniskin` methods:

| Method | Description |
|---|---|
| `Run()` | Full pipeline: deps + export + update + build |
| `AnalyzeDeps()` | Build dependency map, detect cycles |
| `UpdateImports()` | Refresh inline content in mockup-import blocks |
| `ProcessMockupExport()` | Export only (pass 1) |
| `BuildEmbed()` | Build only (pass 2) |
| `SetVerbosity(v)` | Set log detail level |
| `Silent()` | Disable console output |

`DepMap` methods:

| Method | Description |
|---|---|
| `ProcessingOrder()` | Topological sort (dependencies first). Error if cycles exist |
| `HasCycles()` | Returns true if circular dependencies were detected |
| `String()` | Human-readable dependency map |

### Verbosity

Control the level of log detail:

| Level | Constant | Description |
|---|---|---|
| 0 | `VerbositySilent` | No console output |
| 1 | `VerbosityNormal` | Phase headers and processed items (default) |
| 2 | `VerbosityVerbose` | + dependency analysis, processing order |
| 3 | `VerbosityDebug` | + all internal details |

```go
ms := miniskin.MiniskinNew(contentPath, modulesPath)
ms.SetVerbosity(miniskin.VerbosityVerbose)

// Or via MiniskinRun:
miniskin.MiniskinRun(contentPath, modulesPath, miniskin.VerbosityVerbose)
```

### Log output

By default, processing steps are logged to stdout. Verbosity controls what is shown.

```go
ms.Silent()                  // disable console output
ms.Output = os.Stderr        // redirect
ms.Output = myWriter         // any io.Writer
```

If the XML specifies a log file, output is written to both console and file (the log file always receives output regardless of verbosity):

```xml
<miniskin log="miniskin.log">
```

### Generated files tracking

`Result.GeneratedFiles` lists all files created by mockup-export and negative generation, in creation order, with their source:

```go
for _, gf := range result.GeneratedFiles {
    fmt.Printf("%s (from: %s)\n", gf.File, gf.Source)
}
```

## XML format

All configuration uses `*.miniskin.xml` files with a `<miniskin>` root element.

### Root

The root XML file (in `contentPath`) declares globals, escape rules, the bucket list, and optionally a log file and skin directory:

```xml
<miniskin skin-dir="_skin" log="miniskin.log">
  <globals>
    <var name="appName" value="MyApp" />
  </globals>

  <escape ext="*.html,*.html.tmpl" as="html" />
  <escape ext="*.js,*.js.tmpl" as="js" />

  <bucket-list filename="generated_embed.go" module="content" import="myproject/content"
               template="custom_embed.tmpl">
    <bucket src="app" dst="/modules/app/reqctx/generated_assets.go"
            module-name="reqctx" recurse-folder="all" skin-dir="app/_skin"
            template="custom_bucket.tmpl"
            template-function-map="MyTemplateFuncMap()" />
  </bucket-list>
</miniskin>
```

`<bucket-list>` accepts an `omit` attribute to skip codegen outputs.
Values are comma- or space-separated:

| Value | Effect |
|---|---|
| `embed` | skip the embed file (`Codegen.GenerateEmbed`) |
| `module` | skip per-bucket module files (`Codegen.GenerateBucketFile`) |

```xml
<bucket-list omit="embed,module">
  <bucket src=".">
    <resource-list>
      <item type="static,parse" src="./_source.list" file="bundle.js" />
    </resource-list>
  </bucket>
</bucket-list>
```

When both outputs are omitted, `filename` and `module` may be left
unset — useful when miniskin is being used to assemble assets for a
non-Go project (e.g. a JavaScript bundle).

### Subdirectory

Subdirectory `*.miniskin.xml` files contain one or more `<resource-list>` elements and/or a `<mockup-list>`:

```xml
<miniskin>
  <resource-list urlbase="/assets" skin-dir="rskins">
    <item type="static" file="app.css" />
    <item type="static" src="combined_src.css" file="combined.css" />
  </resource-list>

  <mockup-list save-mode="overwrite">
    <var name="policybanner" value="1" />
    <item src="mockup_login.html" negative="login_negative.html" save-mode="append">
      <var name="title" value="Login" />
    </item>
  </mockup-list>
</miniskin>
```

Resource lists can be **chained** (multiple at the same level) and **nested** (child resource-lists inside a parent). A nested `<resource-list>` uses `src` to set its base directory relative to the parent:

```xml
<miniskin>
  <resource-list urlbase="/assets">
    <item type="static" file="app.css" />
  </resource-list>
  <resource-list urlbase="/pages">
    <item type="static" file="index.html" />
    <resource-list src="login" urlbase="/login">
      <item type="static" file="signin.html" />
    </resource-list>
  </resource-list>
</miniskin>
```

Attributes `skin-dir`, `mux-include`, `mux-exclude`, `template-function-map`, and `<escape>` rules cascade from parent to child resource-lists, following the same override pattern used throughout the XML hierarchy.

### Items

Each item in a resource list describes a content file:

```xml
<item type="html-template,nomux,parse" src="signin_src.html" file="signin.html" key="/login/signin" />
```

- `file` — output filename (what gets embedded)
- `src` — source filename (optional; if present, item is processed through the template engine)
- `type` — comma-separated flags: `static`, `html-template`, `response`, `nomux`, `parse`, etc.
- `key` — logical key for asset lookup
- `url` / `alt-url-abs` — URL routing attributes
- `escape` — override default escape type for this item (`html`, `js`, `url`, `sql`, etc.)
- `template-function-map` — Go expression returning `template.FuncMap` for this item (overrides parent)

If `src` is absent, `file` is embedded as-is (no processing).

### Mux include/exclude

The `mux-include` and `mux-exclude` attributes control which items are registered on the HTTP mux. Items excluded by these patterns automatically receive the `nomux` flag (they go to the `assets` map instead of being registered as routes).

These attributes cascade through three levels, each overriding the parent when set:

```
<miniskin>  →  <bucket-list>  →  <bucket>  →  <resource-list>
```

| Attribute | Default | Description |
|---|---|---|
| `mux-include` | `*` | Comma-separated glob patterns; only matching files are included in the mux |
| `mux-exclude` | *(empty)* | Comma-separated glob patterns; matching files are excluded from the mux |

An item is excluded from the mux (gets `nomux` added to its type) if:
- Its filename does **not** match `mux-include`, OR
- Its filename matches `mux-exclude`

Items with an explicit `nomux` flag in their `type` attribute are always excluded regardless of patterns.

Example: include only static assets, exclude templates:

```xml
<miniskin mux-include="*.js,*.css,*.png,*.jpg,fav.ico">
  <bucket-list filename="embed.go" module="content">
    <bucket src="app" dst="/gen.go" module-name="app" />
  </bucket-list>
</miniskin>
```

Equivalent using `mux-exclude`:

```xml
<miniskin mux-exclude="*.html,*.tmpl">
  <bucket-list filename="embed.go" module="content">
    <bucket src="app" dst="/gen.go" module-name="app" />
  </bucket-list>
</miniskin>
```

Override at a lower level:

```xml
<bucket-list mux-exclude="*.html">
  <bucket src="api" dst="/gen.go" module-name="api" mux-exclude="" />
  <!-- api bucket inherits mux-exclude="" only if non-empty; empty = inherit parent -->
</bucket-list>
```

Patterns use Go's `filepath.Match` syntax (e.g. `*.css`, `fav.ico`, `app-*.js`).

### Template function map

The `template-function-map` attribute injects a `template.FuncMap` into parsed templates (items with the `parse` flag). The value is a Go expression that returns `template.FuncMap`, typically a function call.

Cascades through three levels, each overriding the parent when set:

```
<bucket>  →  <resource-list>  →  <item>
```

```xml
<bucket src="app" dst="/gen.go" module-name="app"
        template="miniskin::mux"
        template-function-map="AppFuncMap()">
  <!-- All parsed items in this bucket use AppFuncMap() -->
</bucket>
```

Override at resource-list or item level:

```xml
<resource-list urlbase="/admin" template-function-map="AdminFuncMap()">
  <item type="html-template,nomux,parse" file="page.html" key="/admin/page" />
  <item type="html-template,nomux,parse" file="special.html" key="/admin/special"
        template-function-map="SpecialFuncMap()" />
</resource-list>
```

The generated code calls `.Funcs(expr)` before `.Parse()`:

```go
// Without template-function-map:
parsedTemplates["/page"] = template.Must(template.New("/page").Parse(string(content.PageHtml)))

// With template-function-map="AppFuncMap()":
parsedTemplates["/page"] = template.Must(template.New("/page").Funcs(AppFuncMap()).Parse(string(content.PageHtml)))
```

The function must be defined in the same package as the generated bucket file and must return `template.FuncMap`.

## Generated Go code

### Embed file

The embed file (e.g. `generated_embed.go`) contains one `//go:embed` directive and `[]byte` variable per item:

```go
package content

import _ "embed"

//go:embed app/assets/app.css
var AppAssetsAppCss []byte
```

Each variable is a direct pointer to a binary segment — no copy, no decompression.

### Bucket file

miniskin includes two built-in bucket templates, selectable via the `template` attribute:

#### `miniskin::default`

Used when no `template` attribute is specified. Generates an `Asset` type, an asset slice, and generic lookup/registration functions:

```go
type Asset struct {
    Key  string
    Data []byte
    Mime string
    Type string
}

func Assets() []Asset
func Get(key string) *Asset
func GetParsedTemplate(key string) *template.Template
func StaticFiles() []Asset
func Templates() []Asset
func RegisterRoutes(fn func(url, mime string, data []byte))
```

Items with the `parse` flag are pre-parsed as `*template.Template` at init time. `RegisterRoutes` calls the callback for each `static` item not flagged `nomux`.

#### `miniskin::mux`

Generates code that registers routes directly on an `*http.ServeMux`:

```go
func RegisterRoutes(mux *http.ServeMux, tmplHandlers map[string]http.HandlerFunc)
func GetParsedTemplate(key string) *template.Template
var Templates map[string][]byte
```

`RegisterRoutes` registers static files with exact-path matching and wires template routes via the `tmplHandlers` map.

Items flagged `response` are registered via `serveResponse`, which replays a
canned HTTP response embedded as a raw `.http` file (status line, optional
headers, blank line, optional body). Use it for redirects (`3xx` + `Location`),
bare statuses (`404`, `410`, …), and static error pages — the route comes from
`key`:

```xml
<item type="response" file="old-page.http" key="/old-page/" />
```

```
301 Moved Permanently
Location: https://www.example.com/products

```

The bytes are fixed at build time, like `static`; declared headers are sent
verbatim (net/http still adds `Date`/`Content-Length` and sniffs `Content-Type`
when omitted). Anything dynamic belongs in your own `mux.HandleFunc`.

Usage in XML:

```xml
<bucket template="miniskin::mux" ... />
```

## Custom templates

The `template` attribute on `<bucket-list>` and `<bucket>` accepts three forms:

| Value | Source |
|---|---|
| _(empty)_ | Built-in `miniskin::default` |
| `miniskin::name` | Built-in named template |
| `path/to/file.tmpl` | Custom template file relative to contentPath |

```xml
<bucket-list template="my_embed.tmpl" ...>
    <bucket template="miniskin::mux" ... />
</bucket-list>
```

### Embed template

Available functions: `embedPath`, `embedVar`. Data: full `Result` struct.

### Bucket template

Available functions: `embedVar`, `mimeType`, `hasFlag`, `embedPkg`, `embedImport`. Data: `BucketList`, `Bucket`, `Items`.

If a custom embed template is used, custom bucket templates must also be provided (since variable names may differ).

## Usage

The simplest way — one function call:

```go
if err := miniskin.MiniskinRun(contentPath, modulesPath); err != nil {
    log.Fatal(err)
}
```

With verbose output:

```go
if err := miniskin.MiniskinRun(contentPath, modulesPath, miniskin.VerbosityVerbose); err != nil {
    log.Fatal(err)
}
```

Mockup update only (export + refresh imports):

```go
miniskin.MiniskinMockupUpdate(contentPath, modulesPath)
```

Generate only (build + codegen, no mockup processing):

```go
miniskin.MiniskinGenerate(contentPath, modulesPath)
```

Single-file negative transform (no XML needed):

```go
result := miniskin.TransformNegative(content)
```

For more control, use the types separately:

```go
ms := miniskin.MiniskinNew(contentPath, modulesPath)
ms.SetVerbosity(miniskin.VerbosityVerbose)

result, err := ms.Run()
if err != nil {
    log.Fatal(err) // includes circular dependency errors
}

cg := miniskin.CodegenNew(contentPath, modulesPath)
if err := cg.GenerateAll(result); err != nil {
    log.Fatal(err)
}
```

Dependency analysis only:

```go
ms := miniskin.MiniskinNew(contentPath, modulesPath)
dm, err := ms.AnalyzeDeps()
if err != nil {
    log.Fatal(err)
}
fmt.Print(dm.String())
order, _ := dm.ProcessingOrder()
for i, src := range order {
    fmt.Printf("%d. %s\n", i+1, src)
}
```

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

## File structure example

```
content/
  content.miniskin.xml          # root: globals + bucket-list
  _skin/
    default.html                # skin layout
  app/
    _shared/
      header.html               # include fragment
    assets/
      assets.miniskin.xml       # resource-list for static files
      app.css
      app.js
    login_dialog/
      login.miniskin.xml        # resource-list + mockup-list
      login_mockup.html         # mockup source (mockup-export inside)
      signin_src.html           # source with front-matter + skin
      signin.html               # generated output (gitignored)
```

## AI assistance

The `_mkskill/src/` directory contains miniskin's documentation in modular
sections — the canonical source. mkskill composes every view from them
(this README, `AGENTS.md` and the Claude skill), and the binary carries the
composed docs embedded, so it hands them out by itself.

### Claude Code

```
miniskin -generate-claude-skill -global
```

Installs `~/.claude/skills/miniskin/SKILL.md` (available from every
project). Without `-global` it writes the project-local
`.claude/skills/miniskin/SKILL.md`; `-dst` picks any other path, `-force`
overwrites.

### Other agents (Cursor, Aider, Windsurf, AGENTS.md, …)

```
miniskin -generate-agent-docs
```

Writes `AGENTS.md` in the current directory — the same content without any
tool-specific frontmatter. Point it elsewhere with `-dst`:

```
miniskin -generate-agent-docs -dst .cursor/rules/miniskin.mdc -force
miniskin -generate-agent-docs -dst CONVENTIONS.md -force
```

Suitable for any tool that accepts plain Markdown context.

## Background

miniskin originated as a helper for `go generate` in projects that needed fine-grained control over which assets are embedded and how they are registered. It is not a static site generator — it is an asset assembler. It does not replace tools like Hugo or Jekyll; it operates at a different layer, producing Go source files that compile into the binary.

## License

MIT
