---
mkskill:
  pos: 70
  in: readme
---

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

