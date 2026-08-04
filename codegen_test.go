package miniskin

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- embedVarName tests

func TestEmbedVarNameSimple(t *testing.T) {
	got := embedVarName("app/login/signin.html")
	if got != "FAppLoginSigninHtml" {
		t.Errorf("expected FAppLoginSigninHtml, got %s", got)
	}
}

func TestEmbedVarNameRootLevel(t *testing.T) {
	got := embedVarName("style.css")
	if got != "FStyleCss" {
		t.Errorf("expected FStyleCss, got %s", got)
	}
}

func TestEmbedVarNameWithDashes(t *testing.T) {
	got := embedVarName("app/my-module/my-file.min.js")
	expected := "FAppMy_moduleMy_file_minJs"
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestEmbedVarNameDeepPath(t *testing.T) {
	got := embedVarName("a/b/c/d.txt")
	if got != "FABCDTxt" {
		t.Errorf("expected FABCDTxt, got %s", got)
	}
}

// --- guessMime tests

func TestGuessMimeHTML(t *testing.T) {
	if got := guessMime("page.html"); got != "text/html; charset=utf-8" {
		t.Errorf("expected text/html, got %s", got)
	}
}

func TestGuessMimeCSS(t *testing.T) {
	if got := guessMime("style.css"); got != "text/css; charset=utf-8" {
		t.Errorf("expected text/css, got %s", got)
	}
}

func TestGuessMimeJS(t *testing.T) {
	if got := guessMime("app.js"); got != "application/javascript" {
		t.Errorf("expected application/javascript, got %s", got)
	}
}

func TestGuessMimeJSON(t *testing.T) {
	if got := guessMime("data.json"); got != "application/json" {
		t.Errorf("expected application/json, got %s", got)
	}
}

func TestGuessMimePNG(t *testing.T) {
	if got := guessMime("icon.png"); got != "image/png" {
		t.Errorf("expected image/png, got %s", got)
	}
}

func TestGuessMimeJPG(t *testing.T) {
	if got := guessMime("photo.jpg"); got != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", got)
	}
}

func TestGuessMimeJPEG(t *testing.T) {
	if got := guessMime("photo.jpeg"); got != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", got)
	}
}

func TestGuessMimeSVG(t *testing.T) {
	if got := guessMime("logo.svg"); got != "image/svg+xml" {
		t.Errorf("expected image/svg+xml, got %s", got)
	}
}

func TestGuessMimeICO(t *testing.T) {
	if got := guessMime("favicon.ico"); got != "image/x-icon" {
		t.Errorf("expected image/x-icon, got %s", got)
	}
}

func TestGuessMimeTTF(t *testing.T) {
	if got := guessMime("font.ttf"); got != "application/octet-stream" {
		t.Errorf("expected application/octet-stream, got %s", got)
	}
}

func TestGuessMimeUnknown(t *testing.T) {
	if got := guessMime("data.xyz"); got != "application/octet-stream" {
		t.Errorf("expected application/octet-stream, got %s", got)
	}
}

// --- sanitizePart / sanitizeExt

