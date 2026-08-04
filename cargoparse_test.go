package miniskin

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablo-botella/cargoxml"
)

// TestCargoParseRoot checks the <miniskin> root consumer: its own
// attributes claimed, everything foreign preserved — the piggyback
// promise: a future attribute, a comment and the untyped children all
// survive a parse-and-rewrite round trip.
func TestCargoParseRoot(t *testing.T) {
	const sample = `<miniskin skin-dir="_skin" log="build.log" mux-include="*.html" mux-exclude="*_src.*" mkskill-note="piggyback">
	<!-- the catalog comment -->
	<globals>
		<var name="appName" value="TestApp"/>
	</globals>
	<escape ext="*.html" as="html"/>
</miniskin>`

	root, err := decodeMiniskin(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if root.SkinDir != "_skin" || root.Log != "build.log" ||
		root.MuxInclude != "*.html" || root.MuxExclude != "*_src.*" {
		t.Errorf("own attributes not claimed: %+v", root)
	}
	if len(root.Cargo.MoreAttributes) != 1 || root.Cargo.MoreAttributes[0].Name.Local != "mkskill-note" {
		t.Errorf("foreign attribute should ride the cargo: %+v", root.Cargo.MoreAttributes)
	}

	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := cargoxml.NewEncoderWithCargo(enc).Encode(root); err != nil {
		t.Fatal(err)
	}
	enc.Flush()
	out := buf.String()
	// equivalent, not byte-identical (cargoxml's documented limits): the
	// self-closing form comes back expanded, tab indentation as &#x9;
	for _, want := range []string{
		`skin-dir="_skin"`,
		`mux-include="*.html"`,
		`mux-exclude="*_src.*"`,
		`mkskill-note="piggyback"`,
		"<!-- the catalog comment -->",
		`<var name="appName" value="TestApp"></var>`,
		`<escape ext="*.html" as="html"></escape>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rewrite misses %q:\n%s", want, out)
		}
	}

	if _, err := decodeMiniskin(strings.NewReader(`<other/>`)); err == nil {
		t.Error("a non-miniskin root must be rejected")
	}

	// the globals are typed now: claimed with their vars, out of the cargo
	if len(root.Globals) != 1 || len(root.Globals[0].Vars) != 1 {
		t.Fatalf("globals not claimed: %+v", root.Globals)
	}
	if v := root.Globals[0].Vars[0]; v.Name != "appName" || v.Value != "TestApp" {
		t.Errorf("var not claimed: %+v", v)
	}

	// the rx view of the same parse: claimed lines, the cargo attribute
	// and the comment counter — globals now on the claimed side
	var rx bytes.Buffer
	root.xrayTo(&rx, "sample")
	if len(root.Escapes) != 1 || root.Escapes[0].Ext != "*.html" || root.Escapes[0].As != "html" {
		t.Errorf("escape not claimed: %+v", root.Escapes)
	}

	for _, want := range []string{
		`claimed  skin-dir="_skin"`,
		"claimed  <globals> — 1 var(s), 1 comment(s)", // the comment moved WITH its element
		"claimed  <escape *.html as html>",
		`cargo    attribute mkskill-note="piggyback"`,
	} {
		if !strings.Contains(rx.String(), want) {
			t.Errorf("rx misses %q:\n%s", want, rx.String())
		}
	}
	if strings.Contains(rx.String(), "cargo    child <globals>") {
		t.Error("globals must have left the cargo column")
	}
	if strings.Contains(rx.String(), "cargo    1 comment(s)") {
		t.Error("the comment left the cargo column with its element")
	}
}

// TestCargoParseRawFidelity is the fase-2 promise on a real catalog shape:
// with cargoxml's Raw writer, a tab-indented document with no self-closing
// tags comes back byte-identical.
func TestCargoParseRawFidelity(t *testing.T) {
	// the escape comes BEFORE the globals: claimed children of different
	// types must keep their document order through the rewrite. Note the
	// attributes ride in CANONICAL order (the describe order) — claimed
	// attribute order is normalized, a documented trait of the claimed
	// side (element order is preserved; attribute order is the spec's
	// "not significant").
	const sample = `<miniskin skin-dir="_skin">
	<!-- kept verbatim -->
	<escape ext="*.html" as="html"></escape>
	<globals>
		<var name="appName" value="TestApp"></var>
	</globals>
	<bucket-list filename="generated_embed.go" module="content">
		<bucket src="app" dst="/modules/app/gen.go" module-name="app">
			<escape ext="*.js" as="js"></escape>
		</bucket>
	</bucket-list>
	<resource-list urlbase="/assets">
		<item type="static" file="app.css"></item>
		<resource-list src="login" urlbase="/login">
			<item file="signin.html" src="signin_src.html"></item>
		</resource-list>
	</resource-list>
	<mockup-list save-mode="append">
		<var name="policybanner" value="1"></var>
		<item src="mockup_login.html">
			<var name="title" value="Login"></var>
		</item>
	</mockup-list>
	<origin name="closure-ui">
		<local base="C:\HD\F\_sams\closure-ui">
			<resource id="ui" src="release/closure_ui.js"></resource>
		</local>
	</origin>
	<external>
		<external-item origin="closure-ui" id="ui" dstfile="./src/app_source.js"></external-item>
	</external>
</miniskin>`

	root, err := decodeMiniskin(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	e := cargoxml.NewEncoderWithCargo(enc)
	e.Raw = &buf // the same writer the xml.Encoder wraps
	if err := e.Encode(root); err != nil {
		t.Fatal(err)
	}
	enc.Flush()
	if buf.String() != sample {
		t.Errorf("Raw rewrite must be byte-identical:\n--- want ---\n%s\n--- got ---\n%s", sample, buf.String())
	}

	// the last pieces of the model, claimed and mapped
	if len(root.Origins) != 1 || root.Origins[0].Name != "closure-ui" ||
		root.Origins[0].Local == nil || len(root.Origins[0].Local.Resources) != 1 {
		t.Fatalf("origin not claimed: %+v", root.Origins)
	}
	if len(root.Externals) != 1 || len(root.Externals[0].Items) != 1 ||
		root.Externals[0].Items[0].Dstfile != "./src/app_source.js" {
		t.Fatalf("external not claimed: %+v", root.Externals)
	}
	var rx bytes.Buffer
	root.xrayTo(&rx, "sample")
	for _, want := range []string{
		"claimed  <origin closure-ui> — local(1 res)",
		"claimed  <external> — 1 item(s)",
	} {
		if !strings.Contains(rx.String(), want) {
			t.Errorf("rx misses %q:\n%s", want, rx.String())
		}
	}
	if strings.Contains(rx.String(), "untyped") {
		t.Errorf("the whole model is typed — nothing may remain untyped:\n%s", rx.String())
	}
}

// TestDebugCatalogs runs the debug command's engine over testdata: the
// echo output carries real tabs (never &#x9;) and the file headers; the rx
// report maps what still rides the cargo.
func TestDebugCatalogs(t *testing.T) {
	dir := t.TempDir()
	echo, rx := filepath.Join(dir, "echo.xml"), filepath.Join(dir, "rx.txt")
	if err := DebugCatalogs("testdata", echo, rx); err != nil {
		t.Fatal(err)
	}
	echoData, err := os.ReadFile(echo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(echoData), "\t<globals>") {
		t.Errorf("echo should carry real tabs:\n%s", echoData)
	}
	if strings.Contains(string(echoData), "&#x9;") {
		t.Error("echo must never carry escaped tabs")
	}
	if !strings.Contains(string(echoData), "<!-- ========== testdata/content.miniskin.xml ========== -->") {
		t.Errorf("echo misses the file header:\n%s", echoData)
	}
	rxData, err := os.ReadFile(rx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rxData), "claimed  <bucket-list generated_embed.go> — 1 bucket(s)") {
		t.Errorf("rx misses the claimed bucket-list:\n%s", rxData)
	}
	if !strings.Contains(string(rxData), "claimed  <resource-list /assets> — 1 item(s)") {
		t.Errorf("rx misses the claimed resource-list:\n%s", rxData)
	}
	// the milestone: the whole testdata is typed — nothing untyped anywhere
	if strings.Contains(string(rxData), "untyped") {
		t.Errorf("nothing should remain untyped in testdata:\n%s", rxData)
	}
}

// TestCargoParseTestdata parses the real testdata catalog: no attributes at
// the root there, and the untyped children land whole in the cargo.
func TestCargoParseTestdata(t *testing.T) {
	root, err := parseMiniskinCargo("testdata/content.miniskin.xml")
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Cargo.MoreChildren) != 0 { // the root's cargo column is empty now
		t.Errorf("want no untyped children left, got %d", len(root.Cargo.MoreChildren))
	}
	if len(root.Globals) != 1 {
		t.Errorf("globals should be claimed, got %+v", root.Globals)
	}
	if len(root.BucketLists) != 1 || len(root.BucketLists[0].Buckets) != 1 {
		t.Fatalf("bucket-list not claimed: %+v", root.BucketLists)
	}
	b := root.BucketLists[0].Buckets[0]
	if root.BucketLists[0].Filename != "generated_embed.go" || b.Src != "app" || b.RecurseFolder != "all" {
		t.Errorf("bucket attributes not claimed: %+v / %+v", root.BucketLists[0], b)
	}
}
