// Package miniskin is a build-time template assembler for Go projects.
// It processes content files (HTML, CSS, JS) with percent-tag substitution,
// includes, skins (layouts), and mockup extraction.
//
// # Quick start
//
// For the simplest usage, call [MiniskinRun] to run the full pipeline
// (mockup update, build, and code generation) in a single call.
// Use [MiniskinGenerate] for build + code generation without mockup processing,
// or [MiniskinMockupUpdate] for mockup export + import refresh only.
//
// # Pipeline
//
// The [Miniskin.Run] method executes four steps:
//  1. Analyze dependencies between mockup files and detect circular references
//  2. Process mockup exports (extract content to files via mockup-export directives)
//  3. Update imports (refresh inline content in mockup-import blocks)
//  4. Build embed (process resource items, resolve variables, apply skins)
//
// # Dependency analysis
//
// Use [Miniskin.AnalyzeDeps] to inspect the dependency graph between mockup files.
// The returned [DepMap] provides [DepMap.ProcessingOrder] for topological ordering
// and [DepMap.HasCycles] for circular dependency detection.
//
// # Verbosity
//
// Control log detail with [Miniskin.SetVerbosity] or [Miniskin.Silent].
// [VerbosityNormal] logs phase headers, [VerbosityVerbose] adds dependency analysis
// and processing order, [VerbosityDebug] logs all internal details.
//
// # Types
//
// For finer control, use [MiniskinNew] to create a [Miniskin] instance for
// template processing, and [CodegenNew] to create a [Codegen] instance for
// Go source file generation.
package miniskin

