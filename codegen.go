package miniskin

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
)

// Codegen generates Go source files with //go:embed directives
// from a [Result] produced by [Miniskin.Run] or [Miniskin.BuildEmbed].
type Codegen struct {
	contentPath string
	modulesPath string
}

// CodegenNew creates a Codegen instance.
// contentPath: root directory where content files live.
// modulesPath: root directory where Go modules live.
func CodegenNew(contentPath, modulesPath string) *Codegen {
	return &Codegen{contentPath: contentPath, modulesPath: modulesPath}
}

// MiniskinRun runs the full pipeline: mockup update + build + code generation.
// Pass VerbosityNormal for standard output, or higher levels for more detail.
func MiniskinRun(contentPath, modulesPath string, verbosity ...Verbosity) error {
	ms := MiniskinNew(contentPath, modulesPath)
	if len(verbosity) > 0 {
		ms.SetVerbosity(verbosity[0])
	}
	result, err := ms.Run()
	if err != nil {
		return err
	}
	cg := CodegenNew(contentPath, modulesPath)
	return cg.GenerateAll(result)
}

// MiniskinGenerate builds embed assets and generates Go source code.
// Does not process mockups — use [MiniskinFull] for the complete pipeline.
func MiniskinGenerate(contentPath, modulesPath string, verbosity ...Verbosity) error {
	ms := MiniskinNew(contentPath, modulesPath)
	if len(verbosity) > 0 {
		ms.SetVerbosity(verbosity[0])
	}
	result, err := ms.BuildEmbed()
	if err != nil {
		return err
	}
	cg := CodegenNew(contentPath, modulesPath)
	return cg.GenerateAll(result)
}

// MiniskinMockupUpdate processes mockup exports and refreshes imports.
// Runs dependency analysis, mockup-export extraction, and import refresh.
func MiniskinMockupUpdate(contentPath, modulesPath string, verbosity ...Verbosity) error {
	ms := MiniskinNew(contentPath, modulesPath)
	if len(verbosity) > 0 {
		ms.SetVerbosity(verbosity[0])
	}
	dm, err := ms.AnalyzeDeps()
	if err != nil {
		return err
	}
	if dm.HasCycles() {
		return fmt.Errorf("circular dependencies detected")
	}
	if _, err = ms.ProcessMockupExport(); err != nil {
		return err
	}
	return ms.UpdateImports()
}

// MiniskinMockupClean empties the inline content of mockup-import blocks.
func MiniskinMockupClean(contentPath, modulesPath string, verbosity ...Verbosity) error {
	ms := MiniskinNew(contentPath, modulesPath)
	if len(verbosity) > 0 {
		ms.SetVerbosity(verbosity[0])
	}
	return ms.CleanImports()
}

// GenerateAll writes the embed file and all bucket files.
// Outputs listed in the bucket-list "omit" attribute are skipped:
// "embed" → skip the embed file, "module" → skip per-bucket module files.
func (cg *Codegen) GenerateAll(result *Result) error {
	if !result.BucketList.OmitsCodegen("embed") {
		if err := cg.GenerateEmbed(result); err != nil {
			return err
		}
	}
	if !result.BucketList.OmitsCodegen("module") {
		for _, br := range result.Buckets {
			if err := cg.GenerateBucketFile(result, br); err != nil {
				return err
			}
		}
	}
	return nil
}

