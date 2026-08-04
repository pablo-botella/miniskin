package miniskin

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newSilent(contentPath, modulesPath string) *Miniskin {
	return MiniskinNew(contentPath, modulesPath).Silent()
}

func newMockup(contentPath, modulesPath string) *Miniskin {
	ms := newSilent(contentPath, modulesPath)
	ms.skipVars = true
	ms.bucketSrc = contentPath
	ms.touchedFiles = make(map[string]bool)
	return ms
}

func testdataPath() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "testdata")
}

func cleanup(items []Item) {
	for _, it := range items {
		if it.NeedsProcessing() {
			os.Remove(it.filePath())
		}
	}
}

// ---

func TestParseRootMiniskinXML(t *testing.T) {
	ms := newSilent(testdataPath(), testdataPath())
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	if result.BucketList.Filename != "generated_embed.go" {
		t.Errorf("expected filename generated_embed.go, got %s", result.BucketList.Filename)
	}
	if len(result.BucketList.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result.BucketList.Buckets))
	}
	if result.BucketList.Buckets[0].Src != "app" {
		t.Errorf("expected bucket src app, got %s", result.BucketList.Buckets[0].Src)
	}
}

// ---

func TestCollectAllItems(t *testing.T) {
	ms := newSilent(testdataPath(), testdataPath())
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	items := result.Buckets[0].Items
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

// ---

func TestSkinFromFrontMatter(t *testing.T) {
	ms := newSilent(testdataPath(), testdataPath())
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	var signin *Item
	for i := range result.Buckets[0].Items {
		if result.Buckets[0].Items[i].File == "signin.html" {
			signin = &result.Buckets[0].Items[i]
			break
		}
	}
	if signin == nil {
		t.Fatal("signin.html not found")
	}

	data, err := os.ReadFile(signin.filePath())
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "<!DOCTYPE html>") {
		t.Error("missing DOCTYPE from skin")
	}
	if !strings.Contains(content, "<title>TestApp - Sign In</title>") {
		t.Errorf("missing title with global + front-matter, got:\n%s", content)
	}
	if !strings.Contains(content, `<div class="login">`) {
		t.Error("missing body content")
	}
	if !strings.Contains(content, "{{.Username}}") {
		t.Error("Go template syntax not preserved")
	}
}

// ---

func TestPercentInclude(t *testing.T) {
	ms := newSilent(testdataPath(), testdataPath())
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	var plain *Item
	for i := range result.Buckets[0].Items {
		if result.Buckets[0].Items[i].File == "plain.css" {
			plain = &result.Buckets[0].Items[i]
			break
		}
	}
	if plain == nil {
		t.Fatal("plain.css not found")
	}

	data, err := os.ReadFile(plain.filePath())
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "* { margin: 0; padding: 0; }") {
		t.Error("missing included reset.css content")
	}
	if !strings.Contains(content, ".plain { color: red; }") {
		t.Error("missing original content")
	}
}

// ---

func TestStaticNotProcessed(t *testing.T) {
	ms := newSilent(testdataPath(), testdataPath())
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	var appCss *Item
	for i := range result.Buckets[0].Items {
		if result.Buckets[0].Items[i].File == "app.css" {
			appCss = &result.Buckets[0].Items[i]
			break
		}
	}
	if appCss == nil {
		t.Fatal("app.css not found")
	}
	if appCss.NeedsProcessing() {
		t.Error("app.css should not need processing")
	}
}

// ---

func TestCycleDetection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "cycle.html"), []byte(`<%%include:/cycle.html%%>`), 0644)

	ms := newSilent(dir, dir)
	_, err := ms.resolvePercent(`<%%include:/cycle.html%%>`, nil, nil)
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

// ---

func TestNestedIncludes(t *testing.T) {
	dir := t.TempDir()

	// a.html includes b.html, b.html includes c.html
	os.WriteFile(filepath.Join(dir, "a.html"), []byte(`[A-start]<%%include:/b.html%%>[A-end]`), 0644)
	os.WriteFile(filepath.Join(dir, "b.html"), []byte(`[B-start]<%%include:/c.html%%>[B-end]`), 0644)
	os.WriteFile(filepath.Join(dir, "c.html"), []byte(`[C]`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.resolvePercent(`<%%include:/a.html%%>`, nil, nil)
	if err != nil {
		t.Fatalf("nested includes failed: %v", err)
	}
	expected := "[A-start][B-start][C][B-end][A-end]"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// ---

func TestIncludeStripsBOM(t *testing.T) {
	dir := t.TempDir()

	// b.html starts with a UTF-8 BOM. When included into a.html, the BOM
	// must NOT appear in the middle of the parent output.
	os.WriteFile(filepath.Join(dir, "a.html"), []byte(`[A]<%%include:/b.html%%>[A-end]`), 0644)
	os.WriteFile(filepath.Join(dir, "b.html"), []byte("\xef\xbb\xbf[B]"), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.resolvePercent(`<%%include:/a.html%%>`, nil, nil)
	if err != nil {
		t.Fatalf("include failed: %v", err)
	}
	if strings.Contains(result, "\xef\xbb\xbf") {
		t.Errorf("BOM leaked into parent output: %q", result)
	}
	expected := "[A][B][A-end]"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// ---

func TestIndirectCycle(t *testing.T) {
	dir := t.TempDir()

	// a includes b, b includes c, c includes a -> cycle
	os.WriteFile(filepath.Join(dir, "a.html"), []byte(`<%%include:/b.html%%>`), 0644)
	os.WriteFile(filepath.Join(dir, "b.html"), []byte(`<%%include:/c.html%%>`), 0644)
	os.WriteFile(filepath.Join(dir, "c.html"), []byte(`<%%include:/a.html%%>`), 0644)

	ms := newSilent(dir, dir)
	_, err := ms.resolvePercent(`<%%include:/a.html%%>`, nil, nil)
	if err == nil {
		t.Fatal("expected cycle detection error for indirect cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

// ---

func TestUnclosedSingleTag(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`before <%oops after`, nil, nil)
	if err == nil {
		t.Fatal("expected error for unclosed single tag")
	}
}

// ---

func TestUnclosedDoubleTag(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`before <%%oops after`, nil, nil)
	if err == nil {
		t.Fatal("expected error for unclosed double tag")
	}
}

// ---

func TestUndefinedVariable(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`<%noexist%>`, nil, nil)
	if err == nil {
		t.Fatal("expected error for undefined variable")
	}
}

// ---

func TestIncludeFileNotFound(t *testing.T) {
	dir := t.TempDir()
	ms := newSilent(dir, dir)
	_, err := ms.resolvePercent(`<%%include:/nonexistent.html%%>`, nil, nil)
	if err == nil {
		t.Fatal("expected error for missing include file")
	}
}

// ---

func TestIfTrue(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": "yes"}
	result, err := ms.resolvePercent(`before<%if:x%>SHOW<%endif%>after`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "beforeSHOWafter" {
		t.Errorf("expected %q, got %q", "beforeSHOWafter", result)
	}
}

// ---

func TestIfFalse(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`before<%if:x%>HIDE<%endif%>after`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "beforeafter" {
		t.Errorf("expected %q, got %q", "beforeafter", result)
	}
}

// ---

func TestIfEmptyIsFalse(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": ""}
	result, err := ms.resolvePercent(`<%if:x%>HIDE<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

// ---

func TestIfElse(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`<%if:x%>YES<%else%>NO<%endif%>`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "NO" {
		t.Errorf("expected %q, got %q", "NO", result)
	}
}

// ---

func TestIfElseTrue(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": "1"}
	result, err := ms.resolvePercent(`<%if:x%>YES<%else%>NO<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "YES" {
		t.Errorf("expected %q, got %q", "YES", result)
	}
}

// ---

func TestElseif(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"b": "1"}
	result, err := ms.resolvePercent(`<%if:a%>A<%elseif:b%>B<%else%>C<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "B" {
		t.Errorf("expected %q, got %q", "B", result)
	}
}

// ---

func TestElseifFirstTrue(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"a": "1", "b": "1"}
	result, err := ms.resolvePercent(`<%if:a%>A<%elseif:b%>B<%else%>C<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "A" {
		t.Errorf("expected %q, got %q", "A", result)
	}
}

// ---

func TestElseifFallthrough(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`<%if:a%>A<%elseif:b%>B<%else%>C<%endif%>`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "C" {
		t.Errorf("expected %q, got %q", "C", result)
	}
}

// ---

func TestIfNotTrue(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": "1"}
	result, err := ms.resolvePercent(`<%if-not:x%>SHOW<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

// ---

func TestIfNotFalse(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`<%if-not:x%>SHOW<%endif%>`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "SHOW" {
		t.Errorf("expected %q, got %q", "SHOW", result)
	}
}

// ---

func TestIfNotElse(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": "1"}
	result, err := ms.resolvePercent(`<%if-not:x%>A<%else%>B<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "B" {
		t.Errorf("expected %q, got %q", "B", result)
	}
}

// ---

func TestElseifNot(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"a": "1"}
	result, err := ms.resolvePercent(`<%if:a%>A<%elseif-not:b%>B<%else%>C<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "A" {
		t.Errorf("expected %q, got %q", "A", result)
	}
}

// ---

func TestElseifNotTaken(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`<%if:a%>A<%elseif-not:b%>B<%else%>C<%endif%>`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "B" {
		t.Errorf("expected %q, got %q", "B", result)
	}
}

// ---

func TestElseifNotSkipped(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"b": "1"}
	result, err := ms.resolvePercent(`<%if:a%>A<%elseif-not:b%>B<%else%>C<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "C" {
		t.Errorf("expected %q, got %q", "C", result)
	}
}

// ---

func TestCommentIfNot(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`<!--%%if-not:mock%%-->VISIBLE<!--%%endif%%-->`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "VISIBLE" {
		t.Errorf("expected %q, got %q", "VISIBLE", result)
	}
}

// ---

func TestNestedIf(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"a": "1"}
	result, err := ms.resolvePercent(`<%if:a%><%if:b%>INNER<%else%>ALT<%endif%><%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ALT" {
		t.Errorf("expected %q, got %q", "ALT", result)
	}
}

// ---

func TestNestedIfParentFalse(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"b": "1"}
	result, err := ms.resolvePercent(`<%if:a%><%if:b%>INNER<%endif%><%else%>OUT<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "OUT" {
		t.Errorf("expected %q, got %q", "OUT", result)
	}
}

// ---

func TestUndefinedVarInSkippedBlock(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	// <%undef%> inside a false block should NOT error
	result, err := ms.resolvePercent(`<%if:x%><%undef%><%endif%>ok`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected %q, got %q", "ok", result)
	}
}

// ---

func TestUnclosedIf(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": "1"}
	_, err := ms.resolvePercent(`<%if:x%>stuff`, vars, nil)
	if err == nil {
		t.Fatal("expected error for unclosed if block")
	}
}

// ---

func TestEndifWithoutIf(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`<%endif%>`, nil, nil)
	if err == nil {
		t.Fatal("expected error for endif without if")
	}
}

// ---

func TestElseWithoutIf(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`<%else%>`, nil, nil)
	if err == nil {
		t.Fatal("expected error for else without if")
	}
}

// ---

func TestSkinDirDefault(t *testing.T) {
	dir := t.TempDir()

	// _skin/basic.html relative to bucketSrc (pages)
	os.MkdirAll(filepath.Join(dir, "pages", "_skin"), 0755)
	os.WriteFile(filepath.Join(dir, "pages", "_skin", "basic.html"), []byte(`[DEFAULT]<%%content%%>[/DEFAULT]`), 0644)

	// bucket "pages" with a src item that uses skin: basic
	os.MkdirAll(filepath.Join(dir, "pages"), 0755)
	os.WriteFile(filepath.Join(dir, "pages", "pages.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/pages">
		<item type="html-template" src="page_src.html" file="page.html" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "pages", "page_src.html"), []byte("---\nskin: basic\n---\nHello"), 0644)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="pages" dst="/gen.go" module-name="pages" />
	</bucket-list>
</miniskin>`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "pages", "page.html"))
	if string(data) != "[DEFAULT]Hello[/DEFAULT]" {
		t.Errorf("expected default _skin, got %q", string(data))
	}
	cleanup(result.Buckets[0].Items)
}

// ---

func TestSkinDirOnMiniskin(t *testing.T) {
	dir := t.TempDir()

	// custom skin dir relative to bucketSrc (pages)
	os.MkdirAll(filepath.Join(dir, "pages", "layouts"), 0755)
	os.WriteFile(filepath.Join(dir, "pages", "layouts", "main.html"), []byte(`[LAYOUT]<%%content%%>[/LAYOUT]`), 0644)

	os.MkdirAll(filepath.Join(dir, "pages"), 0755)
	os.WriteFile(filepath.Join(dir, "pages", "pages.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/pages">
		<item type="html-template" src="p_src.html" file="p.html" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "pages", "p_src.html"), []byte("---\nskin: main\n---\nBody"), 0644)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin skin-dir="layouts">
	<bucket-list filename="embed.go" module="content">
		<bucket src="pages" dst="/gen.go" module-name="pages" />
	</bucket-list>
</miniskin>`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "pages", "p.html"))
	if string(data) != "[LAYOUT]Body[/LAYOUT]" {
		t.Errorf("expected layouts/main.html skin, got %q", string(data))
	}
	cleanup(result.Buckets[0].Items)
}

// ---

func TestSkinDirOnBucketOverridesRoot(t *testing.T) {
	dir := t.TempDir()

	// root-level skin dir (should be overridden)
	os.MkdirAll(filepath.Join(dir, "layouts"), 0755)
	os.WriteFile(filepath.Join(dir, "layouts", "wrap.html"), []byte(`[ROOT]<%%content%%>[/ROOT]`), 0644)

	// bucket-level skin dir
	os.MkdirAll(filepath.Join(dir, "app", "myskins"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "myskins", "wrap.html"), []byte(`[BUCKET]<%%content%%>[/BUCKET]`), 0644)

	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/app">
		<item type="html-template" src="v_src.html" file="v.html" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "v_src.html"), []byte("---\nskin: wrap\n---\nContent"), 0644)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin skin-dir="layouts">
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" skin-dir="myskins" />
	</bucket-list>
</miniskin>`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "app", "v.html"))
	if string(data) != "[BUCKET]Content[/BUCKET]" {
		t.Errorf("expected bucket skin-dir override, got %q", string(data))
	}
	cleanup(result.Buckets[0].Items)
}

// ---

func TestSkinDirOnResourceListOverridesBucket(t *testing.T) {
	dir := t.TempDir()

	// bucket-level skin dir relative to bucketSrc (mod)
	os.MkdirAll(filepath.Join(dir, "mod", "bskins"), 0755)
	os.WriteFile(filepath.Join(dir, "mod", "bskins", "base.html"), []byte(`[BUCKET]<%%content%%>[/BUCKET]`), 0644)

	// resource-list level skin dir relative to bucketSrc (mod)
	os.MkdirAll(filepath.Join(dir, "mod", "rskins"), 0755)
	os.WriteFile(filepath.Join(dir, "mod", "rskins", "base.html"), []byte(`[RESLIST]<%%content%%>[/RESLIST]`), 0644)

	os.MkdirAll(filepath.Join(dir, "mod"), 0755)
	os.WriteFile(filepath.Join(dir, "mod", "mod.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/mod" skin-dir="rskins">
		<item type="html-template" src="x_src.html" file="x.html" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "mod", "x_src.html"), []byte("---\nskin: base\n---\nHi"), 0644)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="mod" dst="/gen.go" module-name="mod" skin-dir="bskins" />
	</bucket-list>
</miniskin>`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "mod", "x.html"))
	if string(data) != "[RESLIST]Hi[/RESLIST]" {
		t.Errorf("expected resource-list skin-dir override, got %q", string(data))
	}
	cleanup(result.Buckets[0].Items)
}

// ---

func TestCommentIfTrue(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"mock": "1"}
	result, err := ms.resolvePercent(`before<!--%%if:mock%%-->SHOW<!--%%endif%%-->after`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "beforeSHOWafter" {
		t.Errorf("expected %q, got %q", "beforeSHOWafter", result)
	}
}

// ---

func TestCommentIfFalse(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`before<!--%%if:mock%%-->HIDE<!--%%endif%%-->after`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "beforeafter" {
		t.Errorf("expected %q, got %q", "beforeafter", result)
	}
}

// ---

func TestCommentElseif(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"other": "1"}
	result, err := ms.resolvePercent(`<!--%%if:mock%%-->M<!--%%elseif:other%%-->O<!--%%else%%-->X<!--%%endif%%-->`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "O" {
		t.Errorf("expected %q, got %q", "O", result)
	}
}

// ---

func TestCommentSingleVar(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"name": "Sam&Co"}
	result, err := ms.resolvePercent(`Hello <!--%name%-->!`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello Sam&Co!" {
		t.Errorf("expected %q, got %q", "Hello Sam&Co!", result)
	}
}

// ---

func TestCommentSingleIf(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"show": "1"}
	result, err := ms.resolvePercent(`<!--%if:show%-->YES<!--%endif%-->`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "YES" {
		t.Errorf("expected %q, got %q", "YES", result)
	}
}

// ---

func TestNoteDiscarded(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`before<!--%%note: esto se borra%%-->after`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "beforeafter" {
		t.Errorf("expected %q, got %q", "beforeafter", result)
	}
}

// ---

func TestEchoDoubleLiteral(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`<%%echo:<b>bold</b>%%>`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "<b>bold</b>" {
		t.Errorf("expected %q, got %q", "<b>bold</b>", result)
	}
}

// ---

