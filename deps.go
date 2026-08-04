package miniskin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DepEdge represents a dependency between a mockup source file and a target file.
type DepEdge struct {
	Source string // mockup source file (relative to contentPath)
	Target string // exported/imported file path
	Kind   string // "export" or "import"
}

// DepMap holds the dependency graph for mockup-export/mockup-import directives.
type DepMap struct {
	Edges  []DepEdge
	Cycles [][]string // circular dependency chains
}

// String returns a human-readable representation of the dependency map.
func (dm *DepMap) String() string {
	var b strings.Builder

	// Group edges by source
	bySource := make(map[string][]DepEdge)
	for _, e := range dm.Edges {
		bySource[e.Source] = append(bySource[e.Source], e)
	}

	b.WriteString("=== Dependency Map ===\n")
	for src, edges := range bySource {
		b.WriteString(fmt.Sprintf("  %s\n", src))
		for _, e := range edges {
			b.WriteString(fmt.Sprintf("    -[%s]-> %s\n", e.Kind, e.Target))
		}
	}

	if len(dm.Cycles) > 0 {
		b.WriteString("\n=== Circular Dependencies ===\n")
		for _, cycle := range dm.Cycles {
			b.WriteString("  " + strings.Join(cycle, " → ") + "\n")
		}
	} else {
		b.WriteString("\nNo circular dependencies.\n")
	}
	return b.String()
}

// HasCycles returns true if any circular dependencies were detected.
func (dm *DepMap) HasCycles() bool {
	return len(dm.Cycles) > 0
}

