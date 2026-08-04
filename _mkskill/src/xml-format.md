---
mkskill:
  pos: 90
  in: readme
---

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

