---
mkskill:
  pos: 60
---

## Use with `go generate`

Two options, both produce the same output. Pick whichever fits your project:

### Option A — direct CLI invocation (simpler)

No driver file. Drop the directive in any Go source of your project and you're done:

```go
//go:generate miniskin run -content content
package main
```

Trade-off: every collaborator needs the binary installed (`go install github.com/pablo-botella/miniskin/cmd/miniskin@latest`).

### Option B — library driver (customizable, no install)

Write a tiny `main` in your repo that calls the library, and reference it from `go:generate`. The library becomes a regular module dependency pinned in `go.mod`, so nobody needs to `go install` anything.

```go
// gen.go
package main

import (
	"fmt"
	"os"

	"github.com/pablo-botella/miniskin"
)

func main() {
	if err := miniskin.MiniskinRun("content", "."); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
```

```go
//go:generate go run ./gen.go
package main
```

Use this option when you need to customize the pipeline (extra steps before/after, wrapping `MiniskinNew` directly, environment-driven configuration, etc.).

In both cases, `go generate ./...` then rebuilds the embedded assets as part of the standard Go workflow.

