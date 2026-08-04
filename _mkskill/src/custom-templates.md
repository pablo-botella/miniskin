---
mkskill:
  pos: 110
  in: readme
---

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