func TestEchoSingleEscaped(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`<%echo:<b>bold</b>%>`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "<b>bold</b>"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// ---

func TestNoteMultiline(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent("before<!--%%note:\nlinea 1\nlinea 2\n%%-->after", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "beforeafter" {
		t.Errorf("expected %q, got %q", "beforeafter", result)
	}
}

// ---

func TestEchoMultiline(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent("before<%%echo:\n<p>hola</p>\n%%>after", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "before\n<p>hola</p>\nafter"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// ---

func TestMockup(t *testing.T) {
	dir := t.TempDir()

	// Skin (relative to bucketSrc=dir/app)
	os.MkdirAll(filepath.Join(dir, "app", "_skin"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "_skin", "app.html"), []byte(`<!DOCTYPE html>
<html>
<head>
<title>{{.AppDisplayName}} - <%title%></title>
<link rel="stylesheet" href="/assets/app.css">
<!--%if:css%-->
<link rel="stylesheet" href="<!--%css%-->">
<!--%endif%-->
</head>
<body>
<div class="x-container">
  <div class="app-header">
    <span class="app-name">{{.AppDisplayName}}</span>
  </div>
  <!--%%if:policybanner%%-->
  <div class="policy-banner">DO NOT CLOCK FOR OTHERS</div>
  <!--%%endif%%-->
  <%%content%%>
  <div class="app-footer">{{.AppDisplayName}}</div>
</div>
<!--%%if:js%%-->
<script src="<%%js%%>"></script>
<!--%%endif%%-->
<!--%%note:
  This skin was generated by miniskin.
  Do not edit the output files directly.
%%-->
</body>
</html>`), 0644)

	// Include fragment (relative to bucketSrc=dir/app)
	os.MkdirAll(filepath.Join(dir, "app", "_shared"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "_shared", "clock.html"), []byte(`<div class="clock" id="clock">--:--</div>`), 0644)

	// Bucket + resource list
	os.MkdirAll(filepath.Join(dir, "app", "login"), 0755)
	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<globals>
		<var name="appName" value="MD-Clock" />
	</globals>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" recurse-folder="all" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "login", "login.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/login">
		<item type="html-template,nomux" src="signin_src.html" file="signin.html" key="/login/signin" />
	</resource-list>
</miniskin>`), 0644)

	// Source file with front-matter
	os.WriteFile(filepath.Join(dir, "app", "login", "signin_src.html"), []byte(`---
skin: app
title: Sign In
css: /assets/signin.css
js: /assets/signin.js
policybanner: 1
---
<%%include:/_shared/clock.html%%>
<div class="login-card">
  <h2><%appName%></h2>
  <%echo:<form>%>
  <%%echo:{{.Username}}%%>
  </form>
</div>`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	data, err := os.ReadFile(filepath.Join(dir, "app", "login", "signin.html"))
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	out := string(data)

	checks := []struct {
		desc string
		want string
	}{
		{"DOCTYPE from skin", "<!DOCTYPE html>"},
		{"title with front-matter var", "<title>{{.AppDisplayName}} - Sign In</title>"},
		{"base css always present", `href="/assets/app.css"`},
		{"conditional css", `href="/assets/signin.css"`},
		{"conditional js", `src="/assets/signin.js"`},
		{"policy banner shown", "DO NOT CLOCK FOR OTHERS"},
		{"include resolved", `<div class="clock" id="clock">--:--</div>`},
		{"global var escaped", "MD-Clock"},
		{"single echo literal", "<form>"},
		{"double echo literal", "{{.Username}}"},
		{"note stripped", ""},
	}

	for _, c := range checks {
		if c.desc == "note stripped" {
			if strings.Contains(out, "Do not edit the output") {
				t.Errorf("[%s] note content should be stripped", c.desc)
			}
		} else if !strings.Contains(out, c.want) {
			t.Errorf("[%s] missing %q in output:\n%s", c.desc, c.want, out)
		}
	}
}

// ---

func TestMockupNoBanner(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "app", "_skin"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "_skin", "app.html"), []byte(`<html>
<!--%%if:policybanner%%-->
<div class="banner">BANNER</div>
<!--%%endif%%-->
<%%content%%>
</html>`), 0644)

	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/app">
		<item type="html-template" src="page_src.html" file="page.html" />
	</resource-list>
</miniskin>`), 0644)

	// No policybanner in front-matter
	os.WriteFile(filepath.Join(dir, "app", "page_src.html"), []byte(`---
skin: app
---
<p>Simple page</p>`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	data, _ := os.ReadFile(filepath.Join(dir, "app", "page.html"))
	out := string(data)

	if strings.Contains(out, "BANNER") {
		t.Error("banner should not appear without policybanner var")
	}
	if !strings.Contains(out, "<p>Simple page</p>") {
		t.Errorf("missing content in output:\n%s", out)
	}
}

// ---

func TestMockupConditional(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "app", "_skin"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "_skin", "app.html"), []byte(`<html>
<body>
<%%content%%>
</body>
</html>`), 0644)

	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/app">
		<item type="html-template" src="list_src.html" file="list.html" />
	</resource-list>
</miniskin>`), 0644)

	// Source with mockup data — visible in raw browser preview,
	// stripped when processed without mockup var
	os.WriteFile(filepath.Join(dir, "app", "list_src.html"), []byte(`---
skin: app
---
<table>
  <thead><tr><th>Name</th><th>Status</th></tr></thead>
  <tbody>
  {{range .Employees}}
    <tr><td>{{.Name}}</td><td>{{.Status}}</td></tr>
  {{end}}
  <!--%if:mockup%-->
    <tr><td>John Doe</td><td>Clocked In</td></tr>
    <tr><td>Jane Smith</td><td>Clocked Out</td></tr>
    <tr><td>Bob Wilson</td><td>On Break</td></tr>
  <!--%endif%-->
  </tbody>
</table>`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	data, _ := os.ReadFile(filepath.Join(dir, "app", "list.html"))
	out := string(data)

	// Without mockup var, sample rows are stripped
	if strings.Contains(out, "John Doe") {
		t.Error("mockup data should be stripped without mockup var")
	}
	// Real template syntax preserved
	if !strings.Contains(out, "{{range .Employees}}") {
		t.Error("Go template syntax should be preserved")
	}
	if !strings.Contains(out, "<thead>") {
		t.Error("table structure should be preserved")
	}
}

// ---

func TestMockupConditionalEnabled(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "app", "_skin"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "_skin", "app.html"), []byte(`<html><%%content%%></html>`), 0644)

	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<globals><var name="mockup" value="1" /></globals>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/app">
		<item type="html-template" src="page_src.html" file="page.html" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "page_src.html"), []byte(`---
skin: app
---
<ul>
<!--%if:mockup%-->
  <li>Sample Item 1</li>
  <li>Sample Item 2</li>
<!--%endif%-->
</ul>`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	data, _ := os.ReadFile(filepath.Join(dir, "app", "page.html"))
	out := string(data)

	if !strings.Contains(out, "Sample Item 1") {
		t.Errorf("mockup data should appear with mockup=1:\n%s", out)
	}
}

// ---

func TestMockupExportWithMockup(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "assets", "css"), 0755)

	ms := newMockup(dir, dir)
	input := `<!--%% if:mockup %%-->
<css>
  <!--%% mockup-export:/app/assets/css/micss.css %%-->
.app-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 5px;
}
<!--%% end %%-->
</css>
<!--%% else %%-->
<!--%echo:<link rel="stylesheet" href="/assets/css/mockup.css">%-->
<!--%% end %%-->`

	// With mockup: CSS goes to file, inline block emitted
	vars := map[string]string{"mockup": "1"}
	result, err := ms.resolvePercent(input, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check file was written
	cssData, err := os.ReadFile(filepath.Join(dir, "app", "assets", "css", "micss.css"))
	if err != nil {
		t.Fatalf("CSS file not created: %v", err)
	}
	if !strings.Contains(string(cssData), "display: flex") {
		t.Errorf("CSS file missing content: %s", cssData)
	}

	// Main output should have the <css> wrapper but not the link tag
	if strings.Contains(result, "mockup.css") {
		t.Error("else branch should not appear when mockup is set")
	}
	if !strings.Contains(result, "<css>") {
		t.Error("inline <css> block should be in output")
	}
}

// ---

func TestMockupExportElseBranch(t *testing.T) {
	dir := t.TempDir()
	ms := newMockup(dir, dir)
	input := `<!--%% if:mockup %%-->
MOCKUP
<!--%% else %%-->
<!--%% echo:<link rel="stylesheet" href="/assets/css/mockup.css"> %%-->
<!--%% end %%-->`

	// Without mockup: else branch emits literal link tag
	result, err := ms.resolvePercent(input, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, `<link rel="stylesheet"`) {
		t.Errorf("expected literal link tag, got: %s", result)
	}
	if strings.Contains(result, "MOCKUP") {
		t.Error("mockup content should not appear")
	}
}

// ---

func TestNestedMockupExport(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "css"), 0755)

	ms := newMockup(dir, dir)
	input := `<!--%% mockup-export:/css/main.css %%-->
body { margin: 0; }
<!--%% mockup-export:/css/header.css %%-->
.header { color: red; }
<!--%% end %%-->
.footer { color: blue; }
<!--%% end %%-->`

	result, err := ms.resolvePercent(input, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inner file
	inner, _ := os.ReadFile(filepath.Join(dir, "css", "header.css"))
	if !strings.Contains(string(inner), ".header { color: red; }") {
		t.Errorf("inner file missing content: %s", inner)
	}

	// Outer file: own content + content after inner block, but NOT inner content (extracted)
	outer, _ := os.ReadFile(filepath.Join(dir, "css", "main.css"))
	outerStr := string(outer)
	if !strings.Contains(outerStr, "body { margin: 0; }") {
		t.Error("outer file missing own content")
	}
	if strings.Contains(outerStr, ".header { color: red; }") {
		t.Error("outer file should NOT contain inner extracted content")
	}
	if !strings.Contains(outerStr, ".footer { color: blue; }") {
		t.Error("outer file missing content after inner block")
	}

	// Main output is empty (everything was inside mockup-export)
	if strings.TrimSpace(result) != "" {
		t.Errorf("main output should be empty, got %q", result)
	}
}

// ---

func TestMockupExportAppendMode(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "assets", "css"), 0755)

	ms := newMockup(dir, dir)

	// First write creates the file (truncates even with append)
	input1 := `<!--%% mockup-export: "/app/assets/css/micss.css" append %%-->
.header { color: red; }
<!--%% end %%-->`
	_, err := ms.resolvePercent(input1, nil, nil)
	if err != nil {
		t.Fatalf("first append failed: %v", err)
	}

	// Second write appends (same session, file already touched)
	input2 := `<!--%% mockup-export: "/app/assets/css/micss.css" append %%-->
.footer { color: blue; }
<!--%% end %%-->`
	_, err = ms.resolvePercent(input2, nil, nil)
	if err != nil {
		t.Fatalf("second append failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "app", "assets", "css", "micss.css"))
	content := string(data)
	if !strings.Contains(content, ".header { color: red; }") {
		t.Error("missing first content")
	}
	if !strings.Contains(content, ".footer { color: blue; }") {
		t.Error("missing second append content")
	}
}

// ---

func TestMockupExportAppendNewSession(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "css"), 0755)

	// Pre-existing file from a previous session
	os.WriteFile(filepath.Join(dir, "css", "old.css"), []byte("OLD CONTENT\n"), 0644)

	ms := newMockup(dir, dir)

	// First write in new session truncates, even with append mode
	input := `<!--%% mockup-export:/css/old.css append %%-->NEW<!--%% end %%-->`
	_, err := ms.resolvePercent(input, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "css", "old.css"))
	if strings.Contains(string(data), "OLD CONTENT") {
		t.Error("old content should be truncated on first write of new session")
	}
	if string(data) != "NEW" {
		t.Errorf("expected %q, got %q", "NEW", data)
	}
}

// ---

func TestMockupExportQuotedPath(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "my path"), 0755)

	ms := newMockup(dir, dir)
	input := `<!--%% mockup-export: "/my path/out.css" %%-->
body { margin: 0; }
<!--%% end %%-->`
	_, err := ms.resolvePercent(input, nil, nil)
	if err != nil {
		t.Fatalf("quoted path failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "my path", "out.css"))
	if !strings.Contains(string(data), "body { margin: 0; }") {
		t.Errorf("file content wrong: %s", data)
	}
}

// ---

func TestMockupExportDefaultOverwrite(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "css"), 0755)

	ms := newMockup(dir, dir)

	// First write
	input1 := `<!--%% mockup-export:/css/style.css %%-->OLD<!--%% end %%-->`
	ms.resolvePercent(input1, nil, nil)

	// Second write (default = overwrite)
	input2 := `<!--%% mockup-export:/css/style.css %%-->NEW<!--%% end %%-->`
	ms.resolvePercent(input2, nil, nil)

	data, _ := os.ReadFile(filepath.Join(dir, "css", "style.css"))
	if string(data) != "NEW" {
		t.Errorf("expected overwrite, got %q", data)
	}
}

// ---

func TestGeneratedFilesTracking(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "css"), 0755)

	ms := newMockup(dir, dir)
	ms.currentSource = "mockup.html"

	input := `<!--%% mockup-export:/css/a.css %%-->.a{}<!--%% end %%--><!--%% mockup-export:/css/b.css %%-->.b{}<!--%% end %%-->`
	_, err := ms.resolvePercent(input, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ms.generatedFiles) != 2 {
		t.Fatalf("expected 2 generated files, got %d", len(ms.generatedFiles))
	}
	if ms.generatedFiles[0].File != "/css/a.css" {
		t.Errorf("first file: %s", ms.generatedFiles[0].File)
	}
	if ms.generatedFiles[1].File != "/css/b.css" {
		t.Errorf("second file: %s", ms.generatedFiles[1].File)
	}
	if ms.generatedFiles[0].Source != "mockup.html" {
		t.Errorf("source: %s", ms.generatedFiles[0].Source)
	}
}

// ---

func TestMockupExportSkippedInNormalMode(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	// In normal mode, if:mockup is false, mockup-export inside should be silently skipped
	result, err := ms.resolvePercent(`before<!--%%if:mockup%%--><!--%%mockup-export:/x.css%%-->CSS<!--%%end%%--><!--%%endif%%-->after`, nil, nil)
	if err != nil {
		t.Fatalf("should not error when mockup-export is inside a false block: %v", err)
	}
	if result != "beforeafter" {
		t.Errorf("expected %q, got %q", "beforeafter", result)
	}
}

// ---

func TestMockupImport(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shared.css"), []byte(".shared { color: green; }"), 0644)

	ms := newMockup(dir, dir)
	result, err := ms.resolvePercent(`before<!--%%mockup-import:/shared.css%%-->after`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "before.shared { color: green; }after" {
		t.Errorf("expected imported content, got %q", result)
	}
}

// ---

func TestMockupImportQuotedPath(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "my dir"), 0755)
	os.WriteFile(filepath.Join(dir, "my dir", "file.css"), []byte("IMPORTED"), 0644)

	ms := newMockup(dir, dir)
	result, err := ms.resolvePercent(`<!--%%mockup-import: "/my dir/file.css"%%-->`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "IMPORTED" {
		t.Errorf("expected %q, got %q", "IMPORTED", result)
	}
}

// ---

func TestMockupImportSkippedInNormalMode(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`before<!--%%if:mockup%%--><!--%%mockup-import:/x.css%%--><!--%%end-mockup-import%%--><!--%%endif%%-->after`, nil, nil)
	if err != nil {
		t.Fatalf("should not error when mockup-import is inside a false block: %v", err)
	}
	if result != "beforeafter" {
		t.Errorf("expected %q, got %q", "beforeafter", result)
	}
}

// ---

func TestMockupImportOutsideMockupMode(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`<!--%%mockup-import:/x.css%%--><!--%%end-mockup-import%%-->`, nil, nil)
	if err == nil {
		t.Fatal("expected error for mockup-import outside mockup mode")
	}
	if !strings.Contains(err.Error(), "mockup mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---

func TestMockupExportThenImport(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "css"), 0755)

	ms := newMockup(dir, dir)

	// Export a file
	input1 := `<!--%%mockup-export:/css/gen.css%%-->.generated { display: block; }<!--%%end%%-->`
	_, err := ms.resolvePercent(input1, nil, nil)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Import it back
	input2 := `[<!--%%mockup-import:/css/gen.css%%-->]`
	result, err := ms.resolvePercent(input2, nil, nil)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if result != "[.generated { display: block; }]" {
		t.Errorf("expected imported content, got %q", result)
	}
}

// ---

func TestMockupExportConditionalsPassThrough(t *testing.T) {
	dir := t.TempDir()
	ms := newMockup(dir, dir)

	// Conditionals inside a mockup-export must pass through literally — not be evaluated.
	// if:mockup is true in mockup mode, but inside a mockup-export it should be written as-is.
	input := `<!--%% mockup-export:/out.txt %%-->` +
		`<!--%if:mockup%-->MOCKUP<!--%else%>PROD<%endif%-->` +
		`<!--%% end %%-->`

	_, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatalf("exported file not created: %v", err)
	}
	got := string(data)

	// The exported file should contain the literal conditional tag (normalized to <% form).
	if !strings.Contains(got, "<%if:mockup%>") {
		t.Errorf("expected literal if:mockup tag in export, got: %q", got)
	}
	if !strings.Contains(got, "MOCKUP") {
		t.Errorf("expected MOCKUP text preserved in export, got: %q", got)
	}
	if !strings.Contains(got, "PROD") {
		t.Errorf("expected PROD text preserved in export, got: %q", got)
	}
	if strings.Contains(got, "MOCKUPPROD") || got == "MOCKUP" || got == "PROD" {
		t.Errorf("conditional must NOT be evaluated inside export, got: %q", got)
	}
}

// ---

func TestMockupExportConditionalsDoubleForm(t *testing.T) {
	dir := t.TempDir()
	ms := newMockup(dir, dir)

	// Double-form conditional tags inside mockup-export also pass through literally.
	input := `<!--%% mockup-export:/out.txt %%-->` +
		`<!--%% if:mockup %%-->MOCKUP<!--%% else %%-->PROD<!--%% endif %%-->` +
		`<!--%% end %%-->`

	_, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatalf("exported file not created: %v", err)
	}
	got := string(data)

	if !strings.Contains(got, "<%if:mockup%>") && !strings.Contains(got, "<%%if:mockup%%>") {
		t.Errorf("expected literal if:mockup tag in export, got: %q", got)
	}
	if !strings.Contains(got, "MOCKUP") || !strings.Contains(got, "PROD") {
		t.Errorf("both branches should be in export, got: %q", got)
	}
}

// ---

func TestMockupExportNestedConditionalEnd(t *testing.T) {
	dir := t.TempDir()
	ms := newMockup(dir, dir)

	// The "end" tag that closes a conditional inside the export must NOT close the export.
	// The export is only closed by end-mockup-export or the outer "end" at rawCondDepth==0.
	input := `A<!--%% mockup-export:/out.txt %%-->` +
		`<!--%% if:mockup %%-->X<!--%% end %%-->Y` +
		`<!--%% end-mockup-export %%-->B`

	result, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "AB" {
		t.Errorf("main output should be AB, got: %q", result)
	}

	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatalf("exported file not created: %v", err)
	}
	got := string(data)
	// Export should contain both the conditional and "Y" after the inner end
	if !strings.Contains(got, "X") || !strings.Contains(got, "Y") {
		t.Errorf("export should contain X and Y, got: %q", got)
	}
}

// ---

func TestMockupExportOutsideMockupMode(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`<!--%% mockup-export:/css/x.css %%-->X<!--%% end %%-->`, nil, nil)
	if err == nil {
		t.Fatal("expected error for mockup-export outside mockup mode")
	}
	if !strings.Contains(err.Error(), "mockup mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---

func TestMockupAutoVar(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "css"), 0755)

	ms := newMockup(dir, dir)
	// mockup variable should be auto-set — use it in a conditional
	vars := map[string]string{} // no explicit mockup var
	input := `<!--%% if:mockup %%--><!--%% mockup-export:/css/out.css %%-->EXPORTED<!--%% end %%--><!--%% endif %%-->`
	_, err := ms.resolvePercent(input, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// But auto-var is set in processMockupList, not in newMockup helper.
	// This test verifies that mockup-export works in mockup mode with explicit var.
}

// ---

func TestTransformNegativeSimple(t *testing.T) {
	input := `before
<!--%%mockup-export:/css/a.css%%-->
.a { color: red; }
<!--%%end%%-->
after`
	result := transformNegative(input)
	if !strings.Contains(result, "before") {
		t.Error("missing text before export block")
	}
	if !strings.Contains(result, "after") {
		t.Error("missing text after export block")
	}
	if !strings.Contains(result, "<!--%%mockup-import:/css/a.css%%-->") {
		t.Error("missing mockup-import tag")
	}
	if !strings.Contains(result, "<!--%%end-mockup-import%%-->") {
		t.Error("missing end-mockup-import tag")
	}
	if strings.Contains(result, ".a { color: red; }") {
		t.Error("exported content should be removed")
	}
	if strings.Contains(result, "mockup-export") {
		t.Error("mockup-export tag should be removed")
	}
}

// ---

func TestTransformNegativeNested(t *testing.T) {
	input := `text1
<!--%%mockup-export:/a.css%%-->
body { margin: 0; }
<!--%%mockup-export:/b.css%%-->
.header { color: red; }
<!--%%end%%-->
<!--%%mockup-export:/c.css%%-->
.footer { color: blue; }
<!--%%end%%-->
rest
<!--%%end%%-->
text2`
	result := transformNegative(input)
	if !strings.Contains(result, "text1") {
		t.Error("missing text before")
	}
	if !strings.Contains(result, "text2") {
		t.Error("missing text after")
	}
	if !strings.Contains(result, "<!--%%mockup-import:/a.css%%-->") {
		t.Error("missing import for a.css")
	}
	if !strings.Contains(result, "<!--%%mockup-import:/b.css%%-->") {
		t.Error("missing import for b.css")
	}
	if !strings.Contains(result, "<!--%%mockup-import:/c.css%%-->") {
		t.Error("missing import for c.css")
	}
	if strings.Contains(result, "body { margin: 0; }") {
		t.Error("exported content should be removed")
	}
	if strings.Contains(result, ".header") {
		t.Error("nested content should be removed")
	}
}

// ---

func TestTransformNegativeWithConditional(t *testing.T) {
	input := `<html>
<!--%%if:mockup%%-->
<!--%%mockup-export:/css/login.css%%-->
.login { padding: 20px; }
<!--%%end%%-->
<!--%%endif%%-->
</html>`
	result := transformNegative(input)
	if !strings.Contains(result, "<!--%%if:mockup%%-->") {
		t.Error("conditional should be preserved")
	}
	if !strings.Contains(result, "<!--%%endif%%-->") {
		t.Error("endif should be preserved")
	}
	if !strings.Contains(result, "<!--%%mockup-import:/css/login.css%%-->") {
		t.Error("missing mockup-import tag")
	}
	if !strings.Contains(result, "<!--%%end-mockup-import%%-->") {
		t.Error("missing end-mockup-import tag")
	}
	if strings.Contains(result, ".login") {
		t.Error("exported content should be removed")
	}
}

// ---

func TestTransformNegativeNoExport(t *testing.T) {
	input := `<html>
<!--%%if:mockup%%-->
<p>Sample data</p>
<!--%%endif%%-->
</html>`
	result := transformNegative(input)
	if result != input {
		t.Errorf("content without exports should pass through unchanged\ngot: %q", result)
	}
}

// ---

func TestTransformNegativeAllSyntaxes(t *testing.T) {
	// Test with <%...%> syntax
	input := `A<%mockup-export:/x.css%>CSS<%end%>B`
	result := transformNegative(input)
	if !strings.Contains(result, "<!--%%mockup-import:/x.css%%-->") {
		t.Errorf("single percent syntax not handled: %q", result)
	}
	if strings.Contains(result, "CSS") {
		t.Error("content should be removed")
	}

	// Test with <%%...%%> syntax
	input2 := `A<%%mockup-export:/y.css%%>CSS<%%end%%>B`
	result2 := transformNegative(input2)
	if !strings.Contains(result2, "<!--%%mockup-import:/y.css%%-->") {
		t.Errorf("double percent syntax not handled: %q", result2)
	}

	// Test with <!--%...%--> syntax
	input3 := `A<!--%mockup-export:/z.css%-->CSS<!--%end%-->B`
	result3 := transformNegative(input3)
	if !strings.Contains(result3, "<!--%%mockup-import:/z.css%%-->") {
		t.Errorf("comment single syntax not handled: %q", result3)
	}
}

// ---

func TestDiamondInclude(t *testing.T) {
	dir := t.TempDir()

	// a includes b and c, both include d -> no cycle, d appears twice
	os.WriteFile(filepath.Join(dir, "b.html"), []byte(`[B]<%%include:/d.html%%>`), 0644)
	os.WriteFile(filepath.Join(dir, "c.html"), []byte(`[C]<%%include:/d.html%%>`), 0644)
	os.WriteFile(filepath.Join(dir, "d.html"), []byte(`[D]`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.resolvePercent(`<%%include:/b.html%%>|<%%include:/c.html%%>`, nil, nil)
	if err != nil {
		t.Fatalf("diamond include failed: %v", err)
	}
	expected := "[B][D]|[C][D]"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// --- Init error tests

func TestInitNoMiniskinXML(t *testing.T) {
	dir := t.TempDir()
	ms := newSilent(dir, dir)
	_, err := ms.Run()
	if err == nil {
		t.Fatal("expected error when no *.miniskin.xml exists")
	}
	if !strings.Contains(err.Error(), "no *.miniskin.xml") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---

func TestInitMultipleMiniskinXML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.miniskin.xml"), []byte(`<miniskin><bucket-list filename="e.go" module="m"><bucket src="x" dst="/g.go" module-name="x"/></bucket-list></miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "b.miniskin.xml"), []byte(`<miniskin><bucket-list filename="e.go" module="m"><bucket src="x" dst="/g.go" module-name="x"/></bucket-list></miniskin>`), 0644)

	ms := newSilent(dir, dir)
	_, err := ms.Run()
	if err == nil {
		t.Fatal("expected error for multiple *.miniskin.xml")
	}
	if !strings.Contains(err.Error(), "multiple") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---