func TestSanitizePart(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", "Hello"},
		{"my-file", "My_file"},
		{"a.b", "A_b"},
		{"", ""},
	}
	for _, c := range cases {
		got := sanitizePart(c.in)
		if got != c.want {
			t.Errorf("sanitizePart(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeExt(t *testing.T) {
	cases := []struct{ in, want string }{
		{".html", "Html"},
		{".min-js", "Min_js"},
		{"", ""},
	}
	for _, c := range cases {
		got := sanitizeExt(c.in)
		if got != c.want {
			t.Errorf("sanitizeExt(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- GenerateEmbed tests

func makeCodegenTestResult() *Result {
	return &Result{
		BucketList: BucketList{
			Filename: "generated_embed.go",
			Module:   "content",
			Import:   "example.com/app/content",
			Buckets: []Bucket{
				{Src: "app", Dst: "/modules/app/gen.go", ModuleName: "app"},
			},
		},
		Buckets: []BucketResult{
			{
				Bucket: Bucket{Src: "app", Dst: "/modules/app/gen.go", ModuleName: "app"},
				Items: []Item{
					{Type: "static", File: "style.css", EmbedPath: "app/style.css", Key: "/assets/style.css"},
					{Type: "html-template,parse", File: "index.html", EmbedPath: "app/index.html", Key: "/index"},
				},
			},
		},
	}
}

func TestGenerateEmbed(t *testing.T) {
	dir := t.TempDir()
	result := makeCodegenTestResult()
	cg := CodegenNew(dir, dir)

	err := cg.GenerateEmbed(result)
	if err != nil {
		t.Fatalf("GenerateEmbed failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "generated_embed.go"))
	if err != nil {
		t.Fatalf("generated file not found: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "package content") {
		t.Error("missing package declaration")
	}
	if !strings.Contains(content, `//go:embed app/style.css`) {
		t.Error("missing embed directive for style.css")
	}
	if !strings.Contains(content, `//go:embed app/index.html`) {
		t.Error("missing embed directive for index.html")
	}
	if !strings.Contains(content, `import _ "embed"`) {
		t.Error("missing embed import")
	}
}

func TestGenerateEmbedCustomTemplate(t *testing.T) {
	dir := t.TempDir()
	result := makeCodegenTestResult()
	result.BucketList.Template = "custom_embed.tmpl"
	cg := CodegenNew(dir, dir)

	os.WriteFile(filepath.Join(dir, "custom_embed.tmpl"), []byte(`// custom
package {{.BucketList.Module}}
// items: {{range .Buckets}}{{range .Items}}{{embedPath .}} {{end}}{{end}}
`), 0644)

	err := cg.GenerateEmbed(result)
	if err != nil {
		t.Fatalf("GenerateEmbed with custom template failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "generated_embed.go"))
	content := string(data)
	if !strings.Contains(content, "// custom") {
		t.Error("custom template not used")
	}
	if !strings.Contains(content, "app/style.css") {
		t.Error("missing embed path in custom template output")
	}
}

func TestGenerateEmbedCustomTemplateNotFound(t *testing.T) {
	dir := t.TempDir()
	result := makeCodegenTestResult()
	result.BucketList.Template = "nonexistent.tmpl"
	cg := CodegenNew(dir, dir)

	err := cg.GenerateEmbed(result)
	if err == nil {
		t.Fatal("expected error for missing custom template")
	}
}

// --- GenerateBucketFile tests

func TestGenerateBucketFile(t *testing.T) {
	dir := t.TempDir()
	modDir := t.TempDir()

	result := makeCodegenTestResult()
	result.Buckets[0].Bucket.Dst = "gen.go"
	cg := CodegenNew(dir, modDir)

	br := result.Buckets[0]
	err := cg.GenerateBucketFile(result, br)
	if err != nil {
		t.Fatalf("GenerateBucketFile failed: %v", err)
	}

	dstPath := filepath.Join(modDir, "gen.go")
	data, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("generated file not found: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "package app") {
		t.Error("missing package declaration")
	}
	if !strings.Contains(content, "/assets/style.css") {
		t.Error("missing route URL for style.css")
	}
	if !strings.Contains(content, "text/css") {
		t.Error("missing MIME type for CSS")
	}
	if !strings.Contains(content, "text/html") {
		t.Error("missing MIME type for HTML")
	}
	if !strings.Contains(content, "example.com/app/content") {
		t.Error("missing import path")
	}
}

func TestGenerateBucketFileCustomTemplate(t *testing.T) {
	dir := t.TempDir()
	modDir := t.TempDir()

	result := makeCodegenTestResult()
	result.Buckets[0].Bucket.Dst = "gen.go"
	result.Buckets[0].Bucket.Template = "custom_bucket.tmpl"
	cg := CodegenNew(dir, modDir)

	os.WriteFile(filepath.Join(dir, "custom_bucket.tmpl"), []byte(`// custom bucket
package {{.Bucket.ModuleName}}
// count: {{len .Items}}
`), 0644)

	br := result.Buckets[0]
	err := cg.GenerateBucketFile(result, br)
	if err != nil {
		t.Fatalf("GenerateBucketFile with custom template failed: %v", err)
	}

	dstPath := filepath.Join(modDir, "gen.go")
	data, _ := os.ReadFile(dstPath)
	content := string(data)
	if !strings.Contains(content, "// custom bucket") {
		t.Error("custom template not used")
	}
	if !strings.Contains(content, "count: 2") {
		t.Error("expected 2 items in custom template output")
	}
}

func TestGenerateBucketFileCustomTemplateNotFound(t *testing.T) {
	dir := t.TempDir()
	modDir := t.TempDir()

	result := makeCodegenTestResult()
	result.Buckets[0].Bucket.Dst = "gen.go"
	result.Buckets[0].Bucket.Template = "nonexistent.tmpl"
	cg := CodegenNew(dir, modDir)

	br := result.Buckets[0]
	err := cg.GenerateBucketFile(result, br)
	if err == nil {
		t.Fatal("expected error for missing custom template")
	}
}

// --- GenerateAll tests

func TestCodegenGenerateAll(t *testing.T) {
	dir := t.TempDir()
	modDir := t.TempDir()

	result := makeCodegenTestResult()
	result.Buckets[0].Bucket.Dst = "gen.go"
	cg := CodegenNew(dir, modDir)

	err := cg.GenerateAll(result)
	if err != nil {
		t.Fatalf("GenerateAll failed: %v", err)
	}

	if _, err := os.ReadFile(filepath.Join(dir, "generated_embed.go")); err != nil {
		t.Errorf("embed file not created: %v", err)
	}
	dstPath := filepath.Join(modDir, "gen.go")
	if _, err := os.ReadFile(dstPath); err != nil {
		t.Errorf("bucket file not created: %v", err)
	}
}

// --- Template functions in bucket template

func TestBucketTemplateHasFlag(t *testing.T) {
	dir := t.TempDir()
	modDir := t.TempDir()

	result := makeCodegenTestResult()
	result.Buckets[0].Bucket.Dst = "gen.go"
	cg := CodegenNew(dir, modDir)

	br := result.Buckets[0]
	err := cg.GenerateBucketFile(result, br)
	if err != nil {
		t.Fatalf("GenerateBucketFile failed: %v", err)
	}

	dstPath := filepath.Join(modDir, "gen.go")
	data, _ := os.ReadFile(dstPath)
	content := string(data)

	if !strings.Contains(content, "parsedTemplates") {
		t.Error("expected parsedTemplates init block")
	}
	if !strings.Contains(content, "/index") {
		t.Error("expected /index route in parsedTemplates")
	}
}

// --- response (canned HTTP response) items in the mux template

func TestGenerateBucketFileResponseMux(t *testing.T) {
	dir := t.TempDir()
	modDir := t.TempDir()

	result := &Result{
		BucketList: BucketList{
			Filename: "generated_embed.go",
			Module:   "content",
			Import:   "example.com/app/content",
		},
		Buckets: []BucketResult{
			{
				Bucket: Bucket{Dst: "gen.go", ModuleName: "app", Template: "miniskin::mux"},
				Items: []Item{
					{Type: "static", File: "style.css", EmbedPath: "app/style.css", Key: "/assets/style.css"},
					{Type: "response", File: "redir.http", EmbedPath: "app/redir.http", Key: "/old"},
				},
			},
		},
	}

	cg := CodegenNew(dir, modDir)
	if err := cg.GenerateBucketFile(result, result.Buckets[0]); err != nil {
		t.Fatalf("GenerateBucketFile failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(modDir, "gen.go"))
	if err != nil {
		t.Fatalf("generated file not found: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, `serveResponse(mux, "/old", content.FAppRedirHttp)`) {
		t.Error("missing serveResponse registration for response item")
	}
	if !strings.Contains(content, "func serveResponse(") {
		t.Error("missing serveResponse helper")
	}
	if !strings.Contains(content, "func parseResponse(") {
		t.Error("missing parseResponse helper")
	}
	if !strings.Contains(content, `"strconv"`) || !strings.Contains(content, `"strings"`) {
		t.Error("missing strconv/strings imports needed by parseResponse")
	}
	// a response is not a template route: it must not leak into tmplHandlers/Templates
	if strings.Contains(content, `tmplHandlers["/old"]`) {
		t.Error("response item should not be registered as a template route")
	}

	// the generated file must be valid Go
	if _, err := parser.ParseFile(token.NewFileSet(), "gen.go", content, parser.AllErrors); err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, content)
	}
}

// A bucket with a blob emits a per-blob loader that uses the blob package, plus
// the package import — and the result must be valid Go.
// miniskin::mux is the legacy preset with automatic init and NO blob support:
// blobs are exclusive to miniskin::muxblob. Even when blob items are present, the
// mux template must ignore .Blobs entirely (no mskblob import, no loader) and keep
// compiling as plain embedded routes.
func TestGenerateBucketFileMuxIgnoresBlobs(t *testing.T) {
	dir := t.TempDir()
	modDir := t.TempDir()

	result := &Result{
		BucketList: BucketList{
			Filename: "generated_embed.go",
			Module:   "content",
			Import:   "example.com/app/content",
		},
		Buckets: []BucketResult{
			{
				Bucket: Bucket{Dst: "gen.go", ModuleName: "app", Template: "miniskin::mux"},
				Items: []Item{
					{Type: "static", File: "style.css", EmbedPath: "app/style.css", Key: "/assets/style.css"},
				},
				Blobs: []BlobIndex{
					{Name: "prod-img.blob", Base: "/prod-img/", ID: "947d88a9-7196-490c-87dd-9a8a1262b0ec1"},
				},
			},
		},
	}

	cg := CodegenNew(dir, modDir)
	if err := cg.GenerateBucketFile(result, result.Buckets[0]); err != nil {
		t.Fatalf("GenerateBucketFile failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(modDir, "gen.go"))
	if err != nil {
		t.Fatalf("generated file not found: %v", err)
	}
	content := string(data)

	// the embedded routes still generate normally
	if !strings.Contains(content, `serveStatic(mux, "/assets/style.css"`) {
		t.Error("missing serveStatic registration for embedded item")
	}
	// but NO blob wiring leaks into the legacy mux preset
	for _, unwanted := range []string{
		`"github.com/pablo-botella/mskblob"`,
		"LoadProdImgBlob",
		"mskblob.Load",
	} {
		if strings.Contains(content, unwanted) {
			t.Errorf("miniskin::mux must not emit blob code, found %q", unwanted)
		}
	}

	if _, err := parser.ParseFile(token.NewFileSet(), "gen.go", content, parser.AllErrors); err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, content)
	}
}

func TestValidateBlobAttach(t *testing.T) {
	for _, ok := range []string{"", "mux", "assets", "templates", "mux,assets,templates", "mux templates"} {
		if err := validateBlobAttach(ok); err != nil {
			t.Errorf("validateBlobAttach(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"mxu", "asset", "mux,asset", "foo"} {
		if err := validateBlobAttach(bad); err == nil {
			t.Errorf("validateBlobAttach(%q) = nil, want error", bad)
		}
	}
}

// The miniskin::muxblob template emits the blob-attach model (OnError,
// RegisterMskBlob by guid, serveStaticBlob, the <Base>ID const) and must be valid Go.
func TestGenerateBucketFileMuxBlob(t *testing.T) {
	dir := t.TempDir()
	modDir := t.TempDir()

	result := &Result{
		BucketList: BucketList{Filename: "generated_embed.go", Module: "content", Import: "example.com/app/content"},
		Buckets: []BucketResult{
			{
				Bucket: Bucket{Dst: "gen.go", ModuleName: "app", Template: "miniskin::muxblob"},
				Items: []Item{
					{Type: "static", File: "style.css", EmbedPath: "app/style.css", Key: "/assets/style.css"},
				},
				Blobs: []BlobIndex{
					{Name: "prod-img.blob", Base: "/prod-img/", ID: "947d88a9-7196-490c-87dd-9a8a1262b0ec1", Attach: "mux"},
				},
			},
		},
	}

	cg := CodegenNew(dir, modDir)
	if err := cg.GenerateBucketFile(result, result.Buckets[0]); err != nil {
		t.Fatalf("GenerateBucketFile failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(modDir, "gen.go"))
	if err != nil {
		t.Fatalf("generated file not found: %v", err)
	}
	content := string(data)

	for _, want := range []string{
		`"github.com/pablo-botella/mskblob"`,
		"type OnError func(err error) bool",
		"func RegisterMskBlob(mux *http.ServeMux, b *mskblob.Blob, onError OnError)",
		`case ProdImgBlobID:`,
		`ProdImgBlobID   = "947d88a9-7196-490c-87dd-9a8a1262b0ec1"`,
		`serveStaticBlob(mux, "/prod-img/"+it.URL, b, it)`,
		`serveStatic(mux, "/assets/style.css"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("missing %q in generated output", want)
		}
	}
	// the classic per-blob loader must NOT appear in this template
	if strings.Contains(content, "func LoadProdImgBlob(") {
		t.Error("muxblob must not emit the classic LoadProdImgBlob loader")
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "gen.go", content, parser.AllErrors); err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, content)
	}
}

// A blob with blob-attach="mux,assets,templates" must wire all three branches
// (independent ifs, not a single switch) over the same blob, and stay valid Go.
func TestGenerateBucketFileMuxBlobMultiAttach(t *testing.T) {
	dir := t.TempDir()
	modDir := t.TempDir()

	result := &Result{
		BucketList: BucketList{Filename: "generated_embed.go", Module: "content", Import: "example.com/app/content"},
		Buckets: []BucketResult{
			{
				Bucket: Bucket{Dst: "gen.go", ModuleName: "app", Template: "miniskin::muxblob"},
				Items: []Item{
					{Type: "static", File: "style.css", EmbedPath: "app/style.css", Key: "/assets/style.css"},
				},
				Blobs: []BlobIndex{
					{Name: "all.blob", Base: "/all/", ID: "11111111-2222-3333-4444-555555555555", Attach: "mux,assets,templates"},
				},
			},
		},
	}

	cg := CodegenNew(dir, modDir)
	if err := cg.GenerateBucketFile(result, result.Buckets[0]); err != nil {
		t.Fatalf("GenerateBucketFile failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(modDir, "gen.go"))
	if err != nil {
		t.Fatalf("generated file not found: %v", err)
	}
	content := string(data)

	// all three attach branches must be present over the same blob
	for _, want := range []string{
		`serveStaticBlob(mux, "/all/"+it.URL, b, it)`,         // mux branch
		`assets[it.Key] = Asset{Data: data, Mime: mskblob.MimeByExt(it.URL)}`, // assets branch
		`parsedTemplates[it.Key] = t`,                          // templates branch
	} {
		if !strings.Contains(content, want) {
			t.Errorf("missing %q in generated output (multi-attach branch not wired)", want)
		}
	}

	if _, err := parser.ParseFile(token.NewFileSet(), "gen.go", content, parser.AllErrors); err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, content)
	}
}

// A bucket with no response items must not emit the serveResponse/parseResponse
// helpers nor the strconv import they need.
func TestGenerateBucketFileNoResponseHelpers(t *testing.T) {
	dir := t.TempDir()
	modDir := t.TempDir()

	result := makeCodegenTestResult()
	result.Buckets[0].Bucket.Dst = "gen.go"
	result.Buckets[0].Bucket.Template = "miniskin::mux"

	cg := CodegenNew(dir, modDir)
	if err := cg.GenerateBucketFile(result, result.Buckets[0]); err != nil {
		t.Fatalf("GenerateBucketFile failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(modDir, "gen.go"))
	content := string(data)

	if strings.Contains(content, "serveResponse") {
		t.Error("serveResponse helper emitted without any response item")
	}
	if strings.Contains(content, `"strconv"`) {
		t.Error("strconv imported without any response item")
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "gen.go", content, parser.AllErrors); err != nil {
		t.Fatalf("generated code is not valid Go: %v\n%s", err, content)
	}
}