// The documentation is not automatic — this makes it: `go generate ./...`
// rebuilds every artifact from _mkskill/ (README, AGENTS.md, the skill and
// the cmd README; the tool version is pinned by go.mod), and the golden
// test fails when anything went stale. The second line propagates the
// version-spec's destinations (__publish.bat) without touching the version.
//go:generate go run github.com/pablo-botella/mkskill/cmd/mkskill -q build
//go:generate go run github.com/pablo-botella/mkskill/cmd/mkskill -q -vbuild

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// absPath converts a path to absolute for clearer error messages.
func absPath(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

// GeneratedFile records a file created by mockup-export.
type GeneratedFile struct {
	File   string // output path relative to contentPath
	Source string // source file that triggered the generation
}

// Verbosity controls the level of detail in log output.
type Verbosity int

const (
	// VerbositySilent disables all output.
	VerbositySilent Verbosity = 0
	// VerbosityNormal logs phase headers and processed items.
	VerbosityNormal Verbosity = 1
	// VerbosityVerbose also logs dependency analysis and processing order.
	VerbosityVerbose Verbosity = 2
	// VerbosityDebug logs everything including internal details.
	VerbosityDebug Verbosity = 3
)

// Miniskin is a build-time template assembler.
// It resolves percent tags and applies skins to produce
// final files ready for embedding.
type Miniskin struct {
	contentPath     string
	modulesPath     string
	globals         map[string]string
	defaultSaveMode string // "overwrite" (default) or "append"
	currentSource  string
	currentFileDir string           // directory of the current source file
	lineMode             bool             // line-consuming mode for mockup import/export tags
	importMarkOut        *strings.Builder // buffer where import mark was set
	importMark           int              // position in buffer after imported content
	consumeUntilNewline  bool             // eat chars until \n (line mode)
	generatedFiles  []GeneratedFile
	touchedFiles    map[string]bool // files written in current Run()
	skipVars        bool            // mockup mode: don't resolve variables
	skipNegatives   bool            // skip negative template generation
	Output          io.Writer            // log destination; nil = silent
	Verbosity       Verbosity            // log detail level (default: VerbosityNormal)
	defaultEscapeFn func(string) string  // active default escape for single tags
	activeExport    string               // when set, only process this export path
	bucketSrc       string               // bucket src directory (base for absolute mockup-export paths)
	docBuffer       map[string]string    // labeled buffers populated by doc-block-begin/end
	collectingDocBlocks bool             // pre-pass: doc-block-toc/content emit "" instead of reading docBuffer
	dryRun          bool                 // pre-pass: skip writing item output files
	resourceHashes      map[string]hashEntry // cache of computed hashes for contract-less remote resources (miniskin-resource-hash.xml)
	resourceHashesDirty bool                 // cache changed → rewrite the file
}

// MiniskinNew creates a Miniskin instance.
// contentPath: root directory where source files live.
// modulesPath: root directory where Go modules live.
func MiniskinNew(contentPath, modulesPath string) *Miniskin {
	return &Miniskin{
		contentPath: absPath(contentPath),
		modulesPath: modulesPath,
		Output:      os.Stdout,
		Verbosity:   VerbosityNormal,
	}
}

// Silent disables log output.
func (ms *Miniskin) Silent() *Miniskin {
	ms.Verbosity = VerbositySilent
	ms.Output = nil
	return ms
}

// SetVerbosity sets the log detail level.
func (ms *Miniskin) SetVerbosity(v Verbosity) *Miniskin {
	ms.Verbosity = v
	if v == VerbositySilent {
		ms.Output = nil
	}
	return ms
}

// --- Result types

// BucketResult holds a parsed bucket and its collected items.
type BucketResult struct {
	Bucket Bucket
	Items  []Item      // items embedded + served normally (blob items excluded)
	Blobs  []BlobIndex // blobs packed for this bucket (from blob-name resource-lists)
}

// Result holds everything miniskin parsed and processed.
type Result struct {
	BucketList     BucketList
	Buckets        []BucketResult
	GeneratedFiles []GeneratedFile
}

// --- Init

// init parses the root miniskin.xml and prepares internal state.
// Returns the parsed root and bucket list.
func (ms *Miniskin) init() (*xmlMiniskin, BucketList, error) {
	rootFiles, err := filepath.Glob(filepath.Join(ms.contentPath, "*.miniskin.xml"))
	if err != nil {
		return nil, BucketList{}, err
	}
	if len(rootFiles) == 0 {
		return nil, BucketList{}, fmt.Errorf("no *.miniskin.xml found in %s", absPath(ms.contentPath))
	}
	if len(rootFiles) > 1 {
		return nil, BucketList{}, fmt.Errorf("multiple *.miniskin.xml in root: %v", rootFiles)
	}

	root, err := parseMiniskinXML(rootFiles[0])
	if err != nil {
		return nil, BucketList{}, err
	}
	if root.BucketList == nil {
		return nil, BucketList{}, fmt.Errorf("root %s has no bucket-list", absPath(rootFiles[0]))
	}

	ms.globals = parseGlobals(root.Globals)

	// Log output: if XML specifies log file, write to both console and file
	if root.Log != "" {
		logPath := filepath.Join(ms.contentPath, root.Log)
		f, err := os.Create(logPath)
		if err != nil {
			return nil, BucketList{}, fmt.Errorf("opening log %s: %w", absPath(logPath), err)
		}
		if ms.Output != nil {
			ms.Output = io.MultiWriter(ms.Output, f)
		} else {
			ms.Output = f
		}
	}

	skinDir := root.SkinDir
	if skinDir == "" {
		skinDir = "_skin"
	}

	// Mux cascade: root miniskin defaults to mux-include="*" (everything included)
	rootMuxInclude := root.MuxInclude
	if rootMuxInclude == "" {
		rootMuxInclude = "*"
	}

	bl := parseBucketList(root.BucketList, skinDir, rootMuxInclude, root.MuxExclude, root.Escapes)
	ms.generatedFiles = nil
	ms.touchedFiles = make(map[string]bool)

	return root, bl, nil
}

// --- ProcessMockupExport

// ProcessMockupExport processes all mockup-lists across all buckets.
// Only mockup-export side effects matter; the output is discarded.
// Variables are not resolved — only conditionals and mockup-export are processed.
func (ms *Miniskin) ProcessMockupExport() (*Result, error) {
	_, bl, err := ms.init()
	if err != nil {
		return nil, err
	}

	ms.logf("=== ProcessMockupExport ===")
	for _, bucket := range bl.Buckets {
		if err := ms.processBucketMockups(bucket); err != nil {
			return nil, err
		}
	}

	return &Result{
		BucketList:     bl,
		GeneratedFiles: ms.generatedFiles,
	}, nil
}

// --- BuildEmbed

// BuildEmbed collects and processes all resource items across all buckets.
// Resolves variables, includes, skins, and writes output files.
// Externals are resolved first so item sources that depend on them exist.
func (ms *Miniskin) BuildEmbed() (*Result, error) {
	root, bl, err := ms.init()
	if err != nil {
		return nil, err
	}

	if err := ms.loadResourceHashes(); err != nil {
		return nil, err
	}
	origins, err := ms.loadOrigins(root.Origins)
	if err != nil {
		return nil, err
	}
	if err := ms.processExternals(bl, origins); err != nil {
		return nil, err
	}

	result := &Result{BucketList: bl}

	ms.logf("=== BuildEmbed ===")
	idx := 0
	for _, bucket := range bl.Buckets {
		ms.bucketSrc = resolveSrcPath(bucket.Src, ms.contentPath, ms.contentPath)
		items, err := ms.collectItems(bucket)
		if err != nil {
			return nil, err
		}

		if err := ms.collectDocBlocks(items); err != nil {
			return nil, err
		}

		// block items ran their side effects in collectDocBlocks (populating
		// doc-block buffers); they produce no output file, so drop them before
		// writing, validating, and embedding.
		items = dropBlockItems(items)

		var blobs []BlobIndex
		items, blobs, err = ms.packBlobs(bl, items)
		if err != nil {
			return nil, err
		}

		for i := range items {
			items[i].Index = idx
			idx++
			if items[i].NeedsProcessing() {
				ms.logf("[%d] %s -> %s (src: %s)", items[i].Index, items[i].Src, items[i].File, items[i].dir)
				if err := ms.processItem(&items[i]); err != nil {
					return nil, fmt.Errorf("processing %s: %w", absPath(items[i].filePath()), err)
				}
			}
		}

		for _, item := range items {
			fp := absPath(item.filePath())
			if _, err := os.Stat(fp); err != nil {
				if item.XMLSrc != "" && item.XMLLine > 0 {
					return nil, fmt.Errorf("item %q not found at: %s\n\t(declared in %s line %d)", item.File, fp, item.XMLSrc, item.XMLLine)
				} else if item.XMLSrc != "" {
					return nil, fmt.Errorf("item %q not found at: %s\n\t(declared in %s)", item.File, fp, item.XMLSrc)
				}
				return nil, fmt.Errorf("item %q not found at: %s", item.File, fp)
			}
		}

		if err := ms.computeEmbedPaths(items); err != nil {
			return nil, err
		}
		result.Buckets = append(result.Buckets, BucketResult{
			Bucket: bucket,
			Items:  items,
			Blobs:  blobs,
		})
	}

	if err := ms.flushResourceHashes(); err != nil {
		return nil, err
	}

	result.GeneratedFiles = ms.generatedFiles

	return result, nil
}

// --- Run (convenience: export + update + build)

// Run executes the full pipeline:
//  0. Resolve <external> blocks (copy files from declared origins)
//  1. Analyze dependencies and check for circular references
//  2. Process mockup exports (writes extracted files to disk)
//  3. Update imports (refresh inline content in mockup-import blocks)
//  4. Build embed (process resource items)
func (ms *Miniskin) Run() (*Result, error) {
	root, bl, err := ms.init()
	if err != nil {
		return nil, err
	}

	// --- Step 0: externals (must run before deps so referenced files exist)
	if err := ms.loadResourceHashes(); err != nil {
		return nil, err
	}
	origins, err := ms.loadOrigins(root.Origins)
	if err != nil {
		return nil, err
	}
	if err := ms.processExternals(bl, origins); err != nil {
		return nil, err
	}

	// --- Step 1: analyze dependencies
	ms.logVerbose("=== analyzing dependencies ===")
	dm, err := ms.analyzeDepsFromBuckets(bl)
	if err != nil {
		return nil, err
	}
	if dm.HasCycles() {
		for _, cycle := range dm.Cycles {
			ms.logf("circular dependency: %s", strings.Join(cycle, " → "))
		}
		return nil, fmt.Errorf("circular dependencies detected between mockup files")
	}
	order, _ := dm.ProcessingOrder() // safe: no cycles
	if len(order) > 0 {
		ms.logVerbose("processing order: %s", strings.Join(order, ", "))
	}

	// --- Step 2: mockup exports
	ms.skipNegatives = true
	ms.logf("=== pass 1: mockup exports ===")
	for _, bucket := range bl.Buckets {
		if err := ms.processBucketMockups(bucket); err != nil {
			return nil, err
		}
	}
	ms.skipNegatives = false

	// --- Step 3: update imports
	ms.logf("=== pass 2: update imports ===")
	if err := ms.updateImportsFromBuckets(bl); err != nil {
		return nil, err
	}

	// --- Step 4: build embed
	result := &Result{BucketList: bl}
	ms.logf("=== pass 3: build embed ===")
	idx := 0
	for _, bucket := range bl.Buckets {
		ms.bucketSrc = resolveSrcPath(bucket.Src, ms.contentPath, ms.contentPath)
		items, err := ms.collectItems(bucket)
		if err != nil {
			return nil, err
		}

		if err := ms.collectDocBlocks(items); err != nil {
			return nil, err
		}

		// block items ran their side effects in collectDocBlocks (populating
		// doc-block buffers); they produce no output file, so drop them before
		// writing, validating, and embedding.
		items = dropBlockItems(items)

		var blobs []BlobIndex
		items, blobs, err = ms.packBlobs(bl, items)
		if err != nil {
			return nil, err
		}

		for i := range items {
			items[i].Index = idx
			idx++
			if items[i].NeedsProcessing() {
				ms.logf("[%d] %s -> %s (src: %s)", items[i].Index, items[i].Src, items[i].File, items[i].dir)
				if err := ms.processItem(&items[i]); err != nil {
					return nil, fmt.Errorf("processing %s: %w", absPath(items[i].filePath()), err)
				}
			}
		}

		for _, item := range items {
			fp := absPath(item.filePath())
			if _, err := os.Stat(fp); err != nil {
				if item.XMLSrc != "" && item.XMLLine > 0 {
					return nil, fmt.Errorf("item %q not found at: %s\n\t(declared in %s line %d)", item.File, fp, item.XMLSrc, item.XMLLine)
				} else if item.XMLSrc != "" {
					return nil, fmt.Errorf("item %q not found at: %s\n\t(declared in %s)", item.File, fp, item.XMLSrc)
				}
				return nil, fmt.Errorf("item %q not found at: %s", item.File, fp)
			}
		}

		if err := ms.computeEmbedPaths(items); err != nil {
			return nil, err
		}
		result.Buckets = append(result.Buckets, BucketResult{
			Bucket: bucket,
			Items:  items,
			Blobs:  blobs,
		})
	}

	if err := ms.flushResourceHashes(); err != nil {
		return nil, err
	}

	result.GeneratedFiles = ms.generatedFiles

	return result, nil
}

// analyzeDepsFromBuckets builds the dependency map without calling init() again.
func (ms *Miniskin) analyzeDepsFromBuckets(bl BucketList) (*DepMap, error) {
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
					ms.logDebug("  dep: %s -[export]-> %s", relSrc, e)
				}
				for _, imp := range imports {
					dm.Edges = append(dm.Edges, DepEdge{Source: relSrc, Target: imp, Kind: "import"})
					ms.logDebug("  dep: %s -[import]-> %s", relSrc, imp)
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

// updateImportsFromBuckets refreshes import blocks without calling init() again.
func (ms *Miniskin) updateImportsFromBuckets(bl BucketList) error {
	for _, bucket := range bl.Buckets {
		bucketSrc := resolveSrcPath(bucket.Src, ms.contentPath, ms.contentPath)
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

				_, imports := scanExportsImports(string(data))
				if len(imports) == 0 {
					continue
				}

				updated, err := refreshImports(string(data), bucketSrc, filepath.Dir(srcPath))
				if err != nil {
					return fmt.Errorf("updating %s: %w", mi.Src, err)
				}

				if updated != string(data) {
					if err := os.WriteFile(srcPath, []byte(updated), 0644); err != nil {
						return fmt.Errorf("writing %s: %w", srcPath, err)
					}
					ms.logf("  updated: %s", mi.Src)
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// processItem reads from src, parses front-matter, resolves percent tags,
// applies skin if declared, and writes the result to file.
func (ms *Miniskin) processItem(item *Item) error {
	srcPath := absPath(item.srcPath())

	// Set default escape based on item's escape rules and source file
	srcFile := item.Src
	if srcFile == "" {
		srcFile = item.File
	}
	ms.defaultEscapeFn = resolveDefaultEscape(srcFile, item.escape, item.escapeRules)
	defer func() { ms.defaultEscapeFn = nil }()

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", srcPath, err)
	}

	content := string(data)

	// Parse front-matter: extracts variables, directives, and skin declaration
	fmVars, directives, body, err := parseFrontMatter(content)
	if err != nil {
		return fmt.Errorf("front-matter in %s: %w", item.Src, err)
	}

	// Extract skin from front-matter if present
	skinName := ""
	if fmVars != nil {
		if s, ok := fmVars["skin"]; ok {
			skinName = s
			delete(fmVars, "skin")
		}
	}

	// Resolve percent tags in the body
	ms.currentSource = item.Src
	ms.importMarkOut = nil
	ms.importMark = 0
	ms.consumeUntilNewline = false
	vars := ms.mergeVars(fmVars)
	chain := []string{srcPath}
	body, err = ms.resolvePercent(body, vars, chain)
	if err != nil {
		return err
	}

	// Apply skin if declared
	if skinName != "" {
		body, err = ms.applySkin(skinName, body, fmVars, item.skinDir)
		if err != nil {
			return err
		}
	}

	// Apply directives
	if level, ok := directives["minify"]; ok {
		body, err = applyMinify(body, level, filepath.Ext(item.File))
		if err != nil {
			return fmt.Errorf("minifying %s: %w", item.Src, err)
		}
	}
	if ending, ok := directives["eol"]; ok {
		body = applyLineEnding(body, ending)
	}

	// Write to the output file (skipped during doc-block pre-pass)
	if !ms.dryRun {
		outPath := absPath(item.filePath())
		if err := os.WriteFile(outPath, []byte(body), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", outPath, err)
		}
	}

	return nil
}

// dropBlockItems removes type="block" items from the embed list. A block item is
// processed only for its side effects (populating doc-block buffers, via
// collectDocBlocks); it produces no output file, so it must not be written,
// validated for existence, or embedded.
func dropBlockItems(items []Item) []Item {
	out := items[:0]
	for _, it := range items {
		if it.HasFlag("block") {
			continue
		}
		out = append(out, it)
	}
	return out
}

// collectDocBlocks runs a dry-run pass over the bucket items to populate
// ms.docBuffer with the contents of all doc-block-begin/end blocks. During
// this pass, doc-block-toc and doc-block-content emit "" so missing-buffer
// errors don't fire before the buffers are known.
func (ms *Miniskin) collectDocBlocks(items []Item) error {
	ms.docBuffer = make(map[string]string)
	ms.collectingDocBlocks = true
	ms.dryRun = true
	defer func() {
		ms.collectingDocBlocks = false
		ms.dryRun = false
	}()
	for i := range items {
		if !items[i].NeedsProcessing() {
			continue
		}
		if err := ms.processItem(&items[i]); err != nil {
			return fmt.Errorf("doc-block pre-pass on %s: %w", absPath(items[i].filePath()), err)
		}
	}
	return nil
}

// processBucketMockups walks a bucket and processes all mockup-lists.
// Only mockup-export side effects matter; the output is discarded.
func (ms *Miniskin) processBucketMockups(bucket Bucket) error {
	ms.bucketSrc = resolveSrcPath(bucket.Src, ms.contentPath, ms.contentPath)
	return ms.walkBucket(bucket, func(parsed *xmlMiniskin, dir string, _ string) error {
		if parsed.MockupList != nil {
			return ms.processMockupList(parsed.MockupList, dir, bucket.skinDir)
		}
		return nil
	})
}

// processMockupList processes mockup items with mockup=1.
// Only the mockup-export side effects matter; the output is discarded.
func (ms *Miniskin) processMockupList(ml *xmlMockupList, dir string, defaultSkinDir string) error {
	skinDir := ml.SkinDir
	if skinDir == "" {
		skinDir = defaultSkinDir
	}
	// Line mode: default ON, explicit "off" disables
	if ml.LineMode == "off" {
		ms.lineMode = false
	} else {
		ms.lineMode = true
	}
	// Cascade save-mode: mockup-list → item → tag
	listSaveMode := ml.SaveMode

	for _, mi := range ml.Items {
		ms.logf("  mockup: %s", mi.Src)
		srcPath := absPath(filepath.Join(dir, mi.Src))
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("mockup reading %s: %w", srcPath, err)
		}

		fmVars, _, body, err := parseFrontMatter(string(data))
		if err != nil {
			return fmt.Errorf("mockup front-matter in %s: %w", mi.Src, err)
		}

		skinName := ""
		if fmVars != nil {
			if s, ok := fmVars["skin"]; ok {
				skinName = s
				delete(fmVars, "skin")
			}
		}

		// Merge: globals → mockup-list vars → item vars → front-matter vars
		vars := ms.mergeVars(nil)
		for _, v := range ml.Vars {
			vars[v.Name] = v.Value
		}
		for _, v := range mi.Vars {
			vars[v.Name] = v.Value
		}
		for k, v := range fmVars {
			vars[k] = v
		}
		vars["mockup"] = "1"

		// Set default save-mode: item overrides list
		if mi.SaveMode != "" {
			ms.defaultSaveMode = mi.SaveMode
		} else if listSaveMode != "" {
			ms.defaultSaveMode = listSaveMode
		} else {
			ms.defaultSaveMode = ""
		}

		ms.currentSource = mi.Src
		ms.currentFileDir = filepath.Dir(srcPath)
		ms.importMarkOut = nil
		ms.importMark = 0
		ms.consumeUntilNewline = false

		// Check for internal export dependencies (export A imports export B in same file)
		exportDeps := scanExportDeps(body)
		if hasInternalDeps(exportDeps) {
			// Process exports in dependency order (multiple passes)
			order := exportProcessingOrder(exportDeps)
			for _, exportPath := range order {
				ms.activeExport = exportPath
				ms.skipVars = true
				chain := []string{srcPath}
				_, err = ms.resolvePercent(body, vars, chain)
				ms.skipVars = false
				ms.activeExport = ""
				if err != nil {
					return fmt.Errorf("mockup processing %s (export %s): %w", srcPath, exportPath, err)
				}
			}
		} else {
			// No internal deps — single pass (normal)
			ms.skipVars = true
			chain := []string{srcPath}
			body, err = ms.resolvePercent(body, vars, chain)
			ms.skipVars = false
			if err != nil {
				return fmt.Errorf("mockup processing %s: %w", mi.Src, err)
			}
		}

		if skinName != "" {
			_, err = ms.applySkin(skinName, body, fmVars, skinDir)
			if err != nil {
				return fmt.Errorf("mockup skin %s: %w", mi.Src, err)
			}
		}
		// Output discarded — only mockup-export side effects matter

		// Generate negative template if requested
		if mi.Negative != "" && !ms.skipNegatives {
			negContent := transformNegative(string(data))
			negPath := absPath(filepath.Join(dir, mi.Negative))
			if err := os.WriteFile(negPath, []byte(negContent), 0644); err != nil {
				return fmt.Errorf("writing negative %s: %w", negPath, err)
			}
			ms.logf("    negative: %s (from: %s)", mi.Negative, mi.Src)
			ms.generatedFiles = append(ms.generatedFiles, GeneratedFile{
				File:   mi.Negative,
				Source: mi.Src,
			})
		}
	}
	return nil
}

// ProcessNegatives generates only negative templates across all buckets.
// No mockup-export extraction is performed.
func (ms *Miniskin) ProcessNegatives() (*Result, error) {
	_, bl, err := ms.init()
	if err != nil {
		return nil, err
	}

	ms.logf("=== ProcessNegatives ===")
	for _, bucket := range bl.Buckets {
		if err := ms.walkBucket(bucket, func(parsed *xmlMiniskin, dir string, _ string) error {
			if parsed.MockupList == nil {
				return nil
			}
			for _, mi := range parsed.MockupList.Items {
				if mi.Negative == "" {
					continue
				}
				srcPath := absPath(filepath.Join(dir, mi.Src))
				data, err := os.ReadFile(srcPath)
				if err != nil {
					return fmt.Errorf("reading %s: %w", srcPath, err)
				}
				negContent := transformNegative(string(data))
				negPath := absPath(filepath.Join(dir, mi.Negative))
				if err := os.WriteFile(negPath, []byte(negContent), 0644); err != nil {
					return fmt.Errorf("writing negative %s: %w", mi.Negative, err)
				}
				ms.logf("  negative: %s (from: %s)", mi.Negative, mi.Src)
				ms.generatedFiles = append(ms.generatedFiles, GeneratedFile{
					File:   mi.Negative,
					Source: mi.Src,
				})
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	return &Result{
		BucketList:     bl,
		GeneratedFiles: ms.generatedFiles,
	}, nil
}

func (ms *Miniskin) logf(format string, args ...any) {
	if ms.Output != nil {
		fmt.Fprintf(ms.Output, format+"\n", args...)
	}
}

func (ms *Miniskin) logVerbose(format string, args ...any) {
	if ms.Output != nil && ms.Verbosity >= VerbosityVerbose {
		fmt.Fprintf(ms.Output, format+"\n", args...)
	}
}

func (ms *Miniskin) logDebug(format string, args ...any) {
	if ms.Output != nil && ms.Verbosity >= VerbosityDebug {
		fmt.Fprintf(ms.Output, format+"\n", args...)
	}
}

// computeEmbedPaths sets EmbedPath on each item (relative to contentPath).
func (ms *Miniskin) computeEmbedPaths(items []Item) error {
	for i := range items {
		full := filepath.Join(items[i].dir, items[i].File)
		rel, err := filepath.Rel(ms.contentPath, full)
		if err != nil {
			return fmt.Errorf("embed path for %q: filepath.Rel(%q, %q): %w", items[i].File, ms.contentPath, full, err)
		}
		items[i].EmbedPath = filepath.ToSlash(rel)
	}
	return nil
}

// mergeVars creates a variable map with globals as base, overridden by local vars.
func (ms *Miniskin) mergeVars(local map[string]string) map[string]string {
	vars := make(map[string]string, len(ms.globals)+len(local))
	for k, v := range ms.globals {
		vars[k] = v
	}
	for k, v := range local {
		vars[k] = v
	}
	return vars
}
