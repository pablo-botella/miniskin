package miniskin

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// scaffoldExternal writes a project: a root content.miniskin.xml (with rootInner
// spliced inside <miniskin>, e.g. an <origin> with <remote>), an app bucket holding
// externalXML, and an optional miniskin-origin.xml. Returns (contentDir, bucketDir).
func scaffoldExternal(t *testing.T, rootInner, originFileXML, externalXML string) (string, string) {
	t.Helper()
	root := t.TempDir()
	bucketDir := filepath.Join(root, "app")
	rootXML := `<?xml version="1.0"?>
<miniskin>
  ` + rootInner + `
  <bucket-list filename="generated_embed.go" module="content">
    <bucket src="/app" dst="/generated_embed_list.go" module-name="app" recurse-folder="all" />
  </bucket-list>
</miniskin>`
	mustWrite(t, filepath.Join(root, "content.miniskin.xml"), rootXML)
	mustWrite(t, filepath.Join(bucketDir, "app.miniskin.xml"), externalXML)
	if originFileXML != "" {
		mustWrite(t, filepath.Join(root, "miniskin-origin.xml"), originFileXML)
	}
	return root, bucketDir
}

// runExternals does the full origin/external phase and returns ms + the error.
func runExternals(t *testing.T, content string) (*Miniskin, error) {
	t.Helper()
	ms := newSilent(content, content)
	root, bl, err := ms.init()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := ms.loadResourceHashes(); err != nil {
		t.Fatalf("loadResourceHashes: %v", err)
	}
	origins, err := ms.loadOrigins(root.Origins)
	if err != nil {
		return ms, err
	}
	if err := ms.processExternals(bl, origins); err != nil {
		return ms, err
	}
	return ms, ms.flushResourceHashes()
}

func TestExternalLocalCopy(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "vendor-src")
	srcFile := filepath.Join(srcDir, "release", "lib.js")
	mustWrite(t, srcFile, "// vendor payload\n")

	content := filepath.Join(root, "content")
	bucketDir := filepath.Join(content, "app")
	mustWrite(t, filepath.Join(content, "content.miniskin.xml"), `<?xml version="1.0"?>
<miniskin>
  <bucket-list filename="generated_embed.go" module="content">
    <bucket src="/app" dst="/generated_embed_list.go" module-name="app" recurse-folder="all" />
  </bucket-list>
</miniskin>`)
	mustWrite(t, filepath.Join(bucketDir, "app.miniskin.xml"), `<?xml version="1.0"?>
<miniskin>
  <external>
    <external-item origin="vendor" id="lib.js" dstfile="./src/lib.js" />
  </external>
</miniskin>`)
	mustWrite(t, filepath.Join(content, "miniskin-origin.xml"), `<?xml version="1.0"?>
<miniskin>
  <origin name="vendor">
    <local base="`+srcDir+`">
      <resource id="lib.js" src="release/lib.js" />
    </local>
  </origin>
</miniskin>`)

	ms := newSilent(content, content)
	rootXML, bl, err := ms.init()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	origins, err := ms.loadOrigins(rootXML.Origins)
	if err != nil {
		t.Fatalf("loadOrigins: %v", err)
	}
	if err := ms.processExternals(bl, origins); err != nil {
		t.Fatalf("processExternals: %v", err)
	}

	dstFile := filepath.Join(bucketDir, "src", "lib.js")
	if got, _ := os.ReadFile(dstFile); string(got) != "// vendor payload\n" {
		t.Fatalf("dst content mismatch: %q", got)
	}

	// idempotent: second run must not rewrite (mtime propagated + preserved)
	dstBefore, _ := os.Stat(dstFile)
	if err := ms.processExternals(bl, origins); err != nil {
		t.Fatalf("second processExternals: %v", err)
	}
	dstAfter, _ := os.Stat(dstFile)
	if !dstAfter.ModTime().Equal(dstBefore.ModTime()) {
		t.Errorf("dst rewritten on idempotent run")
	}

	// touching the source forces a refresh
	newer := dstBefore.ModTime().Add(2 * time.Second)
	os.Chtimes(srcFile, newer, newer)
	if err := ms.processExternals(bl, origins); err != nil {
		t.Fatalf("third processExternals: %v", err)
	}
	if final, _ := os.Stat(dstFile); !final.ModTime().Equal(newer) {
		t.Errorf("dst mtime did not refresh")
	}
}

func TestExternalLocalMissingIsError(t *testing.T) {
	root := t.TempDir()
	emptyDir := filepath.Join(root, "vendor-src")
	os.MkdirAll(emptyDir, 0o755)
	content, _ := scaffoldExternal(t,
		"",
		`<?xml version="1.0"?><miniskin>
  <origin name="vendor"><local base="`+emptyDir+`"><resource id="lib.js" src="lib.js" /></local></origin>
</miniskin>`,
		`<?xml version="1.0"?><miniskin><external>
  <external-item origin="vendor" id="lib.js" dstfile="./out.js" />
</external></miniskin>`)
	// scaffoldExternal already made its own root; but we passed our own dirs, so
	// re-run against the scaffold's content
	_, err := runExternals(t, content)
	if err == nil || !strings.Contains(err.Error(), "local resource missing") {
		t.Errorf("expected local-missing error, got: %v", err)
	}
}

