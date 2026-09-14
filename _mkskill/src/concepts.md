---
mkskill:
  pos: 50
  in: readme
---

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
| `include:path [minify:type[:1]]` | Include file contents (double tags only, resolved recursively), optionally minified |
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
| `minify:type` | safe — conservative options |
| `minify:type:1` | aggressive — maximum minification (same as front-matter `@minify:1`) |

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

