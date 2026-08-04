---
mkskill:
  pos: 80
  in: readme
---

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