func TestExternalUnknownOrigin(t *testing.T) {
	content, _ := scaffoldExternal(t, "", "",
		`<?xml version="1.0"?><miniskin><external>
  <external-item origin="ghost" id="x.js" dstfile="./y.js" />
</external></miniskin>`)
	_, err := runExternals(t, content)
	if err == nil || !strings.Contains(err.Error(), `origin "ghost" not found`) {
		t.Errorf("expected unknown-origin error, got: %v", err)
	}
}

func TestExternalUnknownResourceID(t *testing.T) {
	content, _ := scaffoldExternal(t,
		`<origin name="vendor"><remote base="http://example.invalid/"><resource id="a.js" src="a.js" /></remote></origin>`,
		"",
		`<?xml version="1.0"?><miniskin><external>
  <external-item origin="vendor" id="missing.js" dstfile="./y.js" />
</external></miniskin>`)
	_, err := runExternals(t, content)
	if err == nil || !strings.Contains(err.Error(), "no resource id") {
		t.Errorf("expected unknown-resource error, got: %v", err)
	}
}

func TestExternalRemoteContract(t *testing.T) {
	const body = "REMOTE-BYTES"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	// matching contract → downloads and writes
	content, bucketDir := scaffoldExternal(t,
		`<origin name="ui"><remote base="`+srv.URL+`/"><resource id="ui.js" src="ui.js" sha256="`+sha256Hex([]byte(body))+`" /></remote></origin>`,
		"",
		`<?xml version="1.0"?><miniskin><external>
  <external-item origin="ui" id="ui.js" dstfile="./ui.js" />
</external></miniskin>`)
	if _, err := runExternals(t, content); err != nil {
		t.Fatalf("runExternals: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(bucketDir, "ui.js")); string(got) != body {
		t.Errorf("remote file wrong: %q", got)
	}
	// contract resources do NOT get cached
	if _, err := os.Stat(filepath.Join(content, resourceHashFilename)); err == nil {
		t.Errorf("contract resource should not write the hash cache")
	}
}

func TestExternalRemoteContractMismatchErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("real"))
	}))
	defer srv.Close()

	content, bucketDir := scaffoldExternal(t,
		`<origin name="ui"><remote base="`+srv.URL+`/"><resource id="ui.js" src="ui.js" sha256="0000000000000000000000000000000000000000000000000000000000000000" /></remote></origin>`,
		"",
		`<?xml version="1.0"?><miniskin><external>
  <external-item origin="ui" id="ui.js" dstfile="./ui.js" />
</external></miniskin>`)
	_, err := runExternals(t, content)
	if err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("expected sha256 mismatch, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bucketDir, "ui.js")); err == nil {
		t.Error("file written despite mismatch")
	}
}

func TestExternalRemoteNoContractCachesAndReuses(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Write([]byte("cacheme"))
	}))
	defer srv.Close()

	content, _ := scaffoldExternal(t,
		`<origin name="ui"><remote base="`+srv.URL+`/"><resource id="ui.js" src="ui.js" /></remote></origin>`,
		"",
		`<?xml version="1.0"?><miniskin><external>
  <external-item origin="ui" id="ui.js" dstfile="./ui.js" />
</external></miniskin>`)

	if _, err := runExternals(t, content); err != nil {
		t.Fatalf("run 1: %v", err)
	}
	// TOFU wrote the cache
	cache, err := os.ReadFile(filepath.Join(content, resourceHashFilename))
	if err != nil || !strings.Contains(string(cache), sha256Hex([]byte("cacheme"))) {
		t.Fatalf("cache not written with hash: %v\n%s", err, cache)
	}
	// second run: file + cache present → no re-download
	if _, err := runExternals(t, content); err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if h := atomic.LoadInt64(&hits); h != 1 {
		t.Errorf("expected 1 download (cache reuse), got %d", h)
	}
}

func TestExternalLocalWinsOverRemote(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Write([]byte("REMOTE"))
	}))
	defer srv.Close()

	root := t.TempDir()
	localDir := filepath.Join(root, "closure-ui", "release")
	mustWrite(t, filepath.Join(localDir, "closure-ui.js"), "LOCAL")

	content := filepath.Join(root, "content")
	bucketDir := filepath.Join(content, "app")
	mustWrite(t, filepath.Join(content, "content.miniskin.xml"), `<?xml version="1.0"?>
<miniskin>
  <origin name="ui"><remote base="`+srv.URL+`/"><resource id="ui.js" src="closure-ui-min-v0.0.4.js" sha256="deadbeef" /></remote></origin>
  <bucket-list filename="generated_embed.go" module="content">
    <bucket src="/app" dst="/generated_embed_list.go" module-name="app" recurse-folder="all" />
  </bucket-list>
</miniskin>`)
	mustWrite(t, filepath.Join(bucketDir, "app.miniskin.xml"), `<?xml version="1.0"?>
<miniskin><external>
  <external-item origin="ui" id="ui.js" dstfile="./ui.js" />
</external></miniskin>`)
	// per-dev override: local wins, ignorehash
	mustWrite(t, filepath.Join(content, "miniskin-origin.xml"), `<?xml version="1.0"?>
<miniskin>
  <origin name="ui"><local base="`+localDir+`"><resource id="ui.js" src="closure-ui.js" ignorehash="always" /></local></origin>
</miniskin>`)

	if _, err := runExternals(t, content); err != nil {
		t.Fatalf("runExternals: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(bucketDir, "ui.js")); string(got) != "LOCAL" {
		t.Errorf("local should win: got %q", got)
	}
	if h := atomic.LoadInt64(&hits); h != 0 {
		t.Errorf("local override must not download; hits=%d", h)
	}
}
