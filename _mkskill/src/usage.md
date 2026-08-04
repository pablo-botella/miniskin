---
mkskill:
  pos: 120
  in: readme
---

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

