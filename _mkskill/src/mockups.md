---
mkskill:
  pos: 60
  in: readme
---

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