// GenerateEmbed writes the embed file (e.g. generated_embed.go) with //go:embed directives.
// Uses the custom template from bucket-list if set, otherwise the built-in default.
func (cg *Codegen) GenerateEmbed(result *Result) error {
	outPath := absPath(filepath.Join(cg.contentPath, result.BucketList.Filename))

	tmplSrc, err := resolveTemplate(result.BucketList.Template, namedEmbedTemplates, defaultEmbedTmpl, cg.contentPath)
	if err != nil {
		return err
	}

	funcMap := template.FuncMap{
		"embedPath": func(item Item) string {
			return item.EmbedPath
		},
		"embedVar": func(item Item) string {
			return embedVarName(item.EmbedPath)
		},
	}

	tmpl, err := template.New("embed").Funcs(funcMap).Parse(tmplSrc)
	if err != nil {
		return fmt.Errorf("parsing embed template: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outPath, err)
	}
	defer f.Close()

	return tmpl.Execute(f, result)
}

// GenerateBucketFile writes the generated Go file for a single bucket.
// Uses the custom template from the bucket if set, otherwise the built-in default.
func (cg *Codegen) GenerateBucketFile(result *Result, br BucketResult) error {
	tmplSrc, err := resolveTemplate(br.Bucket.Template, namedBucketTemplates, defaultBucketTmpl, cg.contentPath)
	if err != nil {
		return err
	}

	funcMap := template.FuncMap{
		"embedVar": func(item Item) string {
			return embedVarName(item.EmbedPath)
		},
		"mimeType": func(item Item) string {
			return guessMime(item.File)
		},
		"hasFlag": func(item Item, flag string) bool {
			return item.HasFlag(flag)
		},
		"embedPkg": func() string {
			return result.BucketList.Module
		},
		"embedImport": func() string {
			return result.BucketList.Import
		},
		"blobBase":  blobBaseName,
		"attachHas": attachHas,
	}

	tmpl, err := template.New("bucket").Funcs(funcMap).Parse(tmplSrc)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	data := struct {
		BucketList BucketList
		Bucket     Bucket
		Items      []Item
		Blobs      []BlobIndex
	}{
		BucketList: result.BucketList,
		Bucket:     br.Bucket,
		Items:      br.Items,
		Blobs:      br.Blobs,
	}

	projectRoot := cg.modulesPath
	if result.BucketList.ProjectRoot != "" {
		projectRoot = filepath.Join(cg.contentPath, result.BucketList.ProjectRoot)
	}
	dstPath := absPath(filepath.Join(projectRoot, filepath.FromSlash(br.Bucket.Dst)))
	f, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dstPath, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("executing template for %s: %w", dstPath, err)
	}

	return nil
}

// resolveTemplate returns the template source for a given name.
// If name is empty, returns the default. If name starts with "miniskin::",
// looks up the built-in named template. Otherwise, reads from file.
func resolveTemplate(name string, named map[string]string, fallback string, contentPath string) (string, error) {
	if name == "" {
		return fallback, nil
	}
	if strings.HasPrefix(name, "miniskin::") {
		src, ok := named[name]
		if !ok {
			return "", fmt.Errorf("unknown built-in template %q", name)
		}
		return src, nil
	}
	tmplPath := absPath(filepath.Join(contentPath, name))
	data, err := os.ReadFile(tmplPath)
	if err != nil {
		return "", fmt.Errorf("reading template %s: %w", tmplPath, err)
	}
	return string(data), nil
}

// --- helpers

// attachHas reports whether a blob-attach expression includes the given mode
// (mux/assets/templates). A blob may carry several, so the muxblob template tests
// each mode independently rather than switching on a single value.
func attachHas(attach, mode string) bool {
	return slices.Contains(splitAttach(attach), mode)
}

// blobBaseName derives the exported camel base from a blob file name, e.g.
// "prod-img.blob" -> "ProdImgBlob" (used by miniskin::muxex for <Base>ID/<Base>File).
func blobBaseName(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	parts := strings.FieldsFunc(base, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == ' '
	})
	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	sb.WriteString("Blob")
	return sb.String()
}

func embedVarName(relPath string) string {
	dir := filepath.ToSlash(filepath.Dir(relPath))
	base := filepath.Base(relPath)

	var parts []string
	for _, p := range strings.Split(dir, "/") {
		if p != "" && p != "." {
			parts = append(parts, sanitizePart(p))
		}
	}

	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	parts = append(parts, sanitizePart(name))
	parts = append(parts, sanitizeExt(ext))

	// Constant "F" prefix: guarantees every embed var starts with an uppercase
	// letter, so it is always an exported, valid Go identifier — even when the
	// path part begins with "_" (ToUpper leaves it unexported) or a digit.
	return "F" + strings.Join(parts, "")
}

func sanitizePart(s string) string {
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	if len(s) > 0 {
		s = strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

func sanitizeExt(ext string) string {
	ext = strings.TrimPrefix(ext, ".")
	ext = strings.ReplaceAll(ext, "-", "_")
	if len(ext) > 0 {
		ext = strings.ToUpper(ext[:1]) + ext[1:]
	}
	return ext
}

func guessMime(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".ttf":
		return "application/octet-stream"
	case ".ico":
		return "image/x-icon"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}
