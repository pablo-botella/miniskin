---
mkskill:
  pos: 100
  in: readme
---

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

