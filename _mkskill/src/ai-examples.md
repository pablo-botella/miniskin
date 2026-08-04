---
mkskill:
  pos: 230
  in: ai*
---

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