// AnalyzeDeps walks all mockup files and builds a dependency map.
// Detects circular dependencies between mockup sources via export/import chains.
func (ms *Miniskin) AnalyzeDeps() (*DepMap, error) {
	_, bl, err := ms.init()
	if err != nil {
		return nil, err
	}

	dm := &DepMap{}

	for _, bucket := range bl.Buckets {
		if err := ms.walkBucket(bucket, func(parsed *xmlMiniskin, dir string, _ string) error {
			if parsed.MockupList == nil {
				return nil
			}
			for _, mi := range parsed.MockupList.Items {
				srcPath := absPath(filepath.Join(dir, mi.Src))
				data, err := os.ReadFile(srcPath)
				if err != nil {
					return fmt.Errorf("reading %s: %w", srcPath, err)
				}
				exports, imports := scanExportsImports(string(data))
				relSrc, err := filepath.Rel(ms.contentPath, srcPath)
				if err != nil {
					return fmt.Errorf("computing path of %s relative to content root %s: %w", srcPath, ms.contentPath, err)
				}
				relSrc = filepath.ToSlash(relSrc)
				for _, e := range exports {
					dm.Edges = append(dm.Edges, DepEdge{Source: relSrc, Target: e, Kind: "export"})
				}
				for _, imp := range imports {
					dm.Edges = append(dm.Edges, DepEdge{Source: relSrc, Target: imp, Kind: "import"})
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	dm.Cycles = detectCycles(dm.Edges)
	return dm, nil
}

// scanExportsImports extracts mockup-export and mockup-import file paths from content.
func scanExportsImports(content string) (exports, imports []string) {
	walkTags(content, func(tag string) {
		trimmed := strings.TrimSpace(tag)
		if ef, ok := isMockupExport(trimmed); ok {
			exports = append(exports, ef.filename)
		}
		if imf, ok := isMockupImport(trimmed); ok {
			imports = append(imports, imf.filename)
		}
	})
	return
}

// walkTags calls fn for each percent-tag content found in the input.
// Recognizes all four tag syntaxes: <%...%>, <%%...%%>, <!--%...%-->, <!--%%...%%-->.
func walkTags(content string, fn func(tag string)) {
	var tag strings.Builder
	state := stText

	for i := 0; i < len(content); i++ {
		c := content[i]
		switch state {
		case stText:
			// /*<% — JS-comment wrapper opener: skip /* and let < be consumed
			if c == '/' && i+3 < len(content) && content[i+1] == '*' && content[i+2] == '<' && content[i+3] == '%' {
				i += 2
				state = stLT
				continue
			}
			if c == '<' {
				state = stLT
			}
		case stLT:
			switch c {
			case '%':
				state = stLTPct
			case '!':
				state = stCmtBang
			default:
				state = stText
			}
		case stLTPct:
			switch c {
			case '%':
				state = stLTPctPct
			default:
				tag.Reset()
				tag.WriteByte(c)
				state = stSingle
			}
		case stSingle:
			switch c {
			case '%':
				state = stSingleClose
			default:
				tag.WriteByte(c)
			}
		case stSingleClose:
			switch c {
			case '>':
				// %>*/ — JS-comment wrapper closer: also consume */
				if i+2 < len(content) && content[i+1] == '*' && content[i+2] == '/' {
					i += 2
				}
				fn(tag.String())
				state = stText
			case '-':
				state = stSCmtD1
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stSingle
			}
		case stLTPctPct:
			tag.Reset()
			tag.WriteByte(c)
			state = stDouble
		case stDouble:
			switch c {
			case '%':
				state = stDoubleClose1
			default:
				tag.WriteByte(c)
			}
		case stDoubleClose1:
			switch c {
			case '%':
				state = stDoubleClose2
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stDouble
			}
		case stDoubleClose2:
			switch c {
			case '>':
				// %%>*/ — JS-comment wrapper closer: also consume */
				if i+2 < len(content) && content[i+1] == '*' && content[i+2] == '/' {
					i += 2
				}
				fn(tag.String())
				state = stText
			case '-':
				state = stDCmtD1
			default:
				tag.WriteByte('%')
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stDouble
			}
		case stCmtBang:
			switch c {
			case '-':
				state = stCmtDash1
			default:
				state = stText
			}
		case stCmtDash1:
			switch c {
			case '-':
				state = stCmtDash2
			default:
				state = stText
			}
		case stCmtDash2:
			switch c {
			case '%':
				state = stCmtPct1
			default:
				state = stText
			}
		case stCmtPct1:
			switch c {
			case '%':
				tag.Reset()
				state = stCmtTag
			default:
				tag.Reset()
				tag.WriteByte(c)
				state = stCmtSingle
			}
		case stCmtSingle:
			switch c {
			case '%':
				state = stCmtSClose1
			default:
				tag.WriteByte(c)
			}
		case stCmtSClose1:
			switch c {
			case '>':
				fn(tag.String())
				state = stText
			case '-':
				state = stCmtSClose2
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stCmtSingle
			}
		case stCmtSClose2:
			switch c {
			case '-':
				state = stCmtSClose3
			default:
				tag.WriteString("%-")
				tag.WriteByte(c)
				state = stCmtSingle
			}
		case stCmtSClose3:
			switch c {
			case '>':
				fn(tag.String())
				state = stText
			default:
				tag.WriteString("%--")
				tag.WriteByte(c)
				state = stCmtSingle
			}
		case stCmtTag:
			switch c {
			case '%':
				state = stCmtClose1
			default:
				tag.WriteByte(c)
			}
		case stCmtClose1:
			switch c {
			case '%':
				state = stCmtClose2
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stCmtTag
			}
		case stCmtClose2:
			switch c {
			case '>':
				fn(tag.String())
				state = stText
			case '-':
				state = stCmtClose3
			default:
				tag.WriteByte('%')
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stCmtTag
			}
		case stCmtClose3:
			switch c {
			case '-':
				state = stCmtClose4
			default:
				tag.WriteString("%%-")
				tag.WriteByte(c)
				state = stCmtTag
			}
		case stCmtClose4:
			switch c {
			case '>':
				fn(tag.String())
				state = stText
			default:
				tag.WriteString("%%--")
				tag.WriteByte(c)
				state = stCmtTag
			}

		// <%...%--> (single open, comment close)
		case stSCmtD1:
			switch c {
			case '-':
				state = stSCmtD2
			default:
				tag.WriteString("%-")
				tag.WriteByte(c)
				state = stSingle
			}
		case stSCmtD2:
			switch c {
			case '>':
				fn(tag.String())
				state = stText
			default:
				tag.WriteString("%--")
				tag.WriteByte(c)
				state = stSingle
			}

		// <%%...%%--> (double open, comment close)
		case stDCmtD1:
			switch c {
			case '-':
				state = stDCmtD2
			default:
				tag.WriteString("%%-")
				tag.WriteByte(c)
				state = stDouble
			}
		case stDCmtD2:
			switch c {
			case '>':
				fn(tag.String())
				state = stText
			default:
				tag.WriteString("%%--")
				tag.WriteByte(c)
				state = stDouble
			}
		}
	}
}

// ProcessingOrder returns mockup source files sorted so that dependencies
// are processed before the files that import them.
// Returns an error if the graph contains cycles.
func (dm *DepMap) ProcessingOrder() ([]string, error) {
	if dm.HasCycles() {
		return nil, fmt.Errorf("cannot determine processing order: circular dependencies detected")
	}

	sources, deps := buildSourceGraph(dm.Edges)

	// Kahn's algorithm (topological sort via BFS)
	inDegree := make(map[string]int)
	for src := range sources {
		inDegree[src] = 0
	}
	for src, depsSet := range deps {
		inDegree[src] += 0 // ensure entry exists
		for dep := range depsSet {
			_ = dep // counted below
		}
	}
	// Count incoming edges
	for src, depsSet := range deps {
		_ = src
		for dep := range depsSet {
			inDegree[dep] += 0 // ensure entry
		}
	}
	// Reverse: for each A→B (A depends on B), B has an outgoing edge to A
	reverse := make(map[string][]string)
	for src, depsSet := range deps {
		for dep := range depsSet {
			reverse[dep] = append(reverse[dep], src)
			inDegree[src]++ // src has one more incoming dependency
		}
	}

	// Start with nodes that have no dependencies
	var queue []string
	for src := range sources {
		if inDegree[src] == 0 {
			queue = append(queue, src)
		}
	}
	// Sort queue for deterministic output
	sortStrings(queue)

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)
		for _, dependent := range reverse[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	return order, nil
}

// sortStrings sorts a string slice in place (simple insertion sort, small slices).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// buildSourceGraph resolves edges into a source-level dependency graph.
// Returns the set of all sources and a map: source → set of sources it depends on.
func buildSourceGraph(edges []DepEdge) (sources map[string]bool, deps map[string]map[string]bool) {
	// Map: target file → source that exports it
	producedBy := make(map[string]string)
	for _, e := range edges {
		if e.Kind == "export" {
			producedBy[e.Target] = e.Source
		}
	}

	sources = make(map[string]bool)
	deps = make(map[string]map[string]bool)
	for _, e := range edges {
		sources[e.Source] = true
		if e.Kind == "import" {
			if producer, ok := producedBy[e.Target]; ok && producer != e.Source {
				if deps[e.Source] == nil {
					deps[e.Source] = make(map[string]bool)
				}
				deps[e.Source][producer] = true
			}
		}
	}
	return
}

// scanExportDeps identifies which imports are inside which export blocks.
// Returns a map: export path → list of import paths contained within that export.
func scanExportDeps(content string) map[string][]string {
	result := make(map[string][]string)
	type frame struct {
		isExport bool
		path     string
	}
	var stack []frame

	walkTags(content, func(tag string) {
		trimmed := strings.TrimSpace(tag)
		if ef, ok := isMockupExport(trimmed); ok {
			if _, exists := result[ef.filename]; !exists {
				result[ef.filename] = nil
			}
			stack = append(stack, frame{isExport: true, path: ef.filename})
		} else if strings.HasPrefix(trimmed, "if:") || strings.HasPrefix(trimmed, "if-not:") {
			stack = append(stack, frame{isExport: false})
		} else if trimmed == "end" || trimmed == "end-if" || trimmed == "endif" ||
			trimmed == "end-mockup-export" || trimmed == "end-mockup-import" {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		} else if imf, ok := isMockupImport(trimmed); ok {
			// Find nearest enclosing export block
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].isExport {
					result[stack[i].path] = append(result[stack[i].path], imf.filename)
					break
				}
			}
		}
	})

	return result
}

// exportProcessingOrder returns export paths in topological order.
// An export A depends on export B if A contains a mockup-import of B's path.
func exportProcessingOrder(exportDeps map[string][]string) []string {
	// Build dependency graph
	deps := make(map[string]map[string]bool)
	for exportPath, imports := range exportDeps {
		for _, imp := range imports {
			if _, isExport := exportDeps[imp]; isExport && imp != exportPath {
				if deps[exportPath] == nil {
					deps[exportPath] = make(map[string]bool)
				}
				deps[exportPath][imp] = true
			}
		}
	}

	// Topological sort (Kahn's algorithm)
	inDegree := make(map[string]int)
	for path := range exportDeps {
		inDegree[path] = 0
	}
	reverse := make(map[string][]string)
	for path, depSet := range deps {
		for dep := range depSet {
			reverse[dep] = append(reverse[dep], path)
			inDegree[path]++
		}
	}

	var queue []string
	for path := range exportDeps {
		if inDegree[path] == 0 {
			queue = append(queue, path)
		}
	}
	sortStrings(queue)

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)
		for _, dependent := range reverse[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	return order
}

// hasInternalDeps returns true if any export block imports a path
// that is also exported by another block in the same content.
func hasInternalDeps(exportDeps map[string][]string) bool {
	for exportPath, imports := range exportDeps {
		for _, imp := range imports {
			if _, isExport := exportDeps[imp]; isExport && imp != exportPath {
				return true
			}
		}
	}
	return false
}

// detectCycles finds circular dependencies between mockup sources.
// A cycle exists when source A imports a file produced by source B,
// and source B (directly or transitively) imports a file produced by A.
func detectCycles(edges []DepEdge) [][]string {
	sources, deps := buildSourceGraph(edges)

	// DFS cycle detection with coloring
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	parent := make(map[string]string)
	var cycles [][]string

	var dfs func(node string)
	dfs = func(node string) {
		color[node] = gray
		for dep := range deps[node] {
			switch color[dep] {
			case white:
				parent[dep] = node
				dfs(dep)
			case gray:
				// Reconstruct cycle
				cycle := []string{dep}
				curr := node
				for curr != dep {
					cycle = append(cycle, curr)
					curr = parent[curr]
				}
				cycle = append(cycle, dep)
				// Reverse to get natural order
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}
				cycles = append(cycles, cycle)
			}
		}
		color[node] = black
	}

	for src := range sources {
		if color[src] == white {
			dfs(src)
		}
	}

	return cycles
}