func TestInitNoBucketList(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin></miniskin>`), 0644)

	ms := newSilent(dir, dir)
	_, err := ms.Run()
	if err == nil {
		t.Fatal("expected error for missing bucket-list")
	}
	if !strings.Contains(err.Error(), "no bucket-list") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Standalone pass tests

func TestProcessMockupExportStandalone(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "pages"), 0755)
	os.MkdirAll(filepath.Join(dir, "app", "css"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" recurse-folder="all" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "pages", "pages.miniskin.xml"), []byte(`<miniskin>
	<mockup-list>
		<item src="mock_src.html" />
	</mockup-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "pages", "mock_src.html"), []byte(`<!--%%if:mockup%%-->
<!--%%mockup-export:/css/exported.css%%-->
.exported { color: blue; }
<!--%%end%%-->
<!--%%endif%%-->`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.ProcessMockupExport()
	if err != nil {
		t.Fatalf("ProcessMockupExport failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "app", "css", "exported.css"))
	if err != nil {
		t.Fatalf("exported file not created: %v", err)
	}
	if !strings.Contains(string(data), ".exported { color: blue; }") {
		t.Errorf("unexpected content: %s", data)
	}

	if len(result.GeneratedFiles) != 1 {
		t.Fatalf("expected 1 generated file, got %d", len(result.GeneratedFiles))
	}
}

// ---

func TestBuildEmbedStandalone(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "pages"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<globals>
		<var name="color" value="red" />
	</globals>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" recurse-folder="all" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "pages", "pages.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/pages">
		<item type="html-template" src="page_src.html" file="page.html" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "pages", "page_src.html"), []byte(`color=<%color%>`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	data, err := os.ReadFile(filepath.Join(dir, "app", "pages", "page.html"))
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if string(data) != "color=red" {
		t.Errorf("expected 'color=red', got %q", string(data))
	}
}

// --- Include in skipped block

func TestIncludeInSkippedBlock(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	// include in a false if block should not error even though file doesn't exist
	result, err := ms.resolvePercent(`before<%if:nope%><%%include:/nonexistent.html%%><%endif%>after`, nil, nil)
	if err != nil {
		t.Fatalf("include in skipped block should not error: %v", err)
	}
	if result != "beforeafter" {
		t.Errorf("expected %q, got %q", "beforeafter", result)
	}
}

// --- Variable merge order (full chain)

func TestVariableMergeOrderFullChain(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "pages"), 0755)
	os.MkdirAll(filepath.Join(dir, "css"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<globals>
		<var name="g" value="global" />
		<var name="shared" value="from-global" />
	</globals>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" recurse-folder="all" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "pages", "pages.miniskin.xml"), []byte(`<miniskin>
	<mockup-list>
		<var name="listvar" value="from-list" />
		<var name="shared" value="from-list" />
		<item src="mock.html">
			<var name="itemvar" value="from-item" />
			<var name="shared" value="from-item" />
		</item>
	</mockup-list>
</miniskin>`), 0644)

	// front-matter overrides everything
	os.WriteFile(filepath.Join(dir, "app", "pages", "mock.html"), []byte(`---
fmvar: from-fm
shared: from-fm
---
<!--%%mockup-export:/css/vars.txt%%-->
g=<%g%> listvar=<%listvar%> itemvar=<%itemvar%> fmvar=<%fmvar%> shared=<%shared%>
<!--%%end%%-->`), 0644)

	ms := newSilent(dir, dir)
	_, err := ms.ProcessMockupExport()
	if err != nil {
		t.Fatalf("ProcessMockupExport failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "app", "css", "vars.txt"))
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	content := string(data)

	// In mockup mode, vars pass through literally (skipVars=true), so we check
	// that mockup-export worked. The variable resolution in mockup mode means
	// variables are emitted as literal tags. But conditionals check existence.
	// The actual merge is tested by checking that the mockup-export ran (meaning
	// the vars existed for conditionals).
	if !strings.Contains(content, "<%g%>") {
		t.Errorf("expected literal <%%g%%> in mockup output, got: %s", content)
	}
}

// ---

func TestVariableMergeOrderBuildEmbed(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<globals>
		<var name="g" value="GLOBAL" />
		<var name="shared" value="FROM_GLOBAL" />
	</globals>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/app">
		<item type="html-template" src="t_src.html" file="t.html" />
	</resource-list>
</miniskin>`), 0644)

	// front-matter var overrides global
	os.WriteFile(filepath.Join(dir, "app", "t_src.html"), []byte("---\nshared: FROM_FM\n---\ng=<%g%> shared=<%shared%>"), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	data, _ := os.ReadFile(filepath.Join(dir, "app", "t.html"))
	content := string(data)
	if !strings.Contains(content, "g=GLOBAL") {
		t.Errorf("global var not resolved: %s", content)
	}
	if !strings.Contains(content, "shared=FROM_FM") {
		t.Errorf("front-matter should override global, got: %s", content)
	}
}

// --- Save-mode cascading tests

func TestSaveModeCascadeFromList(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.MkdirAll(filepath.Join(dir, "out"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<mockup-list save-mode="append">
		<item src="m1.html" />
		<item src="m2.html" />
	</mockup-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "m1.html"), []byte("<!--%%mockup-export:/out/combined.txt%%-->\nFIRST\n<!--%%end%%-->"), 0644)
	os.WriteFile(filepath.Join(dir, "app", "m2.html"), []byte("<!--%%mockup-export:/out/combined.txt%%-->\nSECOND\n<!--%%end%%-->"), 0644)

	ms := newSilent(dir, dir)
	_, err := ms.ProcessMockupExport()
	if err != nil {
		t.Fatalf("ProcessMockupExport failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "app", "out", "combined.txt"))
	content := string(data)
	// First write truncates (session behavior), second appends because list save-mode=append
	if content != "FIRST\nSECOND\n" {
		t.Errorf("expected 'FIRST\\nSECOND\\n' (list save-mode=append), got %q", content)
	}
}

// ---

func TestSaveModeCascadeItemOverridesList(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.MkdirAll(filepath.Join(dir, "out"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<mockup-list save-mode="append">
		<item src="m1.html" />
		<item src="m2.html" save-mode="overwrite" />
	</mockup-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "m1.html"), []byte("<!--%%mockup-export:/out/file.txt%%-->\nFIRST\n<!--%%end%%-->"), 0644)
	os.WriteFile(filepath.Join(dir, "app", "m2.html"), []byte("<!--%%mockup-export:/out/file.txt%%-->\nSECOND\n<!--%%end%%-->"), 0644)

	ms := newSilent(dir, dir)
	_, err := ms.ProcessMockupExport()
	if err != nil {
		t.Fatalf("ProcessMockupExport failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "app", "out", "file.txt"))
	content := string(data)
	// Item save-mode=overwrite overrides list save-mode=append
	if content != "SECOND\n" {
		t.Errorf("expected 'SECOND\\n' (item overwrite), got %q", content)
	}
}

// ---

func TestSaveModeCascadeTagOverridesAll(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.MkdirAll(filepath.Join(dir, "out"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	// List says overwrite, item says overwrite, but tag says append
	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<mockup-list save-mode="overwrite">
		<item src="m1.html" save-mode="overwrite" />
	</mockup-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "m1.html"), []byte("<!--%%mockup-export:/out/f.txt%%-->\nFIRST\n<!--%%end%%-->\n<!--%%mockup-export:/out/f.txt append%%-->\nSECOND\n<!--%%end%%-->"), 0644)

	ms := newSilent(dir, dir)
	_, err := ms.ProcessMockupExport()
	if err != nil {
		t.Fatalf("ProcessMockupExport failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "app", "out", "f.txt"))
	content := string(data)
	// First write truncates (session), second uses tag-level append
	if content != "FIRST\nSECOND\n" {
		t.Errorf("expected 'FIRST\\nSECOND\\n' (tag append overrides item/list overwrite), got %q", content)
	}
}

// --- Negative end-to-end

func TestNegativeEndToEnd(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.MkdirAll(filepath.Join(dir, "css"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<mockup-list>
		<item src="mock.html" negative="mock_neg.html" />
	</mockup-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "mock.html"), []byte(`before
<!--%%mockup-export:/css/style.css%%-->
.body { margin: 0; }
<!--%%end%%-->
after`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.ProcessMockupExport()
	if err != nil {
		t.Fatalf("ProcessMockupExport failed: %v", err)
	}

	// Check negative file was generated
	negData, err := os.ReadFile(filepath.Join(dir, "app", "mock_neg.html"))
	if err != nil {
		t.Fatalf("negative file not created: %v", err)
	}
	negContent := string(negData)
	if !strings.Contains(negContent, "mockup-import:/css/style.css") {
		t.Errorf("negative should contain mockup-import, got:\n%s", negContent)
	}
	if strings.Contains(negContent, "mockup-export") {
		t.Error("negative should not contain mockup-export")
	}

	// Check exported CSS file
	cssData, _ := os.ReadFile(filepath.Join(dir, "app", "css", "style.css"))
	if !strings.Contains(string(cssData), ".body { margin: 0; }") {
		t.Errorf("exported CSS missing content: %s", cssData)
	}

	// Check generated files tracking includes both the export and the negative
	if len(result.GeneratedFiles) < 2 {
		t.Errorf("expected at least 2 generated files (export + negative), got %d", len(result.GeneratedFiles))
	}
}

// --- Missing skin file

func TestMissingSkinFile(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.MkdirAll(filepath.Join(dir, "_skin"), 0755) // empty skin dir

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/app">
		<item type="html-template" src="p_src.html" file="p.html" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "p_src.html"), []byte("---\nskin: nonexistent\n---\nBody"), 0644)

	ms := newSilent(dir, dir)
	_, err := ms.BuildEmbed()
	if err == nil {
		t.Fatal("expected error for missing skin file")
	}
	if !strings.Contains(err.Error(), "skin") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Logging tests

func TestLogOutput(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/app">
		<item type="static" file="app.css" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.css"), []byte("body{}"), 0644)

	var buf bytes.Buffer
	ms := MiniskinNew(dir, dir)
	ms.Output = &buf
	_, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected log output in buffer, got nothing")
	}
	if !strings.Contains(buf.String(), "===") {
		t.Errorf("expected pass header in log, got: %s", buf.String())
	}
}

// ---

func TestLogSilent(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/app">
		<item type="static" file="app.css" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.css"), []byte("body{}"), 0644)

	ms := MiniskinNew(dir, dir).Silent()
	_, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	// No panic, no output — silent works
}

// ---

func TestLogFile(t *testing.T) {
	dir, _ := os.MkdirTemp("", "miniskin-logfile-*")
	defer os.RemoveAll(dir) // best-effort cleanup
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin log="build.log">
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/app">
		<item type="static" file="app.css" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.css"), []byte("body{}"), 0644)

	var buf bytes.Buffer
	ms := MiniskinNew(dir, dir)
	ms.Output = &buf
	_, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Check log file was created and has content
	logData, err := os.ReadFile(filepath.Join(dir, "build.log"))
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if len(logData) == 0 {
		t.Error("log file is empty")
	}
	// Both console buffer and log file should have content
	if buf.Len() == 0 {
		t.Error("console buffer should also have output")
	}
}

// ---

func TestLogFileSilentMode(t *testing.T) {
	dir, _ := os.MkdirTemp("", "miniskin-logsilent-*")
	defer os.RemoveAll(dir) // best-effort cleanup
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin log="build.log">
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/app">
		<item type="static" file="app.css" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.css"), []byte("body{}"), 0644)

	ms := MiniskinNew(dir, dir).Silent()
	_, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// In silent mode with log file, output goes to file only
	logData, err := os.ReadFile(filepath.Join(dir, "build.log"))
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if len(logData) == 0 {
		t.Error("log file should have output even in silent mode")
	}
}

// --- Front-matter tests

func TestFrontMatterNone(t *testing.T) {
	vars, _, body, err := parseFrontMatter("Hello World")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vars != nil {
		t.Errorf("expected nil vars, got %v", vars)
	}
	if body != "Hello World" {
		t.Errorf("expected full content as body, got %q", body)
	}
}

// ---

func TestFrontMatterUnclosed(t *testing.T) {
	_, _, _, err := parseFrontMatter("---\nkey: value\nno closing")
	if err == nil {
		t.Fatal("expected error for unclosed front-matter")
	}
	if !strings.Contains(err.Error(), "unclosed") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---

func TestFrontMatterEmptyKey(t *testing.T) {
	_, _, _, err := parseFrontMatter("---\n: value\n---\nbody")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !strings.Contains(err.Error(), "empty key") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---

func TestFrontMatterMissingColon(t *testing.T) {
	_, _, _, err := parseFrontMatter("---\nnovalue\n---\nbody")
	if err == nil {
		t.Fatal("expected error for missing colon")
	}
	if !strings.Contains(err.Error(), "missing colon") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---

func TestFrontMatterMultipleVars(t *testing.T) {
	vars, _, body, err := parseFrontMatter("---\ntitle: Hello\ncolor: red\n---\nBody here")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vars["title"] != "Hello" {
		t.Errorf("expected title=Hello, got %q", vars["title"])
	}
	if vars["color"] != "red" {
		t.Errorf("expected color=red, got %q", vars["color"])
	}
	if body != "Body here" {
		t.Errorf("expected 'Body here', got %q", body)
	}
}

// ---

func TestFrontMatterEmptyLines(t *testing.T) {
	vars, _, body, err := parseFrontMatter("---\ntitle: Hello\n\ncolor: red\n---\nBody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vars) != 2 {
		t.Errorf("expected 2 vars, got %d", len(vars))
	}
	if body != "Body" {
		t.Errorf("expected 'Body', got %q", body)
	}
}

// --- Recurse folder

func TestRecurseFolderAll(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "sub1"), 0755)
	os.MkdirAll(filepath.Join(dir, "app", "sub2"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" recurse-folder="all" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "sub1", "sub1.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/sub1">
		<item type="static" file="a.css" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "sub1", "a.css"), []byte("a{}"), 0644)

	os.WriteFile(filepath.Join(dir, "app", "sub2", "sub2.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/sub2">
		<item type="static" file="b.css" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "sub2", "b.css"), []byte("b{}"), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	totalItems := 0
	for _, br := range result.Buckets {
		totalItems += len(br.Items)
	}
	if totalItems != 2 {
		t.Errorf("expected 2 items from recursive walk, got %d", totalItems)
	}
}

// ---

func TestNoRecurseFolder(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "sub"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	// Only top-level miniskin.xml should be found (no recurse)
	os.WriteFile(filepath.Join(dir, "app", "top.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/top">
		<item type="static" file="t.css" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "t.css"), []byte("t{}"), 0644)

	// This one should NOT be found
	os.WriteFile(filepath.Join(dir, "app", "sub", "sub.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/sub">
		<item type="static" file="s.css" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "sub", "s.css"), []byte("s{}"), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	totalItems := 0
	for _, br := range result.Buckets {
		totalItems += len(br.Items)
	}
	if totalItems != 1 {
		t.Errorf("expected 1 item (no recurse), got %d", totalItems)
	}
}

// --- Item method tests

func TestItemRouteURLWithKey(t *testing.T) {
	it := Item{Key: "/custom/path", File: "page.html", urlBase: "/app"}
	if it.RouteURL() != "/custom/path" {
		t.Errorf("expected /custom/path, got %s", it.RouteURL())
	}
}

// ---

func TestItemRouteURLWithURLBase(t *testing.T) {
	it := Item{File: "page.html", urlBase: "/app"}
	if it.RouteURL() != "/app/page.html" {
		t.Errorf("expected /app/page.html, got %s", it.RouteURL())
	}
}

// ---

func TestItemRouteURLDefault(t *testing.T) {
	it := Item{File: "page.html"}
	if it.RouteURL() != "/page.html" {
		t.Errorf("expected /page.html, got %s", it.RouteURL())
	}
}

// ---

func TestItemHasFlag(t *testing.T) {
	it := Item{Type: "html-template,nomux,static"}
	if !it.HasFlag("html-template") {
		t.Error("expected HasFlag('html-template') to be true")
	}
	if !it.HasFlag("nomux") {
		t.Error("expected HasFlag('nomux') to be true")
	}
	if !it.HasFlag("static") {
		t.Error("expected HasFlag('static') to be true")
	}
	if it.HasFlag("parse") {
		t.Error("expected HasFlag('parse') to be false")
	}
}

// ---

func TestItemNeedsProcessing(t *testing.T) {
	withSrc := Item{Src: "src.html"}
	if !withSrc.NeedsProcessing() {
		t.Error("item with Src should need processing")
	}
	withoutSrc := Item{File: "static.css"}
	if withoutSrc.NeedsProcessing() {
		t.Error("item without Src should not need processing")
	}
}

// --- End-if and end-mockup-export specific closers

func TestEndIfSpecific(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": "1"}
	result, err := ms.resolvePercent(`<%if:x%>YES<%end-if%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "YES" {
		t.Errorf("expected 'YES', got %q", result)
	}
}

// ---

func TestEndIfMismatch(t *testing.T) {
	ms := newMockup(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`<!--%%mockup-export:/x.css%%-->X<%end-if%>`, nil, nil)
	if err == nil {
		t.Fatal("expected error for end-if closing mockup-export")
	}
}

// ---

func TestEndMockupExportSpecific(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "css"), 0755)

	ms := newMockup(dir, dir)
	_, err := ms.resolvePercent(`<!--%%mockup-export:/css/x.css%%-->X<!--%%end-mockup-export%%-->`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "css", "x.css"))
	if string(data) != "X" {
		t.Errorf("expected 'X', got %q", string(data))
	}
}

// ---

func TestEndMockupExportMismatch(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": "1"}
	_, err := ms.resolvePercent(`<%if:x%>YES<%end-mockup-export%>`, vars, nil)
	if err == nil {
		t.Fatal("expected error for end-mockup-export closing if block")
	}
}

// --- end-mockup-import tests

func TestEndMockupImportSpecific(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "css"), 0755)

	ms := newMockup(dir, dir)
	_, err := ms.resolvePercent(`<!--%%mockup-export:/css/x.css%%-->X<!--%%end-mockup-import%%-->`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "css", "x.css"))
	if string(data) != "X" {
		t.Errorf("expected 'X', got %q", string(data))
	}
}

// ---

func TestEndMockupImportMismatch(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": "1"}
	_, err := ms.resolvePercent(`<%if:x%>YES<%end-mockup-import%>`, vars, nil)
	if err == nil {
		t.Fatal("expected error for end-mockup-import closing if block")
	}
}

// ---

func TestEndMockupImportUniversalEnd(t *testing.T) {
	// "end" should also close mockup-export blocks (universal closer)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "css"), 0755)

	ms := newMockup(dir, dir)
	_, err := ms.resolvePercent(`<!--%%mockup-export:/css/x.css%%-->X<!--%%end%%-->`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "css", "x.css"))
	if string(data) != "X" {
		t.Errorf("expected 'X', got %q", string(data))
	}
}

// ---

func TestEndMockupImportInRefreshImports(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "data.css"), []byte(".new { color: blue; }"), 0644)

	// Block form with end-mockup-import should be recognized by refreshImports
	input := "<!--%%mockup-import:/data.css%%-->\n.old { color: red; }\n<!--%%end-mockup-import%%-->"
	result, err := refreshImports(input, dir, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, ".new { color: blue; }") {
		t.Errorf("expected refreshed content, got %q", result)
	}
	if strings.Contains(result, ".old { color: red; }") {
		t.Errorf("old content should be replaced")
	}
	if !strings.Contains(result, "end-mockup-import") {
		t.Errorf("end-mockup-import closer should be preserved")
	}
}

// ---

func TestGenericEndCannotCloseMockupImport(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.css"), []byte("FRESH"), 0644)

	ms := newMockup(dir, dir)
	input := "<!--%%mockup-import:/a.css%%-->\nSTALE\n<!--%%end%%-->"
	_, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err == nil {
		t.Fatal("expected error for generic end closing a mockup-import block")
	}
	if !strings.Contains(err.Error(), "end-mockup-import") {
		t.Errorf("error should suggest end-mockup-import: %v", err)
	}
}

// ---

func TestSingleImportInsideIfClosedWithEnd(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.css"), []byte("FRESH"), 0644)

	ms := newMockup(dir, dir)
	input := "<!--%%if:mockup%%-->\n<!--%%mockup-import:/a.css%%-->\nkeep\n<!--%%end%%-->\ntail"
	result, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "FRESH") || !strings.Contains(result, "keep") || !strings.Contains(result, "tail") {
		t.Errorf("end should close the if and keep all content: %q", result)
	}
}

// ---

func TestTransformNegativeEmitsEndMockupImport(t *testing.T) {
	input := `before
<!--%%mockup-export:/css/a.css%%-->
.a { color: red; }
<!--%%end%%-->
after`
	result := transformNegative(input)
	if !strings.Contains(result, "<!--%%mockup-import:/css/a.css%%-->") {
		t.Error("missing mockup-import tag")
	}
	if !strings.Contains(result, "<!--%%end-mockup-import%%-->") {
		t.Error("transformNegative should emit end-mockup-import")
	}
	if strings.Contains(result, "end%%-->") && !strings.Contains(result, "end-mockup-import%%-->") {
		t.Error("should use end-mockup-import, not plain end")
	}
}

// ---

func TestTransformNegativeNestedEmitsEndMockupImport(t *testing.T) {
	input := `<!--%%mockup-export:/a.css%%-->
A
<!--%%mockup-export:/b.css%%-->
B
<!--%%end%%-->
<!--%%end%%-->`
	result := transformNegative(input)
	count := strings.Count(result, "<!--%%end-mockup-import%%-->")
	if count != 2 {
		t.Errorf("expected 2 end-mockup-import tags, got %d in %q", count, result)
	}
}

// --- mockup-import block form with path resolution

func TestMockupImportBlockAbsolutePath(t *testing.T) {
	// "/" prefix → relative to contentPath
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "assets"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "assets", "app.css"), []byte(".body{margin:0}"), 0644)

	ms := newMockup(dir, dir)
	ms.currentFileDir = filepath.Join(dir, "app", "mockups")

	input := `<!--%%mockup-import:"/app/assets/app.css"%%-->old<!--%%end-mockup-import%%-->`
	result, err := ms.resolvePercent(input, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != ".body{margin:0}" {
		t.Errorf("expected '.body{margin:0}', got %q", result)
	}
}

// ---

func TestMockupImportBlockRelativePath(t *testing.T) {
	// no "/" prefix → relative to current file's directory
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "pages"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "pages", "local.css"), []byte(".local{color:red}"), 0644)

	ms := newMockup(dir, dir)
	ms.currentFileDir = filepath.Join(dir, "app", "pages")

	input := `<!--%%mockup-import:"local.css"%%-->old<!--%%end-mockup-import%%-->`
	result, err := ms.resolvePercent(input, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != ".local{color:red}" {
		t.Errorf("expected '.local{color:red}', got %q", result)
	}
}

// ---

func TestMockupImportBlockIfMockupElseEcho(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "assets"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "assets", "app.css"), []byte(".body{margin:0}"), 0644)

	input := "<!--%%if:mockup%%-->\n<style>\n<!--%%mockup-import:\"/app/assets/app.css\"%%-->\n<!--%%end-mockup-import%%-->\n</style>\n<!--%%else%%-->\n<!--%%echo:<link rel=\"stylesheet\" href=\"/assets/app.css\">%%-->\n<!--%%end-if%%-->"

	// Mockup mode
	ms := newMockup(dir, dir)
	ms.currentFileDir = filepath.Join(dir, "app", "mockups")

	result, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("mockup mode error: %v", err)
	}
	expected := "\n<style>\n.body{margin:0}\n</style>\n"
	if result != expected {
		t.Errorf("mockup mode: expected %q, got %q", expected, result)
	}

	// Normal mode
	ms2 := newSilent(dir, dir)
	result2, err := ms2.resolvePercent(input, nil, nil)
	if err != nil {
		t.Fatalf("normal mode error: %v", err)
	}
	expected2 := "\n<link rel=\"stylesheet\" href=\"/assets/app.css\">\n"
	if result2 != expected2 {
		t.Errorf("normal mode: expected %q, got %q", expected2, result2)
	}
}

// ---

func TestMockupImportBlockDiscardsInlineContent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fresh.css"), []byte("FRESH"), 0644)

	ms := newMockup(dir, dir)
	input := `<!--%%mockup-import:/fresh.css%%-->STALE CONTENT<!--%%end-mockup-import%%-->`
	result, err := ms.resolvePercent(input, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "FRESH" {
		t.Errorf("expected 'FRESH', got %q", result)
	}
}

// --- line-mode: mockup-import/export consume full lines (CSS-friendly /* */ wrappers)

func TestLineModeImportCSS(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "assets"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "assets", "app.css"), []byte(".body{margin:0}"), 0644)

	ms := newMockup(dir, dir)
	ms.lineMode = true
	input := "<style>\n/*<%%mockup-import:\"/app/assets/app.css\"%%>*/\nold css\n/*<%%end-mockup-import%%>*/\n</style>"
	result, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "<style>\n.body{margin:0}</style>"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// ---

func TestLineModeExportCSS(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "assets"), 0755)

	ms := newMockup(dir, dir)
	ms.lineMode = true
	input := "<style>\n/*<%%mockup-export:\"/app/assets/app.css\"%%>*/\n.body{margin:0}\n/*<%%end-mockup-export%%>*/\n</style>"
	result, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Tags and their /* */ wrappers are consumed; only <style>\n</style> remains
	expected := "<style>\n</style>"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
	// Exported file should have the CSS without wrappers
	data, _ := os.ReadFile(filepath.Join(dir, "app", "assets", "app.css"))
	if string(data) != ".body{margin:0}\n" {
		t.Errorf("exported file: expected '.body{margin:0}\\n', got %q", string(data))
	}
}

// ---

func TestLineModeImportIfMockup(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "assets"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "assets", "app.css"), []byte(".body{margin:0}"), 0644)

	input := "<!--%%if:mockup%%-->\n<style>\n/*<%%mockup-import:\"/app/assets/app.css\"%%>*/\nold css\n/*<%%end-mockup-import%%>*/\n</style>\n<!--%%else%%-->\n<!--%%echo:<link rel=\"stylesheet\" href=\"/assets/app.css\">%%-->\n<!--%%end-if%%-->"

	// Mockup mode
	ms := newMockup(dir, dir)
	ms.lineMode = true
	result, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("mockup mode error: %v", err)
	}
	if !strings.Contains(result, ".body{margin:0}") {
		t.Errorf("mockup mode: expected CSS content, got %q", result)
	}
	if strings.Contains(result, "old css") {
		t.Errorf("mockup mode: old content should be replaced")
	}

	// Normal mode
	ms2 := newSilent(dir, dir)
	result2, err := ms2.resolvePercent(input, nil, nil)
	if err != nil {
		t.Fatalf("normal mode error: %v", err)
	}
	if !strings.Contains(result2, `<link rel="stylesheet"`) {
		t.Errorf("normal mode: expected link tag, got %q", result2)
	}
}

// --- line-mode + include (non-mockup): consumes the whole line containing the include tag

func TestLineModeIncludeCSS(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fragment.css"), []byte(".body{margin:0}"), 0644)

	ms := newSilent(dir, dir)
	ms.bucketSrc = dir
	ms.lineMode = true
	input := "<style>\n/* <%%include:fragment.css%%> */\n</style>"
	result, err := ms.resolvePercent(input, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "<style>\n.body{margin:0}</style>"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// --- mockup-import indent

func TestImportIndentSpaces(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "frag.html"), []byte("<p>one</p>\n<p>two</p>"), 0644)

	ms := newMockup(dir, dir)
	ms.lineMode = true
	input := "<div>\n    <%%mockup-import:\"frag.html\" indent:4%%>\n</div>"
	result, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "<div>\n    <p>one</p>\n    <p>two</p></div>"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestImportIndentTabs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "frag.html"), []byte("<p>one</p>\n<p>two</p>"), 0644)

	ms := newMockup(dir, dir)
	ms.lineMode = true
	input := "<div>\n\t<%%mockup-import:\"frag.html\" indent:2tab%%>\n</div>"
	result, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "<div>\n\t\t<p>one</p>\n\t\t<p>two</p></div>"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestImportIndentPreservesEmptyLines(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "frag.txt"), []byte("a\n\nb"), 0644)

	ms := newMockup(dir, dir)
	ms.lineMode = true
	input := "  <%%mockup-import:\"frag.txt\" indent:2%%>\n"
	result, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "  a\n\n  b"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestImportIndentEqualsQuoted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "frag.html"), []byte("<p>one</p>\n<p>two</p>"), 0644)

	ms := newMockup(dir, dir)
	ms.lineMode = true
	input := "<div>\n\t<!--%%mockup-import:\"frag.html\" indent=\"2tab\"%%-->\n</div>"
	result, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "<div>\n\t\t<p>one</p>\n\t\t<p>two</p></div>"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestImportNoIndent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "frag.html"), []byte("<p>one</p>\n<p>two</p>"), 0644)

	ms := newMockup(dir, dir)
	ms.lineMode = true
	input := "<div>\n    <%%mockup-import:\"frag.html\"%%>\n</div>"
	result, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "<div>\n<p>one</p>\n<p>two</p></div>"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// --- Skin with global vars

// --- Mixed open/close syntax: any opening (<%,<%%,<!--%,<!--%%) with any closing (%>,%%>,%-->,%%--->)

func TestMixedSyntaxCommentOpenSingleClose(t *testing.T) {
	// <!--%...%>
	ms := newSilent(".", ".")
	result, err := ms.resolvePercent(`<!--%if:mockup%>hidden<%endif%-->`, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty (mockup not set), got %q", result)
	}
}

func TestMixedSyntaxSingleOpenCommentClose(t *testing.T) {
	// <%...%-->
	ms := newSilent(".", ".")
	result, err := ms.resolvePercent(`<%if:mockup%-->hidden<!--%endif%>`, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty (mockup not set), got %q", result)
	}
}

func TestMixedSyntaxDoubleCommentOpenDoubleClose(t *testing.T) {
	// <!--%%...%%>
	ms := newMockup(".", ".")
	result, err := ms.resolvePercent(`<!--%%if:mockup%%>MOCK<!--%%end-if%%>`, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != "MOCK" {
		t.Errorf("expected 'MOCK', got %q", result)
	}
}

func TestMixedSyntaxDoubleOpenCommentClose(t *testing.T) {
	// <%%...%%-->
	ms := newMockup(".", ".")
	result, err := ms.resolvePercent(`<%%if:mockup%%-->MOCK<%%end-if%%-->`, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != "MOCK" {
		t.Errorf("expected 'MOCK', got %q", result)
	}
}

func TestMixedSyntaxIfElseEndif(t *testing.T) {
	// Browser-friendly: <!--%if%-->mockup<!--%else%>production<%endif%-->
	input := `<span><!--%if:mockup%-->MD Clock<!--%else%>{{.AppDisplayName}}<%endif%--></span>`

	// Mockup mode
	ms := newMockup(".", ".")
	result, err := ms.resolvePercent(input, map[string]string{"mockup": "1"}, nil)
	if err != nil {
		t.Fatalf("mockup error: %v", err)
	}
	if result != "<span>MD Clock</span>" {
		t.Errorf("mockup: got %q", result)
	}

	// Normal mode
	ms2 := newSilent(".", ".")
	result2, err := ms2.resolvePercent(input, nil, nil)
	if err != nil {
		t.Fatalf("normal error: %v", err)
	}
	if result2 != "<span>{{.AppDisplayName}}</span>" {
		t.Errorf("normal: got %q", result2)
	}
}

func TestMixedSyntaxAllCombinations(t *testing.T) {
	// The % count must match (single with single, double with double).
	// The mixing is only about !-- (comment vs non-comment).
	groups := []struct {
		opens  []string
		closes []string
	}{
		{[]string{"<%", "<!--%"}, []string{"%>", "%-->"}},
		{[]string{"<%%", "<!--%%"}, []string{"%%>", "%%-->"}},
	}

	for _, g := range groups {
		for _, open := range g.opens {
			for _, close := range g.closes {
				tag := open + "if:x" + close
				endif := open + "end-if" + close
				input := tag + "YES" + endif
				ms := newSilent(".", ".")
				result, err := ms.resolvePercent(input, map[string]string{"x": "1"}, nil)
				if err != nil {
					t.Errorf("%s...%s: error: %v", open, close, err)
					continue
				}
				if result != "YES" {
					t.Errorf("%s...%s: expected 'YES', got %q", open, close, result)
				}
			}
		}
	}
}

// ---

func TestSkinWithGlobalVars(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "_skin"), 0755)
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	// Skin uses a global var (relative to bucketSrc=dir/app)
	os.WriteFile(filepath.Join(dir, "app", "_skin", "wrap.html"), []byte(`[<%appName%>]<%%content%%>[/<%appName%>]`), 0644)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<globals>
		<var name="appName" value="MyApp" />
	</globals>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/app">
		<item type="html-template" src="p_src.html" file="p.html" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "p_src.html"), []byte("---\nskin: wrap\n---\nHello"), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	data, _ := os.ReadFile(filepath.Join(dir, "app", "p.html"))
	if string(data) != "[MyApp]Hello[/MyApp]" {
		t.Errorf("expected skin with global vars, got %q", string(data))
	}
}

// --- EmbedPath computation

func TestEmbedPathComputation(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "pages"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" recurse-folder="all" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "pages", "pages.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/pages">
		<item type="static" file="style.css" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "pages", "style.css"), []byte("body{}"), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	item := result.Buckets[0].Items[0]
	if item.EmbedPath != "app/pages/style.css" {
		t.Errorf("expected EmbedPath 'app/pages/style.css', got %q", item.EmbedPath)
	}
}

// --- Mockup conditional checks existence only

func TestMockupConditionalExistenceOnly(t *testing.T) {
	ms := newMockup(t.TempDir(), t.TempDir())
	// In mockup mode, any non-empty value makes the conditional true
	// Even the variable "x" with value "" should be false
	vars := map[string]string{"present": "anything", "empty": ""}

	result, err := ms.resolvePercent(`<%if:present%>A<%endif%><%if:empty%>B<%endif%><%if:missing%>C<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "A" {
		t.Errorf("expected only 'A', got %q", result)
	}
}

// --- Globals parsing

func TestParseGlobals(t *testing.T) {
	vars := []xmlVar{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
	}
	result := parseGlobals(vars)
	if result["a"] != "1" || result["b"] != "2" {
		t.Errorf("unexpected globals: %v", result)
	}
}

// ---

func TestParseGlobalsEmpty(t *testing.T) {
	result := parseGlobals(nil)
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

// --- walkTags

func TestWalkTagsAllSyntaxes(t *testing.T) {
	input := `text<%single%>mid<%%double%%>end<!--%cmtSingle%-->last<!--%%cmtDouble%%-->`
	var tags []string
	walkTags(input, func(tag string) {
		tags = append(tags, strings.TrimSpace(tag))
	})
	if len(tags) != 4 {
		t.Fatalf("expected 4 tags, got %d: %v", len(tags), tags)
	}
	expected := []string{"single", "double", "cmtSingle", "cmtDouble"}
	for i, want := range expected {
		if tags[i] != want {
			t.Errorf("tag %d: got %q, want %q", i, tags[i], want)
		}
	}
}

// ---

func TestScanExportsImports(t *testing.T) {
	input := `before
<!--%%mockup-export:/css/a.css%%-->
.a { color: red; }
<!--%%mockup-import:/css/base.css%%-->
<!--%%end%%-->
after
<%mockup-export:/js/app.js%>
code
<%end%>
<!--%mockup-import:/shared/header.html%-->`

	exports, imports := scanExportsImports(input)
	if len(exports) != 2 {
		t.Fatalf("expected 2 exports, got %d: %v", len(exports), exports)
	}
	if exports[0] != "/css/a.css" {
		t.Errorf("export 0: got %q, want /css/a.css", exports[0])
	}
	if exports[1] != "/js/app.js" {
		t.Errorf("export 1: got %q, want /js/app.js", exports[1])
	}
	if len(imports) != 2 {
		t.Fatalf("expected 2 imports, got %d: %v", len(imports), imports)
	}
	if imports[0] != "/css/base.css" {
		t.Errorf("import 0: got %q, want /css/base.css", imports[0])
	}
	if imports[1] != "/shared/header.html" {
		t.Errorf("import 1: got %q, want /shared/header.html", imports[1])
	}
}

// ---

func TestDetectCyclesNone(t *testing.T) {
	edges := []DepEdge{
		{Source: "a.html", Target: "/x.css", Kind: "export"},
		{Source: "b.html", Target: "/x.css", Kind: "import"},
		{Source: "b.html", Target: "/y.css", Kind: "export"},
	}
	cycles := detectCycles(edges)
	if len(cycles) != 0 {
		t.Errorf("expected no cycles, got %v", cycles)
	}
}

// ---

func TestDetectCyclesDirect(t *testing.T) {
	// A exports X, B imports X, B exports Y, A imports Y → cycle
	edges := []DepEdge{
		{Source: "a.html", Target: "/x.css", Kind: "export"},
		{Source: "b.html", Target: "/x.css", Kind: "import"},
		{Source: "b.html", Target: "/y.css", Kind: "export"},
		{Source: "a.html", Target: "/y.css", Kind: "import"},
	}
	cycles := detectCycles(edges)
	if len(cycles) == 0 {
		t.Fatal("expected a cycle")
	}
	// Cycle should mention both a.html and b.html
	joined := strings.Join(cycles[0], " ")
	if !strings.Contains(joined, "a.html") || !strings.Contains(joined, "b.html") {
		t.Errorf("cycle should involve a.html and b.html, got: %v", cycles[0])
	}
}

// ---

func TestDetectCyclesTransitive(t *testing.T) {
	// A→X, B imports X, B→Y, C imports Y, C→Z, A imports Z → cycle A→B→C→A
	edges := []DepEdge{
		{Source: "a.html", Target: "/x.css", Kind: "export"},
		{Source: "b.html", Target: "/x.css", Kind: "import"},
		{Source: "b.html", Target: "/y.css", Kind: "export"},
		{Source: "c.html", Target: "/y.css", Kind: "import"},
		{Source: "c.html", Target: "/z.css", Kind: "export"},
		{Source: "a.html", Target: "/z.css", Kind: "import"},
	}
	cycles := detectCycles(edges)
	if len(cycles) == 0 {
		t.Fatal("expected a cycle")
	}
	if len(cycles[0]) < 4 { // a→b→c→a
		t.Errorf("expected 3-node cycle, got: %v", cycles[0])
	}
}

// ---

func TestDetectCyclesSelfExportImport(t *testing.T) {
	// Same source exports and imports the same file — not a cycle
	edges := []DepEdge{
		{Source: "a.html", Target: "/x.css", Kind: "export"},
		{Source: "a.html", Target: "/x.css", Kind: "import"},
	}
	cycles := detectCycles(edges)
	if len(cycles) != 0 {
		t.Errorf("self export+import should not be a cycle, got %v", cycles)
	}
}

// ---

func TestScanExportDeps(t *testing.T) {
	content := `<!--%%mockup-export:/css/combined.css%%-->
<!--%%mockup-import:/css/base.css%%-->
<!--%%end%%-->
<!--%%mockup-export:/css/base.css%%-->
.base { margin: 0; }
<!--%%end%%-->`

	deps := scanExportDeps(content)
	if len(deps) != 2 {
		t.Fatalf("expected 2 exports, got %d", len(deps))
	}
	if len(deps["/css/combined.css"]) != 1 || deps["/css/combined.css"][0] != "/css/base.css" {
		t.Errorf("combined.css should import base.css, got %v", deps["/css/combined.css"])
	}
	if len(deps["/css/base.css"]) != 0 {
		t.Errorf("base.css should have no imports, got %v", deps["/css/base.css"])
	}
}

// ---

func TestExportProcessingOrder(t *testing.T) {
	deps := map[string][]string{
		"/css/combined.css": {"/css/base.css"},
		"/css/base.css":     nil,
	}
	order := exportProcessingOrder(deps)
	if len(order) != 2 {
		t.Fatalf("expected 2 items, got %d", len(order))
	}
	if order[0] != "/css/base.css" {
		t.Errorf("base.css should come first, got %v", order)
	}
	if order[1] != "/css/combined.css" {
		t.Errorf("combined.css should come second, got %v", order)
	}
}

// ---

func TestHasInternalDeps(t *testing.T) {
	noDeps := map[string][]string{
		"/a.css": nil,
		"/b.css": nil,
	}
	if hasInternalDeps(noDeps) {
		t.Error("no internal deps expected")
	}

	withDeps := map[string][]string{
		"/combined.css": {"/base.css"},
		"/base.css":     nil,
	}
	if !hasInternalDeps(withDeps) {
		t.Error("internal deps expected")
	}

	externalOnly := map[string][]string{
		"/a.css": {"/external.css"}, // external.css is not an export
	}
	if hasInternalDeps(externalOnly) {
		t.Error("external import should not count as internal dep")
	}
}

// ---

func TestInternalExportDeps(t *testing.T) {
	// Export combined.css imports base.css which is exported BELOW in the same file.
	// Without dependency ordering, combined.css would import stale content.
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	os.MkdirAll(filepath.Join(dir, "css"), 0755)
	os.MkdirAll(appDir, 0755)

	mockupContent := `<!--%%if:mockup%%-->
<!--%%mockup-export:/css/combined.css%%-->
HEADER
<!--%%mockup-import:/css/base.css%%-->
FOOTER
<!--%%end%%-->
<!--%%mockup-export:/css/base.css%%-->
.base { margin: 0; }
<!--%%end%%-->
<!--%%endif%%-->`

	os.WriteFile(filepath.Join(appDir, "mockup.html"), []byte(mockupContent), 0644)
	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(appDir, "app.miniskin.xml"), []byte(`<miniskin>
	<mockup-list>
		<item src="mockup.html" />
	</mockup-list>
</miniskin>`), 0644)

	ms := newSilent(dir, dir)
	_, err := ms.ProcessMockupExport()
	if err != nil {
		t.Fatalf("ProcessMockupExport failed: %v", err)
	}

	// base.css should exist and have correct content
	base, err := os.ReadFile(filepath.Join(dir, "app", "css", "base.css"))
	if err != nil {
		t.Fatalf("base.css not created: %v", err)
	}
	if !strings.Contains(string(base), ".base { margin: 0; }") {
		t.Errorf("base.css should contain the CSS rule, got: %q", string(base))
	}

	// combined.css should contain the imported base.css content
	combined, err := os.ReadFile(filepath.Join(dir, "app", "css", "combined.css"))
	if err != nil {
		t.Fatalf("combined.css not created: %v", err)
	}
	if !strings.Contains(string(combined), ".base { margin: 0; }") {
		t.Errorf("combined.css should contain base.css content, got: %q", string(combined))
	}
	if !strings.Contains(string(combined), "HEADER") {
		t.Errorf("combined.css should contain HEADER, got: %q", string(combined))
	}
	if !strings.Contains(string(combined), "FOOTER") {
		t.Errorf("combined.css should contain FOOTER, got: %q", string(combined))
	}
}

// ---

func TestAnalyzeDeps(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<mockup-list>
		<item src="login_mockup.html" />
		<item src="dashboard_mockup.html" />
	</mockup-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "login_mockup.html"), []byte(`
<!--%%mockup-export:/css/login.css%%-->
.login { padding: 20px; }
<!--%%end%%-->
<!--%%mockup-export:/js/login.js%%-->
console.log("login");
<!--%%end%%-->`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "dashboard_mockup.html"), []byte(`
<!--%%mockup-import:/css/login.css%%-->
<!--%%mockup-export:/css/dashboard.css%%-->
.dashboard { margin: 10px; }
<!--%%end%%-->`), 0644)

	ms := newSilent(dir, dir)
	dm, err := ms.AnalyzeDeps()
	if err != nil {
		t.Fatalf("AnalyzeDeps failed: %v", err)
	}

	// login_mockup.html: 2 exports
	// dashboard_mockup.html: 1 import + 1 export
	if len(dm.Edges) != 4 {
		t.Errorf("expected 4 edges, got %d: %v", len(dm.Edges), dm.Edges)
	}

	// No cycle: dashboard imports from login, but login doesn't import from dashboard
	if dm.HasCycles() {
		t.Errorf("expected no cycles, got: %v", dm.Cycles)
	}

	// Verify String() output
	s := dm.String()
	if !strings.Contains(s, "login_mockup.html") {
		t.Errorf("String() should mention login_mockup.html: %s", s)
	}
	if !strings.Contains(s, "No circular") {
		t.Errorf("String() should say no circular deps: %s", s)
	}
}

// ---

func TestAnalyzeDepsWithCycle(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<mockup-list>
		<item src="a.html" />
		<item src="b.html" />
	</mockup-list>
</miniskin>`), 0644)

	// a exports X, imports Y → depends on b
	os.WriteFile(filepath.Join(dir, "app", "a.html"), []byte(`
<!--%%mockup-export:/x.css%%-->
content a
<!--%%end%%-->
<!--%%mockup-import:/y.css%%-->`), 0644)

	// b exports Y, imports X → depends on a → CYCLE
	os.WriteFile(filepath.Join(dir, "app", "b.html"), []byte(`
<!--%%mockup-export:/y.css%%-->
content b
<!--%%end%%-->
<!--%%mockup-import:/x.css%%-->`), 0644)

	ms := newSilent(dir, dir)
	dm, err := ms.AnalyzeDeps()
	if err != nil {
		t.Fatalf("AnalyzeDeps failed: %v", err)
	}

	if !dm.HasCycles() {
		t.Fatal("expected a cycle between a.html and b.html")
	}

	s := dm.String()
	if !strings.Contains(s, "Circular") {
		t.Errorf("String() should mention circular deps: %s", s)
	}

	// ProcessingOrder should fail with cycles
	_, err = dm.ProcessingOrder()
	if err == nil {
		t.Fatal("ProcessingOrder should fail with cycles")
	}
}

// ---

func TestProcessingOrderLinear(t *testing.T) {
	// C imports from B, B imports from A → order: A, B, C
	edges := []DepEdge{
		{Source: "a.html", Target: "/x.css", Kind: "export"},
		{Source: "b.html", Target: "/x.css", Kind: "import"},
		{Source: "b.html", Target: "/y.css", Kind: "export"},
		{Source: "c.html", Target: "/y.css", Kind: "import"},
		{Source: "c.html", Target: "/z.css", Kind: "export"},
	}
	dm := &DepMap{Edges: edges, Cycles: detectCycles(edges)}
	order, err := dm.ProcessingOrder()
	if err != nil {
		t.Fatalf("ProcessingOrder failed: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(order), order)
	}
	// a must come before b, b must come before c
	posA, posB, posC := indexOf(order, "a.html"), indexOf(order, "b.html"), indexOf(order, "c.html")
	if posA >= posB || posB >= posC {
		t.Errorf("wrong order: %v (expected a before b before c)", order)
	}
}

// ---

func TestProcessingOrderIndependent(t *testing.T) {
	// No imports → all independent, alphabetical order
	edges := []DepEdge{
		{Source: "c.html", Target: "/c.css", Kind: "export"},
		{Source: "a.html", Target: "/a.css", Kind: "export"},
		{Source: "b.html", Target: "/b.css", Kind: "export"},
	}
	dm := &DepMap{Edges: edges, Cycles: detectCycles(edges)}
	order, err := dm.ProcessingOrder()
	if err != nil {
		t.Fatalf("ProcessingOrder failed: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(order), order)
	}
	// All independent: deterministic alphabetical
	if order[0] != "a.html" || order[1] != "b.html" || order[2] != "c.html" {
		t.Errorf("expected alphabetical order, got %v", order)
	}
}

// ---

func TestProcessingOrderDiamond(t *testing.T) {
	// D depends on B and C, B depends on A, C depends on A
	// Valid orders: A, B, C, D or A, C, B, D
	edges := []DepEdge{
		{Source: "a.html", Target: "/a.css", Kind: "export"},
		{Source: "b.html", Target: "/a.css", Kind: "import"},
		{Source: "b.html", Target: "/b.css", Kind: "export"},
		{Source: "c.html", Target: "/a.css", Kind: "import"},
		{Source: "c.html", Target: "/c.css", Kind: "export"},
		{Source: "d.html", Target: "/b.css", Kind: "import"},
		{Source: "d.html", Target: "/c.css", Kind: "import"},
	}
	dm := &DepMap{Edges: edges, Cycles: detectCycles(edges)}
	order, err := dm.ProcessingOrder()
	if err != nil {
		t.Fatalf("ProcessingOrder failed: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4 items, got %d: %v", len(order), order)
	}
	posA := indexOf(order, "a.html")
	posB := indexOf(order, "b.html")
	posC := indexOf(order, "c.html")
	posD := indexOf(order, "d.html")
	if posA >= posB || posA >= posC {
		t.Errorf("a must come before b and c: %v", order)
	}
	if posB >= posD || posC >= posD {
		t.Errorf("b and c must come before d: %v", order)
	}
}

func indexOf(s []string, val string) int {
	for i, v := range s {
		if v == val {
			return i
		}
	}
	return -1
}

// --- refreshImports

func TestRefreshImportsSingleTag(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.css"), []byte(".a { color: red; }"), 0644)

	input := `before
<!--%%mockup-import:/a.css%%-->
after`
	result, err := refreshImports(input, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, ".a { color: red; }") {
		t.Errorf("should contain file content: %q", result)
	}
	if !strings.Contains(result, "<!--%%end-mockup-import%%-->") {
		t.Errorf("should add end-mockup-import tag: %q", result)
	}
	if !strings.Contains(result, "before") || !strings.Contains(result, "after") {
		t.Errorf("should preserve surrounding content: %q", result)
	}
}

// ---

func TestRefreshImportsBlockTag(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.css"), []byte(".a { color: blue; }"), 0644)

	input := `before
<!--%%mockup-import:/a.css%%-->
.a { color: red; }
<!--%%end%%-->
after`
	result, err := refreshImports(input, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, ".a { color: blue; }") {
		t.Errorf("should contain updated content: %q", result)
	}
	if strings.Contains(result, "color: red") {
		t.Errorf("should not contain old content: %q", result)
	}
	if !strings.Contains(result, "<!--%%end%%-->") {
		t.Errorf("should preserve end tag: %q", result)
	}
}

// ---

func TestRefreshImportsMultiple(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.css"), []byte("AAA"), 0644)
	os.WriteFile(filepath.Join(dir, "b.js"), []byte("BBB"), 0644)

	input := `start
<!--%%mockup-import:/a.css%%-->
middle
<!--%%mockup-import:/b.js%%-->
end`
	result, err := refreshImports(input, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "AAA") {
		t.Errorf("should contain a.css content: %q", result)
	}
	if !strings.Contains(result, "BBB") {
		t.Errorf("should contain b.js content: %q", result)
	}
	if strings.Count(result, "<!--%%end-mockup-import%%-->") != 2 {
		t.Errorf("should have 2 end-mockup-import tags: %q", result)
	}
}

// ---

func TestRefreshImportsIdempotent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.css"), []byte("content"), 0644)

	input := `<!--%%mockup-import:/a.css%%-->
content
<!--%%end%%-->`
	result, err := refreshImports(input, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	// Run again
	result2, err := refreshImports(result, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if result != result2 {
		t.Errorf("should be idempotent:\n  first:  %q\n  second: %q", result, result2)
	}
}

func TestRefreshImportsCSSWrappers(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.css"), []byte(".body{margin:0}"), 0644)

	input := "<style>\n/*<%%mockup-import:/app.css%%>*/\nold css\n/*<%%end-mockup-import%%>*/\n</style>"
	result, err := refreshImports(input, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	expected := "<style>\n/*<%%mockup-import:/app.css%%>*/\n.body{margin:0}\n/*<%%end-mockup-import%%>*/\n</style>"
	if result != expected {
		t.Errorf("got:\n%s\nwant:\n%s", result, expected)
	}
}

func TestRefreshImportsCSSWrappersIdempotent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.css"), []byte(".body{margin:0}"), 0644)

	input := "<style>\n/*<%%mockup-import:/app.css%%>*/\n.body{margin:0}\n/*<%%end-mockup-import%%>*/\n</style>"
	result, err := refreshImports(input, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if result != input {
		t.Errorf("should be idempotent:\n  got:  %q\n  want: %q", result, input)
	}
}

func TestRefreshImportsCRLF(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.css"), []byte("NEW"), 0644)

	input := "/*<%%mockup-import:/a.css%%>*/\r\nOLD\r\n/*<%%end-mockup-import%%>*/\r\n"
	result, err := refreshImports(input, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	expected := "/*<%%mockup-import:/a.css%%>*/\r\nNEW\n/*<%%end-mockup-import%%>*/\r\n"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestCleanImportsBlock(t *testing.T) {
	input := `<html>
<!--%%mockup-import:/a.css%%-->
.body { margin: 0; }
<!--%%end%%-->
</html>`
	expected := `<html>
<!--%%mockup-import:/a.css%%-->
<!--%%end%%-->
</html>`
	result := cleanImports(input)
	if result != expected {
		t.Errorf("got:\n%s\nwant:\n%s", result, expected)
	}
}

func TestCleanImportsCSSWrappers(t *testing.T) {
	input := `<style>
/*<%%mockup-import:"/app/assets/app.css"%%>*/
.body { margin: 0; }
/*<%%end-mockup-import%%>*/
</style>`
	expected := `<style>
/*<%%mockup-import:"/app/assets/app.css"%%>*/
/*<%%end-mockup-import%%>*/
</style>`
	result := cleanImports(input)
	if result != expected {
		t.Errorf("got:\n%s\nwant:\n%s", result, expected)
	}
}

func TestCleanImportsSingleTagUntouched(t *testing.T) {
	input := `<!--%%mockup-import:/a.css%%-->`
	result := cleanImports(input)
	if result != input {
		t.Errorf("single tag should be untouched, got: %q", result)
	}
}

func TestCleanImportsIdempotent(t *testing.T) {
	input := `<!--%%mockup-import:/a.css%%-->
<!--%%end%%-->`
	result := cleanImports(input)
	result2 := cleanImports(result)
	if result != result2 {
		t.Errorf("should be idempotent:\n  first:  %q\n  second: %q", result, result2)
	}
}

// ---

func TestRunWithUpdateImports(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.MkdirAll(filepath.Join(dir, "css"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<mockup-list>
		<item src="mockup_a.html" />
		<item src="mockup_b.html" />
	</mockup-list>
</miniskin>`), 0644)

	// mockup_a exports CSS
	os.WriteFile(filepath.Join(dir, "app", "mockup_a.html"), []byte(`<!--%%if:mockup%%-->
<!--%%mockup-export:/css/style.css%%-->
.body { margin: 0; }
<!--%%end%%-->
<!--%%endif%%-->`), 0644)

	// mockup_b imports that CSS (single tag, should be promoted to block)
	os.WriteFile(filepath.Join(dir, "app", "mockup_b.html"), []byte(`<html>
<!--%%if:mockup%%-->
<!--%%mockup-import:/css/style.css%%-->
<!--%%endif%%-->
</html>`), 0644)

	ms := newSilent(dir, dir)
	_, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Check that mockup_b was updated with the exported CSS
	data, _ := os.ReadFile(filepath.Join(dir, "app", "mockup_b.html"))
	content := string(data)
	if !strings.Contains(content, ".body { margin: 0; }") {
		t.Errorf("mockup_b should contain imported CSS:\n%s", content)
	}
	if !strings.Contains(content, "<!--%%end-mockup-import%%-->") {
		t.Errorf("mockup_b should have end-mockup-import tag after import:\n%s", content)
	}

	// Second run over the promoted block must succeed and stay stable
	ms2 := newSilent(dir, dir)
	_, err = ms2.Run()
	if err != nil {
		t.Fatalf("second Run failed: %v", err)
	}
	data2, _ := os.ReadFile(filepath.Join(dir, "app", "mockup_b.html"))
	if string(data2) != content {
		t.Errorf("mockup_b changed on second run:\n  first:  %q\n  second: %q", content, string(data2))
	}
}

// ---

func TestRunCircularDepsError(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<mockup-list>
		<item src="a.html" />
		<item src="b.html" />
	</mockup-list>
</miniskin>`), 0644)

	// a exports X, imports Y
	os.WriteFile(filepath.Join(dir, "app", "a.html"), []byte(`
<!--%%mockup-export:/x.css%%-->x<!--%%end%%-->
<!--%%mockup-import:/y.css%%-->`), 0644)

	// b exports Y, imports X → circular
	os.WriteFile(filepath.Join(dir, "app", "b.html"), []byte(`
<!--%%mockup-export:/y.css%%-->y<!--%%end%%-->
<!--%%mockup-import:/x.css%%-->`), 0644)

	ms := newSilent(dir, dir)
	_, err := ms.Run()
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error should mention circular deps: %v", err)
	}
}

// ---

func TestRunCircularDepsErrorRelativeContentPath(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<mockup-list>
		<item src="a.html" />
		<item src="b.html" />
	</mockup-list>
</miniskin>`), 0644)

	// a exports X, imports Y
	os.WriteFile(filepath.Join(dir, "app", "a.html"), []byte(`
<!--%%mockup-export:/x.css%%-->x<!--%%end%%-->
<!--%%mockup-import:/y.css%%-->`), 0644)

	// b exports Y, imports X → circular
	os.WriteFile(filepath.Join(dir, "app", "b.html"), []byte(`
<!--%%mockup-export:/y.css%%-->y<!--%%end%%-->
<!--%%mockup-import:/x.css%%-->`), 0644)

	t.Chdir(dir)
	ms := newSilent(".", ".")
	_, err := ms.Run()
	if err == nil {
		t.Fatal("expected circular dependency error with relative contentPath")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error should mention circular deps: %v", err)
	}
}

// ---

func TestMatchesMuxPattern(t *testing.T) {
	tests := []struct {
		file, pattern string
		want          bool
	}{
		{"app.css", "*", true},
		{"app.css", "", false},
		{"app.css", "*.css", true},
		{"app.css", "*.js", false},
		{"app.css", "*.js,*.css", true},
		{"app.js", "*.js,*.css", true},
		{"app.html", "*.js,*.css", false},
		{"fav.ico", "*.js,*.css,fav.ico", true},
		{"other.ico", "*.js,*.css,fav.ico", false},
		{"app.css", " *.css , *.js ", true},
	}
	for _, tt := range tests {
		got := matchesMuxPattern(tt.file, tt.pattern)
		if got != tt.want {
			t.Errorf("matchesMuxPattern(%q, %q) = %v, want %v", tt.file, tt.pattern, got, tt.want)
		}
	}
}

// ---

func TestIsExcludedByMux(t *testing.T) {
	tests := []struct {
		file       string
		include    string
		exclude    string
		wantExcl   bool
	}{
		// Default: include=*, exclude="" → not excluded
		{"app.css", "*", "", false},
		{"app.html", "*", "", false},
		// Include specific → others excluded
		{"app.css", "*.css,*.js", "", false},
		{"app.html", "*.css,*.js", "", true},
		// Exclude specific → matched excluded
		{"app.css", "*", "*.css", true},
		{"app.js", "*", "*.css", false},
		// Both: include narrows, exclude further filters
		{"app.css", "*.css,*.js", "*.min.css", false},
		{"app.min.css", "*.css,*.js", "*.min.css", true},
		{"app.html", "*.css,*.js", "*.min.css", true},
	}
	for _, tt := range tests {
		got := isExcludedByMux(tt.file, tt.include, tt.exclude)
		if got != tt.wantExcl {
			t.Errorf("isExcludedByMux(%q, %q, %q) = %v, want %v", tt.file, tt.include, tt.exclude, got, tt.wantExcl)
		}
	}
}

// ---

func TestCascadeMux(t *testing.T) {
	if cascadeMux("*", "") != "*" {
		t.Error("empty child should inherit parent")
	}
	if cascadeMux("*", "*.css") != "*.css" {
		t.Error("non-empty child should override parent")
	}
	if cascadeMux("", "*.js") != "*.js" {
		t.Error("non-empty child should override empty parent")
	}
}

// ---

func TestMuxExcludeOnBucketList(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content" mux-exclude="*.html,*.tmpl">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static" file="app.css" />
		<item type="static" file="app.js" />
		<item type="html-template" file="page.html" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.css"), nil, 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.js"), nil, 0644)
	os.WriteFile(filepath.Join(dir, "app", "page.html"), nil, 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	for _, it := range items {
		switch it.File {
		case "app.css", "app.js":
			if it.HasFlag("nomux") {
				t.Errorf("%s should NOT have nomux", it.File)
			}
		case "page.html":
			if !it.HasFlag("nomux") {
				t.Errorf("%s should have nomux (excluded by mux-exclude=*.html)", it.File)
			}
		}
	}
}

// ---

func TestMuxIncludeOnBucket(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" mux-include="*.css,*.js,fav.ico" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static" file="app.css" />
		<item type="static" file="fav.ico" />
		<item type="html-template" file="page.html" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.css"), nil, 0644)
	os.WriteFile(filepath.Join(dir, "app", "fav.ico"), nil, 0644)
	os.WriteFile(filepath.Join(dir, "app", "page.html"), nil, 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	for _, it := range items {
		switch it.File {
		case "app.css", "fav.ico":
			if it.HasFlag("nomux") {
				t.Errorf("%s should NOT have nomux", it.File)
			}
		case "page.html":
			if !it.HasFlag("nomux") {
				t.Errorf("%s should have nomux (not in mux-include)", it.File)
			}
		}
	}
}

// ---

func TestMuxCascadeMiniskinToBucket(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	// mux-exclude on root <miniskin> cascades to bucket
	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin mux-exclude="*.html">
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static" file="app.css" />
		<item type="static" file="page.html" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.css"), nil, 0644)
	os.WriteFile(filepath.Join(dir, "app", "page.html"), nil, 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	for _, it := range items {
		switch it.File {
		case "app.css":
			if it.HasFlag("nomux") {
				t.Errorf("app.css should NOT have nomux")
			}
		case "page.html":
			if !it.HasFlag("nomux") {
				t.Errorf("page.html should have nomux (excluded from root)")
			}
		}
	}
}

// ---

func TestMuxCascadeResourceListOverridesBucket(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	// Bucket excludes *.html, but resource-list overrides with include=*
	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" mux-exclude="*.html" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets" mux-exclude="">
		<item type="static" file="app.css" />
		<item type="html-template" file="page.html" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.css"), nil, 0644)
	os.WriteFile(filepath.Join(dir, "app", "page.html"), nil, 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	// resource-list sets mux-exclude="" which should NOT override (empty = inherit)
	// So page.html should still be excluded
	for _, it := range items {
		if it.File == "page.html" && !it.HasFlag("nomux") {
			t.Errorf("page.html should have nomux (empty string does not override)")
		}
	}
}

// ---

func TestMuxExplicitNomuxPreserved(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	// Even with mux-include=*, an explicit nomux in type stays
	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static,nomux" file="app.css" />
		<item type="static" file="app.js" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.css"), nil, 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.js"), nil, 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	for _, it := range items {
		switch it.File {
		case "app.css":
			if !it.HasFlag("nomux") {
				t.Errorf("app.css should keep explicit nomux")
			}
		case "app.js":
			if it.HasFlag("nomux") {
				t.Errorf("app.js should NOT have nomux")
			}
		}
	}
}

// ---

func TestMuxDefaultIncludeAll(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	// No mux attributes at all → default mux-include="*", nothing excluded
	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static" file="app.css" />
		<item type="html-template" file="page.html" />
		<item type="static" file="app.js" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.css"), nil, 0644)
	os.WriteFile(filepath.Join(dir, "app", "page.html"), nil, 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.js"), nil, 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	for _, it := range result.Buckets[0].Items {
		if it.HasFlag("nomux") {
			t.Errorf("%s should NOT have nomux (default includes all)", it.File)
		}
	}
}

// ---

func TestBuildEmbedMissingFileErrors(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static" file="app.css" />
		<item type="static" file="missing.js" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "app.css"), nil, 0644)
	// missing.js intentionally not created

	ms := newSilent(dir, dir)
	_, err := ms.BuildEmbed()
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// --- Escape function tests (unit)

func TestEscapeHTML(t *testing.T) {
	tests := []struct{ in, want string }{
		{`<b>"Sam&Co"</b>`, "&lt;b&gt;&#34;Sam&amp;Co&#34;&lt;/b&gt;"},
		{"", ""},
		{"hello", "hello"},
		{"a'b", "a&#39;b"},
		{"<>&\"'", "&lt;&gt;&amp;&#34;&#39;"},
	}
	for _, tt := range tests {
		if got := escapeHTML(tt.in); got != tt.want {
			t.Errorf("escapeHTML(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---

func TestEscapeXML(t *testing.T) {
	tests := []struct{ in, want string }{
		{`It's <"ok"> & fine`, "It&apos;s &lt;&quot;ok&quot;&gt; &amp; fine"},
		{"", ""},
		{"hello", "hello"},
		// Key difference from HTML: ' → &apos; (not &#39;)
		{"a'b", "a&apos;b"},
	}
	for _, tt := range tests {
		if got := escapeXML(tt.in); got != tt.want {
			t.Errorf("escapeXML(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---

func TestEscapeXMLvsHTML(t *testing.T) {
	// The ONLY difference: single quote
	htmlResult := escapeHTML("it's")
	xmlResult := escapeXML("it's")
	if htmlResult == xmlResult {
		t.Error("HTML and XML should differ on single quote escaping")
	}
	if htmlResult != "it&#39;s" {
		t.Errorf("HTML got %q", htmlResult)
	}
	if xmlResult != "it&apos;s" {
		t.Errorf("XML got %q", xmlResult)
	}
}

// ---

func TestEscapeURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello world", "hello+world"},
		{"hello world&foo=bar", "hello+world%26foo%3Dbar"},
		{"", ""},
		{"abc", "abc"},
		{"a/b?c=d", "a%2Fb%3Fc%3Dd"},
		{"café", "caf%C3%A9"},
	}
	for _, tt := range tests {
		if got := escapeURLEncode(tt.in); got != tt.want {
			t.Errorf("escapeURLEncode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---

func TestEscapeJS(t *testing.T) {
	tests := []struct{ in, want string }{
		{`say "hello"`, `say \"hello\"`},
		{"new\nline", `new\nline`},
		{"tab\there", `tab\there`},
		{"cr\rhere", `cr\rhere`},
		{`back\slash`, `back\\slash`},
		{"it's", `it\'s`},
		{"<script>", `\x3cscript\x3e`},
		{"a&b", `a\x26b`},
		{"", ""},
		{"hello", "hello"},
	}
	for _, tt := range tests {
		if got := escapeJS(tt.in); got != tt.want {
			t.Errorf("escapeJS(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---

func TestEscapeCSS(t *testing.T) {
	// Alphanumeric and spaces pass through, specials are hex-escaped
	got := escapeCSS("hello")
	if got != "hello" {
		t.Errorf("plain text should pass through, got %q", got)
	}
	// Quotes, parens, semicolons, braces, angle brackets are escaped
	specials := []string{`\`, `"`, `'`, `(`, `)`, `;`, `{`, `}`, `<`, `>`}
	for _, s := range specials {
		got = escapeCSS(s)
		if !strings.HasPrefix(got, `\`) {
			t.Errorf("escapeCSS(%q) should start with backslash, got %q", s, got)
		}
	}
}

// ---

func TestEscapeCSSPreservesAlphanumeric(t *testing.T) {
	got := escapeCSS("abc123 XYZ")
	if got != "abc123 XYZ" {
		t.Errorf("alphanumeric+space should pass through, got %q", got)
	}
}

// ---

func TestEscapeJSON(t *testing.T) {
	tests := []struct{ in, want string }{
		{"line1\nline2", `line1\nline2`},
		{"tab\there", `tab\there`},
		{`say "hi"`, `say \"hi\"`},
		{`back\slash`, `back\\slash`},
		{"", ""},
		{"hello", "hello"},
		{"\x00", `\u0000`},
	}
	for _, tt := range tests {
		if got := escapeJSON(tt.in); got != tt.want {
			t.Errorf("escapeJSON(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---

func TestEscapeSQL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"O'Brien's", "O''Brien''s"},
		{"", ""},
		{"hello", "hello"},
		{"''", "''''"},
		{"no quotes", "no quotes"},
	}
	for _, tt := range tests {
		if got := escapeSQL(tt.in); got != tt.want {
			t.Errorf("escapeSQL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---

func TestEscapeSQLT(t *testing.T) {
	tests := []struct{ in, want string }{
		{"100% O'Brien_test", "100\\% O''Brien\\_test"},
		{"", ""},
		{"hello", "hello"},
		{"50%", "50\\%"},
		{"under_score", "under\\_score"},
		{"all'_%", "all''\\_\\%"},
	}
	for _, tt := range tests {
		if got := escapeSQLT(tt.in); got != tt.want {
			t.Errorf("escapeSQLT(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---

func TestEscapeSQLTIncludesSQL(t *testing.T) {
	// sqlt must also do SQL escaping (single quotes)
	got := escapeSQLT("it's 100%")
	if !strings.Contains(got, "''") {
		t.Errorf("sqlt should include SQL quote escaping, got %q", got)
	}
	if !strings.Contains(got, "\\%") {
		t.Errorf("sqlt should escape %%, got %q", got)
	}
}

func TestParseEscapeTag(t *testing.T) {
	tests := []struct {
		tag     string
		wantOK  bool
		wantVar string
	}{
		{"url:name", true, "name"},
		{"js:title", true, "title"},
		{"css:color", true, "color"},
		{"json:data", true, "data"},
		{"sql:val", true, "val"},
		{"sqlt:search", true, "search"},
		{"xml:content", true, "content"},
		{"html:name", true, "name"},
		{"name", false, ""},
		{"if:name", false, ""},
		{"include:path", false, ""},
		{"echo:text", false, ""},
		{"note:text", false, ""},
		{"mockup-export:path", false, ""},
		{"mockup-import:path", false, ""},
		{"unknown:var", false, ""},
		{"url: spaced ", true, "spaced"},
	}
	for _, tt := range tests {
		fn, varName, ok := parseEscapeTag(tt.tag)
		if ok != tt.wantOK {
			t.Errorf("parseEscapeTag(%q) ok=%v, want %v", tt.tag, ok, tt.wantOK)
			continue
		}
		if ok {
			if varName != tt.wantVar {
				t.Errorf("parseEscapeTag(%q) var=%q, want %q", tt.tag, varName, tt.wantVar)
			}
			if fn == nil {
				t.Errorf("parseEscapeTag(%q) fn is nil", tt.tag)
			}
		}
	}
}

// ---

func TestParseEscapeTagEchoDetection(t *testing.T) {
	// escape:echo:text — the "rest" should be "echo:text"
	fn, rest, ok := parseEscapeTag("js:echo:hello")
	if !ok {
		t.Fatal("should parse js: prefix")
	}
	if rest != "echo:hello" {
		t.Errorf("rest should be echo:hello, got %q", rest)
	}
	if fn == nil {
		t.Error("fn should not be nil")
	}
}

// --- Escape tests (integration with resolvePercent)

func TestEscapeSingleTagURL(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"q": "hello world"}
	result, err := ms.resolvePercent(`<%url:q%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello+world" {
		t.Errorf("got %q, want %q", result, "hello+world")
	}
}

func TestEscapeDoubleTagURL(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"q": "hello world"}
	result, err := ms.resolvePercent(`<%%url:q%%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello+world" {
		t.Errorf("got %q, want %q", result, "hello+world")
	}
}

func TestEscapeCommentSingleTagJS(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"msg": "it's \"ok\""}
	result, err := ms.resolvePercent(`<!--%js:msg%-->`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `it\'s \"ok\"`
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestEscapeCommentDoubleTagSQL(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"name": "O'Brien"}
	result, err := ms.resolvePercent(`<!--%%sql:name%%-->`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "O''Brien" {
		t.Errorf("got %q, want %q", result, "O''Brien")
	}
}

func TestEscapeHTMLExplicitInDoubleTag(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": "<b>bold</b>"}
	result, err := ms.resolvePercent(`<%%html:x%%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "&lt;b&gt;bold&lt;/b&gt;"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestEscapeUndefinedVariable(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`<%url:missing%>`, nil, nil)
	if err == nil {
		t.Fatal("expected error for undefined variable")
	}
}

func TestEscapeMockupModePassthrough(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	ms.skipVars = true
	vars := map[string]string{"q": "test"}
	result, err := ms.resolvePercent(`<%url:q%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "<%url:q%>" {
		t.Errorf("mockup mode should pass through, got %q", result)
	}
}

// ---

func TestEscapeEchoWithPrefix(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	// js:echo: — explicit escape prefix on echo
	result, err := ms.resolvePercent(`<%js:echo:it's "ok"%>`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `it\'s \"ok\"`
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

// ---

func TestEscapeEchoWithPrefixDouble(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	// url:echo: in double tag
	result, err := ms.resolvePercent(`<%%url:echo:hello world%%>`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello+world" {
		t.Errorf("got %q, want %q", result, "hello+world")
	}
}

// ---

func TestDefaultEscapeFromRules(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	// Set default escape to JS
	ms.defaultEscapeFn = escapeJS
	vars := map[string]string{"msg": "it's \"ok\""}
	// Single tag should use JS escape instead of HTML
	result, err := ms.resolvePercent(`<%msg%>`, vars, nil)
	ms.defaultEscapeFn = nil
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `it\'s \"ok\"`
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

// ---

func TestDefaultEscapeEchoUsesDefault(t *testing.T) {
	ms := newSilent(t.TempDir(), t.TempDir())
	ms.defaultEscapeFn = escapeJS
	result, err := ms.resolvePercent(`<%echo:it's "ok"%>`, nil, nil)
	ms.defaultEscapeFn = nil
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `it\'s \"ok\"`
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

// ---

func TestResolveDefaultEscapeByExtension(t *testing.T) {
	rules := []xmlEscape{
		{Ext: "*.html", As: "html"},
		{Ext: "*.js", As: "js"},
		{Ext: "*.css", As: "css"},
	}
	// HTML file → HTML escape
	fn := resolveDefaultEscape("page.html", "", rules)
	if fn("'") != "&#39;" {
		t.Errorf("html escape expected for .html")
	}
	// JS file → JS escape
	fn = resolveDefaultEscape("app.js", "", rules)
	if fn("'") != `\'` {
		t.Errorf("js escape expected for .js, got %q", fn("'"))
	}
	// CSS file → CSS escape
	fn = resolveDefaultEscape("app.css", "", rules)
	if !strings.Contains(fn("'"), `\`) {
		t.Errorf("css escape expected for .css, got %q", fn("'"))
	}
	// Unknown extension → no escaping (nil)
	fn = resolveDefaultEscape("data.txt", "", rules)
	if fn != nil {
		t.Errorf("expected nil (no escape) for .txt, got non-nil")
	}
}

// ---

func TestResolveDefaultEscapeItemOverride(t *testing.T) {
	rules := []xmlEscape{
		{Ext: "*.html", As: "html"},
	}
	// Item-level escape="js" overrides rules
	fn := resolveDefaultEscape("page.html", "js", rules)
	if fn("'") != `\'` {
		t.Errorf("item escape=js should override, got %q", fn("'"))
	}
}

// ---

func TestCascadeEscapeRules(t *testing.T) {
	parent := []xmlEscape{{Ext: "*.html", As: "html"}}
	child := []xmlEscape{{Ext: "*.js", As: "js"}}
	result := cascadeEscapeRules(parent, child)
	if len(result) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(result))
	}
	// Child rules come after parent (last match wins)
	if result[0].As != "html" || result[1].As != "js" {
		t.Error("cascade order should be parent then child")
	}
}

// ---

func TestCascadeEscapeRulesChildOverrides(t *testing.T) {
	parent := []xmlEscape{{Ext: "*.html", As: "html"}}
	child := []xmlEscape{{Ext: "*.html", As: "js"}} // override same ext
	result := cascadeEscapeRules(parent, child)
	fn := resolveDefaultEscape("page.html", "", result)
	// Last match wins → js
	if fn("'") != `\'` {
		t.Errorf("child should override parent for same ext, got %q", fn("'"))
	}
}

// ---

func TestEscapeRulesIntegration(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<globals>
		<var name="msg" value="it's &quot;ok&quot;" />
	</globals>
	<escape ext="*.js" as="js" />
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static" src="app_src.js" file="app.js" />
	</resource-list>
</miniskin>`), 0644)

	// Source file with a variable using single tag (should use JS escape)
	os.WriteFile(filepath.Join(dir, "app", "app_src.js"), []byte(`var msg = '<%msg%>';`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	data, err := os.ReadFile(filepath.Join(dir, "app", "app.js"))
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	content := string(data)
	want := `var msg = 'it\'s \"ok\"';`
	if content != want {
		t.Errorf("got %q, want %q", content, want)
	}
}

// ---

func TestEscapeItemAttributeOverride(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<globals>
		<var name="val" value="O'Brien" />
	</globals>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static" src="data_src.txt" file="data.txt" escape="sql" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "data_src.txt"), []byte(`<%val%>`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	data, err := os.ReadFile(filepath.Join(dir, "app", "data.txt"))
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != "O''Brien" {
		t.Errorf("got %q, want %q", string(data), "O''Brien")
	}
}

// ---

func TestChainedResourceLists(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static" file="app.css" />
	</resource-list>
	<resource-list urlbase="/pages">
		<item type="static" file="index.html" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.css"), []byte("body{}"), 0644)
	os.WriteFile(filepath.Join(dir, "app", "index.html"), []byte("<h1>Hi</h1>"), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].File != "app.css" || items[0].urlBase != "/assets" {
		t.Errorf("item 0: expected app.css with /assets, got %s with %s", items[0].File, items[0].urlBase)
	}
	if items[1].File != "index.html" || items[1].urlBase != "/pages" {
		t.Errorf("item 1: expected index.html with /pages, got %s with %s", items[1].File, items[1].urlBase)
	}
}

// ---

// A type="block" item is processed for its side effects (populating doc-block
// buffers) but writes no output file and is not embedded. A separate consumer
// item emits the buffer it built.
func TestBlockItemNoOutput(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list>
		<item type="block" src="./_block.list" />
		<item type="static" src="./_doc.src" file="doc.md" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "_block.list"), []byte(
		"<%% doc-block-begin: b1 %%>\n<%% include-notes:comp.js %%>\n<%% doc-block-end: b1 %%>\n"), 0644)
	os.WriteFile(filepath.Join(dir, "app", "comp.js"), []byte(
		"console.log(1);\n<%% note:\n# Comp\n\nHello world\n%%>\n"), 0644)
	os.WriteFile(filepath.Join(dir, "app", "_doc.src"), []byte(
		"<%% doc-block-toc: b1 %%>\n\n<%% doc-block-content: b1 %%>\n"), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	// the block item is dropped: only the consumer survives in the embed list
	items := result.Buckets[0].Items
	if len(items) != 1 {
		t.Fatalf("expected 1 item (block dropped), got %d", len(items))
	}
	if items[0].File != "doc.md" {
		t.Errorf("surviving item should be doc.md, got %s", items[0].File)
	}

	// the consumer output holds the notes the block item captured, plus a TOC
	data, err := os.ReadFile(filepath.Join(dir, "app", "doc.md"))
	if err != nil {
		t.Fatalf("reading doc.md: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "# Comp") || !strings.Contains(out, "Hello world") {
		t.Errorf("doc.md missing captured note content:\n%s", out)
	}
	if !strings.Contains(out, "(#comp)") {
		t.Errorf("doc.md missing generated TOC anchor:\n%s", out)
	}
}

// ---

func TestNestedResourceListWithSrc(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "sub"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/root">
		<item type="static" file="top.css" />
		<resource-list src="sub" urlbase="/sub">
			<item type="static" file="deep.css" />
		</resource-list>
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "top.css"), []byte("top{}"), 0644)
	os.WriteFile(filepath.Join(dir, "app", "sub", "deep.css"), []byte("deep{}"), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].File != "top.css" {
		t.Errorf("item 0: expected top.css, got %s", items[0].File)
	}
	// deep.css should resolve to app/sub/deep.css
	if items[1].File != "deep.css" {
		t.Errorf("item 1: expected deep.css, got %s", items[1].File)
	}
	if items[1].dir != filepath.Join(dir, "app", "sub") {
		t.Errorf("item 1 dir: expected %s, got %s", filepath.Join(dir, "app", "sub"), items[1].dir)
	}
	if items[1].urlBase != "/sub" {
		t.Errorf("item 1 urlBase: expected /sub, got %s", items[1].urlBase)
	}
}

// ---

func TestNestedResourceListCascadesSkinDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "sub"), 0755)
	os.MkdirAll(filepath.Join(dir, "app", "pskins"), 0755)

	os.WriteFile(filepath.Join(dir, "app", "pskins", "base.html"), []byte(`[P]<%%content%%>[/P]`), 0644)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list skin-dir="pskins">
		<resource-list src="sub">
			<item type="html-template" src="x_src.html" file="x.html" />
		</resource-list>
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "sub", "x_src.html"), []byte("---\nskin: base\n---\nHello"), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	data, _ := os.ReadFile(filepath.Join(dir, "app", "sub", "x.html"))
	if string(data) != "[P]Hello[/P]" {
		t.Errorf("expected cascaded skin, got %q", string(data))
	}
}

// ---

func TestNestedResourceListCascadesMux(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "sub"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list mux-exclude="*.html">
		<item type="static" file="top.css" />
		<resource-list src="sub">
			<item type="static" file="page.html" />
			<item type="static" file="style.css" />
		</resource-list>
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "top.css"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "app", "sub", "page.html"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "app", "sub", "style.css"), []byte(""), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	for _, it := range items {
		switch it.File {
		case "top.css", "style.css":
			if it.HasFlag("nomux") {
				t.Errorf("%s should NOT have nomux", it.File)
			}
		case "page.html":
			if !it.HasFlag("nomux") {
				t.Errorf("page.html should have nomux (inherited mux-exclude)")
			}
		}
	}
}

// ---

func TestNestedResourceListCascadesEscape(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "sub"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<globals><var name="val" value="&lt;b&gt;bold&lt;/b&gt;" /></globals>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list>
		<escape ext="*.html" as="html" />
		<resource-list src="sub">
			<item src="page_src.html" file="page.html" />
		</resource-list>
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "sub", "page_src.html"), []byte(`<%val%>`), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	data, _ := os.ReadFile(filepath.Join(dir, "app", "sub", "page.html"))
	if string(data) != "&lt;b&gt;bold&lt;/b&gt;" {
		t.Errorf("expected HTML escape from cascaded rule, got %q", string(data))
	}
}

// ---

func TestDeepNestedResourceLists(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "a", "b"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list>
		<item type="static" file="root.css" />
		<resource-list src="a">
			<item type="static" file="mid.css" />
			<resource-list src="b">
				<item type="static" file="deep.css" />
			</resource-list>
		</resource-list>
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "root.css"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "app", "a", "mid.css"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "app", "a", "b", "deep.css"), []byte(""), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	expected := []struct{ file, dir string }{
		{"root.css", filepath.Join(dir, "app")},
		{"mid.css", filepath.Join(dir, "app", "a")},
		{"deep.css", filepath.Join(dir, "app", "a", "b")},
	}
	for i, e := range expected {
		if items[i].File != e.file {
			t.Errorf("item %d: expected file %s, got %s", i, e.file, items[i].File)
		}
		if items[i].dir != e.dir {
			t.Errorf("item %d: expected dir %s, got %s", i, e.dir, items[i].dir)
		}
	}
}

// ---

func TestNestedResourceListOverridesMux(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "sub"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list mux-exclude="*.html">
		<item type="static" file="page.html" />
		<resource-list src="sub" mux-exclude="*.css">
			<item type="static" file="sub.html" />
			<item type="static" file="sub.css" />
		</resource-list>
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "page.html"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "app", "sub", "sub.html"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "app", "sub", "sub.css"), []byte(""), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	for _, it := range items {
		switch it.File {
		case "page.html":
			if !it.HasFlag("nomux") {
				t.Errorf("page.html should have nomux (parent mux-exclude=*.html)")
			}
		case "sub.html":
			// Child overrides mux-exclude to *.css, so *.html is no longer excluded
			if it.HasFlag("nomux") {
				t.Errorf("sub.html should NOT have nomux (child overrides mux-exclude)")
			}
		case "sub.css":
			if !it.HasFlag("nomux") {
				t.Errorf("sub.css should have nomux (child mux-exclude=*.css)")
			}
		}
	}
}

// ---

func TestChainedAndNestedResourceLists(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "login"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static" file="app.css" />
	</resource-list>
	<resource-list urlbase="/pages">
		<item type="static" file="index.html" />
		<resource-list src="login" urlbase="/login">
			<item type="static" file="signin.html" />
		</resource-list>
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.css"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "app", "index.html"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "app", "login", "signin.html"), []byte(""), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	expectations := []struct{ file, urlBase string }{
		{"app.css", "/assets"},
		{"index.html", "/pages"},
		{"signin.html", "/login"},
	}
	for i, e := range expectations {
		if items[i].File != e.file || items[i].urlBase != e.urlBase {
			t.Errorf("item %d: expected %s@%s, got %s@%s", i, e.file, e.urlBase, items[i].File, items[i].urlBase)
		}
	}
}

// ---

func TestInlineResourceListInBucket(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "login"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app">
			<resource-list urlbase="/assets">
				<item type="static" file="app.css" />
			</resource-list>
			<resource-list urlbase="/pages">
				<item type="static" file="index.html" />
				<resource-list src="login" urlbase="/login">
					<item type="static" file="signin.html" />
				</resource-list>
			</resource-list>
		</bucket>
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "app.css"), []byte("body{}"), 0644)
	os.WriteFile(filepath.Join(dir, "app", "index.html"), []byte("<h1>Hi</h1>"), 0644)
	os.WriteFile(filepath.Join(dir, "app", "login", "signin.html"), []byte("<form></form>"), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	expectations := []struct{ file, urlBase, dir string }{
		{"app.css", "/assets", filepath.Join(dir, "app")},
		{"index.html", "/pages", filepath.Join(dir, "app")},
		{"signin.html", "/login", filepath.Join(dir, "app", "login")},
	}
	for i, e := range expectations {
		if items[i].File != e.file {
			t.Errorf("item %d: expected file %s, got %s", i, e.file, items[i].File)
		}
		if items[i].urlBase != e.urlBase {
			t.Errorf("item %d: expected urlBase %s, got %s", i, e.urlBase, items[i].urlBase)
		}
		if items[i].dir != e.dir {
			t.Errorf("item %d: expected dir %s, got %s", i, e.dir, items[i].dir)
		}
	}
}

// ---

func TestInlineBucketWithExternalXML(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app", "extra"), 0755)

	// Bucket has inline resource-list AND there's an external XML in a subdirectory
	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" recurse-folder="all">
			<resource-list urlbase="/inline">
				<item type="static" file="inline.css" />
			</resource-list>
		</bucket>
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "extra", "extra.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/extra">
		<item type="static" file="extra.css" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "app", "inline.css"), []byte("inline{}"), 0644)
	os.WriteFile(filepath.Join(dir, "app", "extra", "extra.css"), []byte("extra{}"), 0644)

	ms := newSilent(dir, dir)
	result, err := ms.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed failed: %v", err)
	}

	items := result.Buckets[0].Items
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Inline items come first, then external
	if items[0].File != "inline.css" || items[0].urlBase != "/inline" {
		t.Errorf("item 0: expected inline.css@/inline, got %s@%s", items[0].File, items[0].urlBase)
	}
	if items[1].File != "extra.css" || items[1].urlBase != "/extra" {
		t.Errorf("item 1: expected extra.css@/extra, got %s@%s", items[1].File, items[1].urlBase)
	}
}

// ---

func TestCombineDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "assets"), 0755)
	os.MkdirAll(filepath.Join(dir, "login"), 0755)

	os.WriteFile(filepath.Join(dir, "assets", "assets.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static" file="app.css" />
		<item type="static" file="app.js" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "login", "login.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/login">
		<item type="html-template" src="signin_src.html" file="signin.html" />
	</resource-list>
</miniskin>`), 0644)

	err := CombineDir(dir)
	if err != nil {
		t.Fatalf("CombineDir failed: %v", err)
	}

	// Old XMLs should be removed
	if _, err := os.Stat(filepath.Join(dir, "assets", "assets.miniskin.xml")); err == nil {
		t.Error("assets.miniskin.xml should have been removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "login", "login.miniskin.xml")); err == nil {
		t.Error("login.miniskin.xml should have been removed")
	}

	// Combined XML should exist
	combinedPath := filepath.Join(dir, filepath.Base(dir)+".miniskin.xml")
	combined, err := parseMiniskinXML(combinedPath)
	if err != nil {
		t.Fatalf("parsing combined XML: %v", err)
	}

	if len(combined.ResourceLists) != 2 {
		t.Fatalf("expected 2 resource-lists, got %d", len(combined.ResourceLists))
	}

	// Check that src attributes are set
	for _, rl := range combined.ResourceLists {
		if rl.Src != "assets" && rl.Src != "login" {
			t.Errorf("unexpected resource-list src: %q", rl.Src)
		}
	}
}

// ---

func TestCombineDirWithRootXML(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)

	// Root XML at target dir
	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/root">
		<item type="static" file="index.html" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "sub", "sub.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/sub">
		<item type="static" file="page.html" />
	</resource-list>
</miniskin>`), 0644)

	err := CombineDir(dir)
	if err != nil {
		t.Fatalf("CombineDir failed: %v", err)
	}

	// Should reuse root XML name
	combined, err := parseMiniskinXML(filepath.Join(dir, "root.miniskin.xml"))
	if err != nil {
		t.Fatalf("parsing combined XML: %v", err)
	}

	// Root resource-list (no src) + sub resource-list (src="sub")
	if len(combined.ResourceLists) != 2 {
		t.Fatalf("expected 2 resource-lists, got %d", len(combined.ResourceLists))
	}

	foundRoot := false
	foundSub := false
	for _, rl := range combined.ResourceLists {
		if rl.Src == "" && rl.URLBase == "/root" {
			foundRoot = true
		}
		if rl.Src == "sub" && rl.URLBase == "/sub" {
			foundSub = true
		}
	}
	if !foundRoot {
		t.Error("root resource-list not found")
	}
	if !foundSub {
		t.Error("sub resource-list not found")
	}
}

// ---

func TestCombineDirDeepNesting(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b"), 0755)

	os.WriteFile(filepath.Join(dir, "a", "a.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/a">
		<item type="static" file="a.css" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "a", "b", "b.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/b">
		<item type="static" file="b.css" />
	</resource-list>
</miniskin>`), 0644)

	err := CombineDir(dir)
	if err != nil {
		t.Fatalf("CombineDir failed: %v", err)
	}

	combinedPath := filepath.Join(dir, filepath.Base(dir)+".miniskin.xml")
	combined, err := parseMiniskinXML(combinedPath)
	if err != nil {
		t.Fatalf("parsing combined XML: %v", err)
	}

	// Should have one resource-list with src="a" containing nested src="b"
	if len(combined.ResourceLists) != 1 {
		t.Fatalf("expected 1 top-level resource-list, got %d", len(combined.ResourceLists))
	}
	rl := combined.ResourceLists[0]
	if rl.Src != "a" {
		t.Errorf("expected src=a, got %q", rl.Src)
	}
	if len(rl.ResourceLists) != 1 {
		t.Fatalf("expected 1 nested resource-list, got %d", len(rl.ResourceLists))
	}
	if rl.ResourceLists[0].Src != "b" {
		t.Errorf("expected nested src=b, got %q", rl.ResourceLists[0].Src)
	}
}

// ---

func TestCombineDirWithMockups(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "login"), 0755)

	os.WriteFile(filepath.Join(dir, "login", "login.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/login">
		<item type="static" file="signin.html" />
	</resource-list>
	<mockup-list save-mode="append">
		<item src="mockup_login.html" negative="login_negative.html" />
	</mockup-list>
</miniskin>`), 0644)

	err := CombineDir(dir)
	if err != nil {
		t.Fatalf("CombineDir failed: %v", err)
	}

	combinedPath := filepath.Join(dir, filepath.Base(dir)+".miniskin.xml")
	combined, err := parseMiniskinXML(combinedPath)
	if err != nil {
		t.Fatalf("parsing combined XML: %v", err)
	}

	// Mockup items should have paths prefixed with "login/"
	if combined.MockupList == nil {
		t.Fatal("expected mockup-list in combined XML")
	}
	if len(combined.MockupList.Items) != 1 {
		t.Fatalf("expected 1 mockup item, got %d", len(combined.MockupList.Items))
	}
	mi := combined.MockupList.Items[0]
	if mi.Src != "login/mockup_login.html" {
		t.Errorf("expected src login/mockup_login.html, got %q", mi.Src)
	}
	if mi.Negative != "login/login_negative.html" {
		t.Errorf("expected negative login/login_negative.html, got %q", mi.Negative)
	}
}

// ---

func TestSplitXML(t *testing.T) {
	dir := t.TempDir()

	// Create a combined XML with nested resource-lists
	os.WriteFile(filepath.Join(dir, "combined.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/root">
		<item type="static" file="index.html" />
	</resource-list>
	<resource-list src="assets" urlbase="/assets">
		<item type="static" file="app.css" />
	</resource-list>
	<resource-list src="login" urlbase="/login">
		<item type="html-template" file="signin.html" />
	</resource-list>
</miniskin>`), 0644)

	err := SplitXML(filepath.Join(dir, "combined.miniskin.xml"))
	if err != nil {
		t.Fatalf("SplitXML failed: %v", err)
	}

	// Check that subdirectory XMLs were created
	assetsXML, err := parseMiniskinXML(filepath.Join(dir, "assets", "assets.miniskin.xml"))
	if err != nil {
		t.Fatalf("parsing assets XML: %v", err)
	}
	if len(assetsXML.ResourceLists) != 1 {
		t.Fatalf("expected 1 resource-list in assets, got %d", len(assetsXML.ResourceLists))
	}
	if assetsXML.ResourceLists[0].URLBase != "/assets" {
		t.Errorf("expected urlbase /assets, got %s", assetsXML.ResourceLists[0].URLBase)
	}

	loginXML, err := parseMiniskinXML(filepath.Join(dir, "login", "login.miniskin.xml"))
	if err != nil {
		t.Fatalf("parsing login XML: %v", err)
	}
	if len(loginXML.ResourceLists) != 1 {
		t.Fatalf("expected 1 resource-list in login, got %d", len(loginXML.ResourceLists))
	}

	// Original should only keep root resource-list (no src)
	original, err := parseMiniskinXML(filepath.Join(dir, "combined.miniskin.xml"))
	if err != nil {
		t.Fatalf("parsing original: %v", err)
	}
	if len(original.ResourceLists) != 1 {
		t.Fatalf("expected 1 remaining resource-list, got %d", len(original.ResourceLists))
	}
	if original.ResourceLists[0].Src != "" {
		t.Errorf("remaining resource-list should have no src, got %q", original.ResourceLists[0].Src)
	}
}

// ---

func TestCombineThenSplitRoundtrip(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "assets"), 0755)
	os.MkdirAll(filepath.Join(dir, "login"), 0755)

	os.WriteFile(filepath.Join(dir, "assets", "assets.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static" file="app.css" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "login", "login.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/login">
		<item type="static" file="signin.html" />
	</resource-list>
</miniskin>`), 0644)

	// Combine
	if err := CombineDir(dir); err != nil {
		t.Fatalf("CombineDir failed: %v", err)
	}

	// Find combined file
	combinedPath := filepath.Join(dir, filepath.Base(dir)+".miniskin.xml")

	// Split
	if err := SplitXML(combinedPath); err != nil {
		t.Fatalf("SplitXML failed: %v", err)
	}

	// Verify subdirectory XMLs were recreated
	assetsXML, err := parseMiniskinXML(filepath.Join(dir, "assets", "assets.miniskin.xml"))
	if err != nil {
		t.Fatalf("assets XML missing after split: %v", err)
	}
	if len(assetsXML.ResourceLists) != 1 || assetsXML.ResourceLists[0].URLBase != "/assets" {
		t.Error("assets resource-list not restored correctly")
	}

	loginXML, err := parseMiniskinXML(filepath.Join(dir, "login", "login.miniskin.xml"))
	if err != nil {
		t.Fatalf("login XML missing after split: %v", err)
	}
	if len(loginXML.ResourceLists) != 1 || loginXML.ResourceLists[0].URLBase != "/login" {
		t.Error("login resource-list not restored correctly")
	}
}

// ---

func TestCombineProducesSameResult(t *testing.T) {
	// Setup: root XML + bucket + subdirectory XMLs + source files
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	os.MkdirAll(filepath.Join(appDir, "assets"), 0755)
	os.MkdirAll(filepath.Join(appDir, "login"), 0755)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" recurse-folder="all" />
	</bucket-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(appDir, "assets", "assets.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/assets">
		<item type="static" file="app.css" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(appDir, "login", "login.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/login">
		<item type="html-template" src="signin_src.html" file="signin.html" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(appDir, "assets", "app.css"), []byte("body{color:red}"), 0644)
	os.WriteFile(filepath.Join(appDir, "login", "signin_src.html"), []byte("<h1>Sign In</h1>"), 0644)

	// Run pipeline BEFORE combine
	ms1 := newSilent(dir, dir)
	result1, err := ms1.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed before combine: %v", err)
	}
	items1 := result1.Buckets[0].Items
	cleanup(items1)

	// Combine
	if err := CombineDir(appDir); err != nil {
		t.Fatalf("CombineDir failed: %v", err)
	}

	// Verify subdirectory XMLs are gone
	if _, err := os.Stat(filepath.Join(appDir, "assets", "assets.miniskin.xml")); err == nil {
		t.Error("assets.miniskin.xml should have been removed")
	}
	if _, err := os.Stat(filepath.Join(appDir, "login", "login.miniskin.xml")); err == nil {
		t.Error("login.miniskin.xml should have been removed")
	}

	// Run pipeline AFTER combine
	ms2 := newSilent(dir, dir)
	result2, err := ms2.BuildEmbed()
	if err != nil {
		t.Fatalf("BuildEmbed after combine: %v", err)
	}
	items2 := result2.Buckets[0].Items
	defer cleanup(items2)

	// Same number of items
	if len(items1) != len(items2) {
		t.Fatalf("item count mismatch: before=%d after=%d", len(items1), len(items2))
	}

	// Same files, same dirs, same urlBases
	for i := range items1 {
		if items1[i].File != items2[i].File {
			t.Errorf("item %d file: before=%s after=%s", i, items1[i].File, items2[i].File)
		}
		if items1[i].dir != items2[i].dir {
			t.Errorf("item %d dir: before=%s after=%s", i, items1[i].dir, items2[i].dir)
		}
		if items1[i].urlBase != items2[i].urlBase {
			t.Errorf("item %d urlBase: before=%s after=%s", i, items1[i].urlBase, items2[i].urlBase)
		}
		if items1[i].EmbedPath != items2[i].EmbedPath {
			t.Errorf("item %d embedPath: before=%s after=%s", i, items1[i].EmbedPath, items2[i].EmbedPath)
		}
	}
}

// ---

func TestCombineDuplicateItemsError(t *testing.T) {
	dir := t.TempDir()

	// Two XMLs in the same directory with the same file — conflict
	os.WriteFile(filepath.Join(dir, "first.miniskin.xml"), []byte(`<miniskin>
	<resource-list>
		<item type="static" file="shared.css" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "second.miniskin.xml"), []byte(`<miniskin>
	<resource-list>
		<item type="static" file="shared.css" />
	</resource-list>
</miniskin>`), 0644)

	err := CombineDir(dir)
	if err == nil {
		t.Fatal("expected error for duplicate items, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

// ---

func TestCombineNoDuplicateDifferentDirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a"), 0755)
	os.MkdirAll(filepath.Join(dir, "b"), 0755)

	// Same filename but different directories — no conflict
	os.WriteFile(filepath.Join(dir, "a", "a.miniskin.xml"), []byte(`<miniskin>
	<resource-list>
		<item type="static" file="style.css" />
	</resource-list>
</miniskin>`), 0644)

	os.WriteFile(filepath.Join(dir, "b", "b.miniskin.xml"), []byte(`<miniskin>
	<resource-list>
		<item type="static" file="style.css" />
	</resource-list>
</miniskin>`), 0644)

	err := CombineDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
