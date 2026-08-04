// cargoparse.go is the cargoxml side of the catalog: each element is a
// consumer that claims what miniskin models and keeps everything foreign in
// its cargo — *.miniskin.xml files can carry other tools' data (the mkskill
// piggyback) and survive rewrites whole, comments included. It replaces
// xmlparse.go's encoding/xml structs one element at a time; both live side
// by side until the swap.
package miniskin

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablo-botella/cargoxml"
)

// cxMiniskin is the <miniskin> document element: the catalog root.
type cxMiniskin struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml // the unclaimed: foreign attributes, children not yet typed, comments and formatting

	SkinDir    string // skin-dir
	Log        string // log
	MuxInclude string // mux-include
	MuxExclude string // mux-exclude

	Globals       []*cxGlobals      // globals — each element as it came; merging is the consumer's business
	Escapes       []*cxEscape       // escape — the root-level escape rules
	BucketLists   []*cxBucketList   // bucket-list — each as it came
	ResourceLists []*cxResourceList // resource-list — the subdirectory catalogs' top level
	MockupLists   []*cxMockupList   // mockup-list — each as it came
	Origins       []*cxOrigin       // origin — the per-developer registry entries
	Externals     []*cxExternal     // external — each as it came

	items []cargoxml.XmlTokenProducer // every claimed child, in document order — what the rewrite emits
}

// compile-time checks: describe side and producer in place
var (
	_ cargoxml.XmlDescribeWithCargo = (*cxMiniskin)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxMiniskin)(nil)
)

// GetCargoXml wires the preservation: whatever the events below do not
// claim ends up stored here by the decoder.
func (m *cxMiniskin) GetCargoXml() *cargoxml.CargoXml {
	if m.Cargo == nil {
		m.Cargo = cargoxml.NewCargoXml()
	}
	return m.Cargo
}

// OnXmlStart rejects any document whose root is not <miniskin>.
func (m *cxMiniskin) OnXmlStart(d *cargoxml.DecoderWithCargo, frame *cargoxml.DecoderStackFrame) error {
	if frame.NodeName.Local != "miniskin" {
		return fmt.Errorf("root element is <%s>, want <miniskin>", frame.NodeName.Local)
	}
	return nil
}

// OnXmlChildStart assigns a consumer to each known child; an unknown child
// is left unclaimed, so the decoder parses it generically into the cargo.
func (m *cxMiniskin) OnXmlChildStart(d *cargoxml.DecoderWithCargo, child *cargoxml.DecoderStackFrame) error {
	switch child.NodeName.Local {
	case "globals":
		g := &cxGlobals{}
		m.Globals = append(m.Globals, g)
		m.items = append(m.items, g)
		child.Consumer = g
	case "escape":
		e := &cxEscape{}
		m.Escapes = append(m.Escapes, e)
		m.items = append(m.items, e)
		child.Consumer = e
	case "bucket-list":
		bl := &cxBucketList{}
		m.BucketLists = append(m.BucketLists, bl)
		m.items = append(m.items, bl)
		child.Consumer = bl
	case "resource-list":
		rl := &cxResourceList{}
		m.ResourceLists = append(m.ResourceLists, rl)
		m.items = append(m.items, rl)
		child.Consumer = rl
	case "mockup-list":
		ml := &cxMockupList{}
		m.MockupLists = append(m.MockupLists, ml)
		m.items = append(m.items, ml)
		child.Consumer = ml
	case "origin":
		o := &cxOrigin{}
		m.Origins = append(m.Origins, o)
		m.items = append(m.items, o)
		child.Consumer = o
	case "external":
		ex := &cxExternal{}
		m.Externals = append(m.Externals, ex)
		m.items = append(m.items, ex)
		child.Consumer = ex
	}
	return nil
}

// OnXmlAttribute claims the root's own attributes; anything else falls to
// the cargo.
func (m *cxMiniskin) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	switch a.Name.Local {
	case "skin-dir":
		m.SkinDir = a.Value
	case "log":
		m.Log = a.Value
	case "mux-include":
		m.MuxInclude = a.Value
	case "mux-exclude":
		m.MuxExclude = a.Value
	default:
		return false
	}
	return true
}

// --- describe side + producer: how the root presents itself for rewriting ---

func (m *cxMiniskin) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "miniskin"}
}

func (m *cxMiniskin) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode // whatever the cargo holds rides along
}

// XmlDescribeAttributes answers the root's own attributes; empty ones are
// omitted (absent in, absent out).
func (m *cxMiniskin) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	var attrs []xml.Attr
	add := func(name, value string) {
		if value != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
		}
	}
	add("skin-dir", m.SkinDir)
	add("log", m.Log)
	add("mux-include", m.MuxInclude)
	add("mux-exclude", m.MuxExclude)
	return attrs
}

func (m *cxMiniskin) XmlDescribeInitialComments(ctx context.Context) []string { return nil }
func (m *cxMiniskin) XmlDescribeText(ctx context.Context) []string            { return nil }

// XmlDescribeItems answers the claimed children in document order — the
// claim list records it as the decoder fires; the untyped ones follow
// from the cargo.
func (m *cxMiniskin) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer {
	return m.items
}

func (m *cxMiniskin) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, m, cargoxml.PreserveMixed)
}

// --- <globals>: a container of <var> ---

// cxGlobals is one <globals> element as it came in the document.
type cxGlobals struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml // the unclaimed — and this element's own trails (its formatting)

	Vars []*cxVar // var
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxGlobals)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxGlobals)(nil)
)

func (g *cxGlobals) GetCargoXml() *cargoxml.CargoXml {
	if g.Cargo == nil {
		g.Cargo = cargoxml.NewCargoXml()
	}
	return g.Cargo
}

func (g *cxGlobals) OnXmlChildStart(d *cargoxml.DecoderWithCargo, child *cargoxml.DecoderStackFrame) error {
	if child.NodeName.Local == "var" {
		v := &cxVar{}
		g.Vars = append(g.Vars, v)
		child.Consumer = v
	}
	return nil
}

func (g *cxGlobals) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "globals"}
}

func (g *cxGlobals) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (g *cxGlobals) XmlDescribeAttributes(ctx context.Context) []xml.Attr          { return nil }
func (g *cxGlobals) XmlDescribeInitialComments(ctx context.Context) []string       { return nil }
func (g *cxGlobals) XmlDescribeText(ctx context.Context) []string                  { return nil }

func (g *cxGlobals) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer {
	var items []cargoxml.XmlTokenProducer
	for _, v := range g.Vars {
		items = append(items, v)
	}
	return items
}

func (g *cxGlobals) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, g, cargoxml.PreserveMixed)
}

// --- <bucket-list>: the embed generation config ---

// cxBucketList is one <bucket-list> element: the codegen head plus its
// buckets and escape rules.
type cxBucketList struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	Filename    string // filename
	Module      string // module
	Import      string // import
	Template    string // template
	ProjectRoot string // project-root
	BlobOut     string // blob-out
	MuxInclude  string // mux-include
	MuxExclude  string // mux-exclude
	Omit        string // omit

	Escapes []*cxEscape // escape
	Buckets []*cxBucket // bucket

	items []cargoxml.XmlTokenProducer // claimed children in document order
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxBucketList)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxBucketList)(nil)
)

func (bl *cxBucketList) GetCargoXml() *cargoxml.CargoXml {
	if bl.Cargo == nil {
		bl.Cargo = cargoxml.NewCargoXml()
	}
	return bl.Cargo
}

func (bl *cxBucketList) OnXmlChildStart(d *cargoxml.DecoderWithCargo, child *cargoxml.DecoderStackFrame) error {
	switch child.NodeName.Local {
	case "escape":
		e := &cxEscape{}
		bl.Escapes = append(bl.Escapes, e)
		bl.items = append(bl.items, e)
		child.Consumer = e
	case "bucket":
		b := &cxBucket{}
		bl.Buckets = append(bl.Buckets, b)
		bl.items = append(bl.items, b)
		child.Consumer = b
	}
	return nil
}

func (bl *cxBucketList) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	switch a.Name.Local {
	case "filename":
		bl.Filename = a.Value
	case "module":
		bl.Module = a.Value
	case "import":
		bl.Import = a.Value
	case "template":
		bl.Template = a.Value
	case "project-root":
		bl.ProjectRoot = a.Value
	case "blob-out":
		bl.BlobOut = a.Value
	case "mux-include":
		bl.MuxInclude = a.Value
	case "mux-exclude":
		bl.MuxExclude = a.Value
	case "omit":
		bl.Omit = a.Value
	default:
		return false
	}
	return true
}

func (bl *cxBucketList) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "bucket-list"}
}

func (bl *cxBucketList) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (bl *cxBucketList) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	var attrs []xml.Attr
	add := func(name, value string) {
		if value != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
		}
	}
	add("filename", bl.Filename)
	add("module", bl.Module)
	add("import", bl.Import)
	add("template", bl.Template)
	add("project-root", bl.ProjectRoot)
	add("blob-out", bl.BlobOut)
	add("mux-include", bl.MuxInclude)
	add("mux-exclude", bl.MuxExclude)
	add("omit", bl.Omit)
	return attrs
}

func (bl *cxBucketList) XmlDescribeInitialComments(ctx context.Context) []string { return nil }
func (bl *cxBucketList) XmlDescribeText(ctx context.Context) []string            { return nil }

func (bl *cxBucketList) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer {
	return bl.items
}

func (bl *cxBucketList) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, bl, cargoxml.PreserveMixed)
}

// --- <bucket>: one embed bucket ---

// cxBucket is one <bucket> element: a source folder bound to a generated
// destination. Its resource-list and mockup-list children stay in the
// cargo until their own pasito.
type cxBucket struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	Src                 string // src
	Dst                 string // dst
	ModuleName          string // module-name
	RecurseFolder       string // recurse-folder
	Template            string // template
	SkinDir             string // skin-dir
	MuxInclude          string // mux-include
	MuxExclude          string // mux-exclude
	TemplateFunctionMap string // template-function-map

	Escapes       []*cxEscape       // escape
	ResourceLists []*cxResourceList // resource-list
	MockupLists   []*cxMockupList   // mockup-list

	items []cargoxml.XmlTokenProducer // claimed children in document order
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxBucket)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxBucket)(nil)
)

func (b *cxBucket) GetCargoXml() *cargoxml.CargoXml {
	if b.Cargo == nil {
		b.Cargo = cargoxml.NewCargoXml()
	}
	return b.Cargo
}

func (b *cxBucket) OnXmlChildStart(d *cargoxml.DecoderWithCargo, child *cargoxml.DecoderStackFrame) error {
	switch child.NodeName.Local {
	case "escape":
		e := &cxEscape{}
		b.Escapes = append(b.Escapes, e)
		b.items = append(b.items, e)
		child.Consumer = e
	case "resource-list":
		rl := &cxResourceList{}
		b.ResourceLists = append(b.ResourceLists, rl)
		b.items = append(b.items, rl)
		child.Consumer = rl
	case "mockup-list":
		ml := &cxMockupList{}
		b.MockupLists = append(b.MockupLists, ml)
		b.items = append(b.items, ml)
		child.Consumer = ml
	}
	return nil
}

func (b *cxBucket) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	switch a.Name.Local {
	case "src":
		b.Src = a.Value
	case "dst":
		b.Dst = a.Value
	case "module-name":
		b.ModuleName = a.Value
	case "recurse-folder":
		b.RecurseFolder = a.Value
	case "template":
		b.Template = a.Value
	case "skin-dir":
		b.SkinDir = a.Value
	case "mux-include":
		b.MuxInclude = a.Value
	case "mux-exclude":
		b.MuxExclude = a.Value
	case "template-function-map":
		b.TemplateFunctionMap = a.Value
	default:
		return false
	}
	return true
}

func (b *cxBucket) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "bucket"}
}

func (b *cxBucket) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (b *cxBucket) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	var attrs []xml.Attr
	add := func(name, value string) {
		if value != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
		}
	}
	add("src", b.Src)
	add("dst", b.Dst)
	add("module-name", b.ModuleName)
	add("recurse-folder", b.RecurseFolder)
	add("template", b.Template)
	add("skin-dir", b.SkinDir)
	add("mux-include", b.MuxInclude)
	add("mux-exclude", b.MuxExclude)
	add("template-function-map", b.TemplateFunctionMap)
	return attrs
}

func (b *cxBucket) XmlDescribeInitialComments(ctx context.Context) []string { return nil }
func (b *cxBucket) XmlDescribeText(ctx context.Context) []string            { return nil }

func (b *cxBucket) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer {
	return b.items
}

func (b *cxBucket) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, b, cargoxml.PreserveMixed)
}

// --- <resource-list>: assets, nested lists included ---

// cxResourceList is one <resource-list> element: its items, its escape
// rules — and nested resource-lists, the same type all the way down.
type cxResourceList struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	Src                 string // src
	URLBase             string // urlbase
	SkinDir             string // skin-dir
	BlobName            string // blob-name
	PreserveBlobID      string // preserve-blob-if-id
	BlobAttach          string // blob-attach
	MuxInclude          string // mux-include
	MuxExclude          string // mux-exclude
	TemplateFunctionMap string // template-function-map

	Escapes       []*cxEscape       // escape
	Items         []*cxItem         // item
	ResourceLists []*cxResourceList // resource-list — nested

	items []cargoxml.XmlTokenProducer // claimed children in document order
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxResourceList)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxResourceList)(nil)
)

func (rl *cxResourceList) GetCargoXml() *cargoxml.CargoXml {
	if rl.Cargo == nil {
		rl.Cargo = cargoxml.NewCargoXml()
	}
	return rl.Cargo
}

func (rl *cxResourceList) OnXmlChildStart(d *cargoxml.DecoderWithCargo, child *cargoxml.DecoderStackFrame) error {
	switch child.NodeName.Local {
	case "escape":
		e := &cxEscape{}
		rl.Escapes = append(rl.Escapes, e)
		rl.items = append(rl.items, e)
		child.Consumer = e
	case "item":
		it := &cxItem{}
		rl.Items = append(rl.Items, it)
		rl.items = append(rl.items, it)
		child.Consumer = it
	case "resource-list":
		nested := &cxResourceList{}
		rl.ResourceLists = append(rl.ResourceLists, nested)
		rl.items = append(rl.items, nested)
		child.Consumer = nested
	}
	return nil
}

func (rl *cxResourceList) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	switch a.Name.Local {
	case "src":
		rl.Src = a.Value
	case "urlbase":
		rl.URLBase = a.Value
	case "skin-dir":
		rl.SkinDir = a.Value
	case "blob-name":
		rl.BlobName = a.Value
	case "preserve-blob-if-id":
		rl.PreserveBlobID = a.Value
	case "blob-attach":
		rl.BlobAttach = a.Value
	case "mux-include":
		rl.MuxInclude = a.Value
	case "mux-exclude":
		rl.MuxExclude = a.Value
	case "template-function-map":
		rl.TemplateFunctionMap = a.Value
	default:
		return false
	}
	return true
}

func (rl *cxResourceList) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "resource-list"}
}

func (rl *cxResourceList) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (rl *cxResourceList) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	var attrs []xml.Attr
	add := func(name, value string) {
		if value != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
		}
	}
	add("src", rl.Src)
	add("urlbase", rl.URLBase)
	add("skin-dir", rl.SkinDir)
	add("blob-name", rl.BlobName)
	add("preserve-blob-if-id", rl.PreserveBlobID)
	add("blob-attach", rl.BlobAttach)
	add("mux-include", rl.MuxInclude)
	add("mux-exclude", rl.MuxExclude)
	add("template-function-map", rl.TemplateFunctionMap)
	return attrs
}

func (rl *cxResourceList) XmlDescribeInitialComments(ctx context.Context) []string { return nil }
func (rl *cxResourceList) XmlDescribeText(ctx context.Context) []string            { return nil }

func (rl *cxResourceList) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer {
	return rl.items
}

func (rl *cxResourceList) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, rl, cargoxml.PreserveMixed)
}

// --- <origin>: one entry of the per-developer registry ---

// cxOrigin is one <origin> element: a named source of external files, with
// its local and/or remote side. A duplicate <local>/<remote> is left
// unclaimed — the first one is the typed truth, the rest ride the cargo.
type cxOrigin struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	Name   string    // name
	Local  *cxSource // local
	Remote *cxSource // remote

	items []cargoxml.XmlTokenProducer // claimed children in document order
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxOrigin)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxOrigin)(nil)
)

func (o *cxOrigin) GetCargoXml() *cargoxml.CargoXml {
	if o.Cargo == nil {
		o.Cargo = cargoxml.NewCargoXml()
	}
	return o.Cargo
}

func (o *cxOrigin) OnXmlChildStart(d *cargoxml.DecoderWithCargo, child *cargoxml.DecoderStackFrame) error {
	switch child.NodeName.Local {
	case "local":
		if o.Local == nil {
			o.Local = &cxSource{tag: "local"}
			o.items = append(o.items, o.Local)
			child.Consumer = o.Local
		}
	case "remote":
		if o.Remote == nil {
			o.Remote = &cxSource{tag: "remote"}
			o.items = append(o.items, o.Remote)
			child.Consumer = o.Remote
		}
	}
	return nil
}

func (o *cxOrigin) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	if a.Name.Local == "name" {
		o.Name = a.Value
		return true
	}
	return false
}

func (o *cxOrigin) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "origin"}
}

func (o *cxOrigin) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (o *cxOrigin) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	if o.Name == "" {
		return nil
	}
	return []xml.Attr{{Name: xml.Name{Local: "name"}, Value: o.Name}}
}

func (o *cxOrigin) XmlDescribeInitialComments(ctx context.Context) []string          { return nil }
func (o *cxOrigin) XmlDescribeText(ctx context.Context) []string                     { return nil }
func (o *cxOrigin) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer { return o.items }

func (o *cxOrigin) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, o, cargoxml.PreserveMixed)
}

// --- <local> / <remote>: one side of an origin ---

// cxSource is one side of an origin — the same shape under two tag names,
// so the tag it came with is part of its identity.
type cxSource struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	tag  string // "local" or "remote" — the element name it came with
	Base string // base

	Resources []*cxResource // resource

	items []cargoxml.XmlTokenProducer // claimed children in document order
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxSource)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxSource)(nil)
)

func (s *cxSource) GetCargoXml() *cargoxml.CargoXml {
	if s.Cargo == nil {
		s.Cargo = cargoxml.NewCargoXml()
	}
	return s.Cargo
}

func (s *cxSource) OnXmlChildStart(d *cargoxml.DecoderWithCargo, child *cargoxml.DecoderStackFrame) error {
	if child.NodeName.Local == "resource" {
		r := &cxResource{}
		s.Resources = append(s.Resources, r)
		s.items = append(s.items, r)
		child.Consumer = r
	}
	return nil
}

func (s *cxSource) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	if a.Name.Local == "base" {
		s.Base = a.Value
		return true
	}
	return false
}

func (s *cxSource) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: s.tag}
}

func (s *cxSource) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (s *cxSource) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	if s.Base == "" {
		return nil
	}
	return []xml.Attr{{Name: xml.Name{Local: "base"}, Value: s.Base}}
}

func (s *cxSource) XmlDescribeInitialComments(ctx context.Context) []string          { return nil }
func (s *cxSource) XmlDescribeText(ctx context.Context) []string                     { return nil }
func (s *cxSource) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer { return s.items }

func (s *cxSource) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, s, cargoxml.PreserveMixed)
}

// --- <resource>: one file an origin offers ---

// cxResource is one <resource> of an origin's side.
type cxResource struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	ID         string // id
	Src        string // src
	Sha256     string // sha256
	IgnoreHash string // ignorehash
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxResource)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxResource)(nil)
)

func (r *cxResource) GetCargoXml() *cargoxml.CargoXml {
	if r.Cargo == nil {
		r.Cargo = cargoxml.NewCargoXml()
	}
	return r.Cargo
}

func (r *cxResource) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	switch a.Name.Local {
	case "id":
		r.ID = a.Value
	case "src":
		r.Src = a.Value
	case "sha256":
		r.Sha256 = a.Value
	case "ignorehash":
		r.IgnoreHash = a.Value
	default:
		return false
	}
	return true
}

func (r *cxResource) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "resource"}
}

func (r *cxResource) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (r *cxResource) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	var attrs []xml.Attr
	add := func(name, value string) {
		if value != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
		}
	}
	add("id", r.ID)
	add("src", r.Src)
	add("sha256", r.Sha256)
	add("ignorehash", r.IgnoreHash)
	return attrs
}

func (r *cxResource) XmlDescribeInitialComments(ctx context.Context) []string          { return nil }
func (r *cxResource) XmlDescribeText(ctx context.Context) []string                     { return nil }
func (r *cxResource) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer { return nil }

func (r *cxResource) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, r, cargoxml.PreserveMixed)
}

// --- <external>: the files to copy in at build time ---

// cxExternal is one <external> block: its external-item list.
type cxExternal struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	Items []*cxExternalItem // external-item

	items []cargoxml.XmlTokenProducer // claimed children in document order
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxExternal)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxExternal)(nil)
)

func (ex *cxExternal) GetCargoXml() *cargoxml.CargoXml {
	if ex.Cargo == nil {
		ex.Cargo = cargoxml.NewCargoXml()
	}
	return ex.Cargo
}

func (ex *cxExternal) OnXmlChildStart(d *cargoxml.DecoderWithCargo, child *cargoxml.DecoderStackFrame) error {
	if child.NodeName.Local == "external-item" {
		it := &cxExternalItem{}
		ex.Items = append(ex.Items, it)
		ex.items = append(ex.items, it)
		child.Consumer = it
	}
	return nil
}

func (ex *cxExternal) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "external"}
}

func (ex *cxExternal) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (ex *cxExternal) XmlDescribeAttributes(ctx context.Context) []xml.Attr           { return nil }
func (ex *cxExternal) XmlDescribeInitialComments(ctx context.Context) []string        { return nil }
func (ex *cxExternal) XmlDescribeText(ctx context.Context) []string                   { return nil }
func (ex *cxExternal) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer {
	return ex.items
}

func (ex *cxExternal) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, ex, cargoxml.PreserveMixed)
}

// --- <external-item>: one file to copy in ---

// cxExternalItem is one <external-item> of an external block.
type cxExternalItem struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	Origin  string // origin
	ID      string // id
	Dstfile string // dstfile
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxExternalItem)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxExternalItem)(nil)
)

func (it *cxExternalItem) GetCargoXml() *cargoxml.CargoXml {
	if it.Cargo == nil {
		it.Cargo = cargoxml.NewCargoXml()
	}
	return it.Cargo
}

func (it *cxExternalItem) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	switch a.Name.Local {
	case "origin":
		it.Origin = a.Value
	case "id":
		it.ID = a.Value
	case "dstfile":
		it.Dstfile = a.Value
	default:
		return false
	}
	return true
}

func (it *cxExternalItem) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "external-item"}
}

func (it *cxExternalItem) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (it *cxExternalItem) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	var attrs []xml.Attr
	add := func(name, value string) {
		if value != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
		}
	}
	add("origin", it.Origin)
	add("id", it.ID)
	add("dstfile", it.Dstfile)
	return attrs
}

func (it *cxExternalItem) XmlDescribeInitialComments(ctx context.Context) []string          { return nil }
func (it *cxExternalItem) XmlDescribeText(ctx context.Context) []string                     { return nil }
func (it *cxExternalItem) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer { return nil }

func (it *cxExternalItem) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, it, cargoxml.PreserveMixed)
}

// --- <mockup-list>: the mockup pipeline config ---

// cxMockupList is one <mockup-list> element: shared vars plus the mockup
// items (whose <item> is its own kind, nothing to do with an asset item).
type cxMockupList struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	SkinDir  string // skin-dir
	SaveMode string // save-mode
	LineMode string // line-mode

	Vars  []*cxVar        // var — shared by every mockup
	Items []*cxMockupItem // item

	items []cargoxml.XmlTokenProducer // claimed children in document order
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxMockupList)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxMockupList)(nil)
)

func (ml *cxMockupList) GetCargoXml() *cargoxml.CargoXml {
	if ml.Cargo == nil {
		ml.Cargo = cargoxml.NewCargoXml()
	}
	return ml.Cargo
}

func (ml *cxMockupList) OnXmlChildStart(d *cargoxml.DecoderWithCargo, child *cargoxml.DecoderStackFrame) error {
	switch child.NodeName.Local {
	case "var":
		v := &cxVar{}
		ml.Vars = append(ml.Vars, v)
		ml.items = append(ml.items, v)
		child.Consumer = v
	case "item":
		it := &cxMockupItem{}
		ml.Items = append(ml.Items, it)
		ml.items = append(ml.items, it)
		child.Consumer = it
	}
	return nil
}

func (ml *cxMockupList) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	switch a.Name.Local {
	case "skin-dir":
		ml.SkinDir = a.Value
	case "save-mode":
		ml.SaveMode = a.Value
	case "line-mode":
		ml.LineMode = a.Value
	default:
		return false
	}
	return true
}

func (ml *cxMockupList) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "mockup-list"}
}

func (ml *cxMockupList) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (ml *cxMockupList) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	var attrs []xml.Attr
	add := func(name, value string) {
		if value != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
		}
	}
	add("skin-dir", ml.SkinDir)
	add("save-mode", ml.SaveMode)
	add("line-mode", ml.LineMode)
	return attrs
}

func (ml *cxMockupList) XmlDescribeInitialComments(ctx context.Context) []string { return nil }
func (ml *cxMockupList) XmlDescribeText(ctx context.Context) []string            { return nil }

func (ml *cxMockupList) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer {
	return ml.items
}

func (ml *cxMockupList) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, ml, cargoxml.PreserveMixed)
}

// --- mockup <item>: one mockup source with its own vars ---

// cxMockupItem is one mockup <item>: a source file plus its per-item vars.
type cxMockupItem struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	Src      string // src
	Negative string // negative
	SaveMode string // save-mode

	Vars []*cxVar // var

	items []cargoxml.XmlTokenProducer // claimed children in document order
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxMockupItem)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxMockupItem)(nil)
)

func (it *cxMockupItem) GetCargoXml() *cargoxml.CargoXml {
	if it.Cargo == nil {
		it.Cargo = cargoxml.NewCargoXml()
	}
	return it.Cargo
}

func (it *cxMockupItem) OnXmlChildStart(d *cargoxml.DecoderWithCargo, child *cargoxml.DecoderStackFrame) error {
	if child.NodeName.Local == "var" {
		v := &cxVar{}
		it.Vars = append(it.Vars, v)
		it.items = append(it.items, v)
		child.Consumer = v
	}
	return nil
}

func (it *cxMockupItem) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	switch a.Name.Local {
	case "src":
		it.Src = a.Value
	case "negative":
		it.Negative = a.Value
	case "save-mode":
		it.SaveMode = a.Value
	default:
		return false
	}
	return true
}

func (it *cxMockupItem) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "item"}
}

func (it *cxMockupItem) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (it *cxMockupItem) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	var attrs []xml.Attr
	add := func(name, value string) {
		if value != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
		}
	}
	add("src", it.Src)
	add("negative", it.Negative)
	add("save-mode", it.SaveMode)
	return attrs
}

func (it *cxMockupItem) XmlDescribeInitialComments(ctx context.Context) []string          { return nil }
func (it *cxMockupItem) XmlDescribeText(ctx context.Context) []string                     { return nil }
func (it *cxMockupItem) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer { return it.items }

func (it *cxMockupItem) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, it, cargoxml.PreserveMixed)
}

// --- <item>: one asset ---

// cxItem is one <item> element: an asset of a resource-list.
type cxItem struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	Type                string // type
	File                string // file
	Src                 string // src
	URL                 string // url
	AltURL              string // alt-url-abs
	Key                 string // key
	Escape              string // escape
	TemplateFunctionMap string // template-function-map
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxItem)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxItem)(nil)
)

func (it *cxItem) GetCargoXml() *cargoxml.CargoXml {
	if it.Cargo == nil {
		it.Cargo = cargoxml.NewCargoXml()
	}
	return it.Cargo
}

func (it *cxItem) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	switch a.Name.Local {
	case "type":
		it.Type = a.Value
	case "file":
		it.File = a.Value
	case "src":
		it.Src = a.Value
	case "url":
		it.URL = a.Value
	case "alt-url-abs":
		it.AltURL = a.Value
	case "key":
		it.Key = a.Value
	case "escape":
		it.Escape = a.Value
	case "template-function-map":
		it.TemplateFunctionMap = a.Value
	default:
		return false
	}
	return true
}

func (it *cxItem) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "item"}
}

func (it *cxItem) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (it *cxItem) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	var attrs []xml.Attr
	add := func(name, value string) {
		if value != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
		}
	}
	add("type", it.Type)
	add("file", it.File)
	add("src", it.Src)
	add("url", it.URL)
	add("alt-url-abs", it.AltURL)
	add("key", it.Key)
	add("escape", it.Escape)
	add("template-function-map", it.TemplateFunctionMap)
	return attrs
}

func (it *cxItem) XmlDescribeInitialComments(ctx context.Context) []string          { return nil }
func (it *cxItem) XmlDescribeText(ctx context.Context) []string                     { return nil }
func (it *cxItem) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer { return nil }

func (it *cxItem) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, it, cargoxml.PreserveMixed)
}

// --- <escape>: one extension → escaping rule ---

// cxEscape is one <escape ext="…" as="…"/> rule.
type cxEscape struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	Ext string // ext
	As  string // as
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxEscape)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxEscape)(nil)
)

func (e *cxEscape) GetCargoXml() *cargoxml.CargoXml {
	if e.Cargo == nil {
		e.Cargo = cargoxml.NewCargoXml()
	}
	return e.Cargo
}

func (e *cxEscape) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	switch a.Name.Local {
	case "ext":
		e.Ext = a.Value
	case "as":
		e.As = a.Value
	default:
		return false
	}
	return true
}

func (e *cxEscape) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "escape"}
}

func (e *cxEscape) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

func (e *cxEscape) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	var attrs []xml.Attr
	add := func(name, value string) {
		if value != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
		}
	}
	add("ext", e.Ext)
	add("as", e.As)
	return attrs
}

func (e *cxEscape) XmlDescribeInitialComments(ctx context.Context) []string          { return nil }
func (e *cxEscape) XmlDescribeText(ctx context.Context) []string                     { return nil }
func (e *cxEscape) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer { return nil }

func (e *cxEscape) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, e, cargoxml.PreserveMixed)
}

// --- <var>: one name/value pair ---

// cxVar is one <var name="…" value="…"/>.
type cxVar struct {
	cargoxml.NullXmlConsumer
	Cargo *cargoxml.CargoXml

	Name  string // name
	Value string // value
}

var (
	_ cargoxml.XmlDescribeWithCargo = (*cxVar)(nil)
	_ cargoxml.XmlTokenProducer     = (*cxVar)(nil)
)

func (v *cxVar) GetCargoXml() *cargoxml.CargoXml {
	if v.Cargo == nil {
		v.Cargo = cargoxml.NewCargoXml()
	}
	return v.Cargo
}

func (v *cxVar) OnXmlAttribute(d *cargoxml.DecoderWithCargo, a *xml.Attr) bool {
	switch a.Name.Local {
	case "name":
		v.Name = a.Value
	case "value":
		v.Value = a.Value
	default:
		return false
	}
	return true
}

func (v *cxVar) XmlDescribeNodeName(ctx context.Context) xml.Name {
	return xml.Name{Local: "var"}
}

func (v *cxVar) XmlDescribeNodeType(ctx context.Context, policy cargoxml.MixedNodePolicy) cargoxml.XmlNodeType {
	return cargoxml.XmlMixedNode
}

// XmlDescribeAttributes answers the pair; empty ones are omitted (absent
// in, absent out — the house rule).
func (v *cxVar) XmlDescribeAttributes(ctx context.Context) []xml.Attr {
	var attrs []xml.Attr
	add := func(name, value string) {
		if value != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
		}
	}
	add("name", v.Name)
	add("value", v.Value)
	return attrs
}

func (v *cxVar) XmlDescribeInitialComments(ctx context.Context) []string          { return nil }
func (v *cxVar) XmlDescribeText(ctx context.Context) []string                     { return nil }
func (v *cxVar) XmlDescribeItems(ctx context.Context) []cargoxml.XmlTokenProducer { return nil }

func (v *cxVar) XmlTokens(ctx context.Context) iter.Seq[xml.Token] {
	return cargoxml.DescribedTokens(ctx, v, cargoxml.PreserveMixed)
}

// decodeMiniskin parses one catalog from a reader.
func decodeMiniskin(r io.Reader) (*cxMiniskin, error) {
	root := &cxMiniskin{}
	d := cargoxml.NewDecoderWithCargo(xml.NewDecoder(r))
	d.Root = root
	if err := d.Parse(); err != nil {
		return nil, err
	}
	return root, nil
}

// parseMiniskinCargo parses one *.miniskin.xml catalog file.
func parseMiniskinCargo(path string) (*cxMiniskin, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return decodeMiniskin(f)
}

// --- the bridge: cargo parse → the xml* structs the engine consumes ---
//
// Same data, new parser underneath: the engine keeps reading xmlMiniskin
// and friends (plain DTOs now — their encoding/xml tags are vestigial),
// while the parse itself preserves everything foreign. The old Unmarshal
// semantics are mirrored: globals>var accumulates across every <globals>,
// and the single-pointer children (bucket-list, mockup-list, external)
// keep the LAST occurrence, exactly as encoding/xml did.

func (m *cxMiniskin) toXml() *xmlMiniskin {
	ms := &xmlMiniskin{
		SkinDir:    m.SkinDir,
		Log:        m.Log,
		MuxInclude: m.MuxInclude,
		MuxExclude: m.MuxExclude,
	}
	for _, g := range m.Globals {
		for _, v := range g.Vars {
			ms.Globals = append(ms.Globals, xmlVar{Name: v.Name, Value: v.Value})
		}
	}
	for _, e := range m.Escapes {
		ms.Escapes = append(ms.Escapes, xmlEscape{Ext: e.Ext, As: e.As})
	}
	if n := len(m.BucketLists); n > 0 {
		ms.BucketList = m.BucketLists[n-1].toXml()
	}
	for _, rl := range m.ResourceLists {
		ms.ResourceLists = append(ms.ResourceLists, rl.toXml())
	}
	if n := len(m.MockupLists); n > 0 {
		ml := m.MockupLists[n-1].toXml()
		ms.MockupList = &ml
	}
	for _, o := range m.Origins {
		ms.Origins = append(ms.Origins, o.toXml())
	}
	if n := len(m.Externals); n > 0 {
		ex := m.Externals[n-1].toXml()
		ms.External = &ex
	}
	return ms
}

func cxEscapesToXml(escapes []*cxEscape) []xmlEscape {
	var out []xmlEscape
	for _, e := range escapes {
		out = append(out, xmlEscape{Ext: e.Ext, As: e.As})
	}
	return out
}

func cxVarsToXml(vars []*cxVar) []xmlVar {
	var out []xmlVar
	for _, v := range vars {
		out = append(out, xmlVar{Name: v.Name, Value: v.Value})
	}
	return out
}

func (bl *cxBucketList) toXml() *xmlBucketList {
	out := &xmlBucketList{
		Filename:    bl.Filename,
		Module:      bl.Module,
		Import:      bl.Import,
		Template:    bl.Template,
		ProjectRoot: bl.ProjectRoot,
		BlobOut:     bl.BlobOut,
		MuxInclude:  bl.MuxInclude,
		MuxExclude:  bl.MuxExclude,
		Omit:        bl.Omit,
		Escapes:     cxEscapesToXml(bl.Escapes),
	}
	for _, b := range bl.Buckets {
		xb := xmlBucket{
			Src:                 b.Src,
			Dst:                 b.Dst,
			ModuleName:          b.ModuleName,
			RecurseFolder:       b.RecurseFolder,
			Template:            b.Template,
			SkinDir:             b.SkinDir,
			MuxInclude:          b.MuxInclude,
			MuxExclude:          b.MuxExclude,
			TemplateFunctionMap: b.TemplateFunctionMap,
			Escapes:             cxEscapesToXml(b.Escapes),
		}
		for _, rl := range b.ResourceLists {
			xb.ResourceLists = append(xb.ResourceLists, rl.toXml())
		}
		for _, ml := range b.MockupLists {
			xb.MockupLists = append(xb.MockupLists, ml.toXml())
		}
		out.Buckets = append(out.Buckets, xb)
	}
	return out
}

func (rl *cxResourceList) toXml() xmlResourceList {
	out := xmlResourceList{
		Src:                 rl.Src,
		URLBase:             rl.URLBase,
		SkinDir:             rl.SkinDir,
		BlobName:            rl.BlobName,
		PreserveBlobID:      rl.PreserveBlobID,
		BlobAttach:          rl.BlobAttach,
		MuxInclude:          rl.MuxInclude,
		MuxExclude:          rl.MuxExclude,
		TemplateFunctionMap: rl.TemplateFunctionMap,
		Escapes:             cxEscapesToXml(rl.Escapes),
	}
	for _, it := range rl.Items {
		out.Items = append(out.Items, xmlItem{
			Type:                it.Type,
			File:                it.File,
			Src:                 it.Src,
			URL:                 it.URL,
			AltURL:              it.AltURL,
			Key:                 it.Key,
			Escape:              it.Escape,
			TemplateFunctionMap: it.TemplateFunctionMap,
		})
	}
	for _, nested := range rl.ResourceLists {
		out.ResourceLists = append(out.ResourceLists, nested.toXml())
	}
	return out
}

func (ml *cxMockupList) toXml() xmlMockupList {
	out := xmlMockupList{
		SkinDir:  ml.SkinDir,
		SaveMode: ml.SaveMode,
		LineMode: ml.LineMode,
		Vars:     cxVarsToXml(ml.Vars),
	}
	for _, it := range ml.Items {
		out.Items = append(out.Items, xmlMockupItem{
			Src:      it.Src,
			Negative: it.Negative,
			SaveMode: it.SaveMode,
			Vars:     cxVarsToXml(it.Vars),
		})
	}
	return out
}

func (s *cxSource) toXml() *xmlSource {
	out := &xmlSource{Base: s.Base}
	for _, r := range s.Resources {
		out.Resources = append(out.Resources, xmlResource{
			ID:         r.ID,
			Src:        r.Src,
			Sha256:     r.Sha256,
			IgnoreHash: r.IgnoreHash,
		})
	}
	return out
}

func (o *cxOrigin) toXml() xmlOrigin {
	out := xmlOrigin{Name: o.Name}
	if o.Local != nil {
		out.Local = o.Local.toXml()
	}
	if o.Remote != nil {
		out.Remote = o.Remote.toXml()
	}
	return out
}

func (ex *cxExternal) toXml() xmlExternal {
	var out xmlExternal
	for _, it := range ex.Items {
		out.Items = append(out.Items, xmlExternalItem{
			Origin:  it.Origin,
			ID:      it.ID,
			Dstfile: it.Dstfile,
		})
	}
	return out
}

// --- the debug command: every catalog through the cargo path ---

// DebugCatalogs parses every *.miniskin.xml under content with the cargo
// path and writes the chosen outputs: echo is the byte-faithful rewrite
// (the preservation, live — and the migration's regression mirror: the
// parse gets more typed underneath, the echo must never change), rx the
// claimed-vs-cargo report (the progress map). Destinations are a file
// path or "-" for stdout; with neither given, the rx goes to stdout.
func DebugCatalogs(content, echoDst, xrayDst string) error {
	if echoDst == "" && xrayDst == "" {
		xrayDst = "-"
	}
	echoW, closeEcho, err := debugDest(echoDst)
	if err != nil {
		return err
	}
	defer closeEcho()
	xrayW, closeXray, err := debugDest(xrayDst)
	if err != nil {
		return err
	}
	defer closeXray()

	return filepath.WalkDir(content, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".miniskin.xml") {
			return err
		}
		root, err := parseMiniskinCargo(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if echoW != nil {
			fmt.Fprintf(echoW, "<!-- ========== %s ========== -->\n", filepath.ToSlash(path))
			if err := root.echoTo(echoW); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			fmt.Fprintln(echoW)
		}
		if xrayW != nil {
			root.xrayTo(xrayW, filepath.ToSlash(path))
		}
		return nil
	})
}

// debugDest resolves one output destination: "" is off, "-" is stdout,
// anything else a file.
func debugDest(dst string) (io.Writer, func(), error) {
	switch dst {
	case "":
		return nil, func() {}, nil
	case "-":
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(dst)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// echoTo re-emits the catalog byte-faithfully (cargoxml's Raw writer).
func (m *cxMiniskin) echoTo(w io.Writer) error {
	enc := xml.NewEncoder(w)
	e := cargoxml.NewEncoderWithCargo(enc)
	e.Raw = w // the same writer the xml.Encoder wraps: whitespace verbatim
	if err := e.Encode(m); err != nil {
		return err
	}
	return enc.Flush()
}

// xrayTo prints what the cargo parse claimed and what still rides the
// cargo — the map of the migration, one catalog at a time.
func (m *cxMiniskin) xrayTo(w io.Writer, path string) {
	fmt.Fprintf(w, "== %s\n", path)
	claimed := func(name, value string) {
		if value != "" {
			fmt.Fprintf(w, "   claimed  %s=%q\n", name, value)
		}
	}
	claimed("skin-dir", m.SkinDir)
	claimed("log", m.Log)
	claimed("mux-include", m.MuxInclude)
	claimed("mux-exclude", m.MuxExclude)
	for _, g := range m.Globals {
		fmt.Fprintf(w, "   claimed  <globals> — %d var(s)%s\n", len(g.Vars), commentNote(g.comments()))
	}
	for _, e := range m.Escapes {
		n := 0
		if e.Cargo != nil {
			n = countComments(e.Cargo.Trails)
		}
		fmt.Fprintf(w, "   claimed  <escape %s as %s>%s\n", e.Ext, e.As, commentNote(n))
	}
	for _, bl := range m.BucketLists {
		fmt.Fprintf(w, "   claimed  <bucket-list %s> — %d bucket(s), %d escape(s)\n", bl.Filename, len(bl.Buckets), len(bl.Escapes))
		cargoChildren(w, bl.Cargo, "bucket-list")
		for _, b := range bl.Buckets {
			fmt.Fprintf(w, "   claimed    <bucket %s -> %s>\n", b.Src, b.Dst)
			cargoChildren(w, b.Cargo, "bucket "+b.Src)
			for _, rl := range b.ResourceLists {
				rxResourceList(w, rl, "    ")
			}
			for _, ml := range b.MockupLists {
				rxMockupList(w, ml, "    ")
			}
		}
	}
	for _, rl := range m.ResourceLists {
		rxResourceList(w, rl, "")
	}
	for _, ml := range m.MockupLists {
		rxMockupList(w, ml, "")
	}
	for _, o := range m.Origins {
		sides := ""
		if o.Local != nil {
			sides += fmt.Sprintf(" local(%d res)", len(o.Local.Resources))
		}
		if o.Remote != nil {
			sides += fmt.Sprintf(" remote(%d res)", len(o.Remote.Resources))
		}
		fmt.Fprintf(w, "   claimed  <origin %s> —%s\n", o.Name, sides)
		cargoChildren(w, o.Cargo, "origin "+o.Name)
	}
	for _, ex := range m.Externals {
		fmt.Fprintf(w, "   claimed  <external> — %d item(s)\n", len(ex.Items))
		cargoChildren(w, ex.Cargo, "external")
	}
	if m.Cargo == nil {
		return
	}
	for _, a := range m.Cargo.MoreAttributes {
		fmt.Fprintf(w, "   cargo    attribute %s=%q\n", a.Name.Local, a.Value)
	}
	for _, c := range m.Cargo.MoreChildren {
		fmt.Fprintf(w, "   cargo    child <%s> (untyped)\n", c.Name.Local)
	}
	comments := countComments(m.Cargo.Trails)
	for _, c := range m.Cargo.MoreChildren {
		comments += genericComments(c)
	}
	if comments > 0 {
		fmt.Fprintf(w, "   cargo    %d comment(s)\n", comments)
	}
}

// rxResourceList prints one resource-list line and recurses into the
// nested ones, two spaces deeper each level.
func rxResourceList(w io.Writer, rl *cxResourceList, pad string) {
	label := rl.URLBase
	if label == "" {
		label = rl.Src
	}
	fmt.Fprintf(w, "   claimed  %s<resource-list %s> — %d item(s), %d escape(s)\n", pad, label, len(rl.Items), len(rl.Escapes))
	cargoChildren(w, rl.Cargo, "resource-list "+label)
	for _, nested := range rl.ResourceLists {
		rxResourceList(w, nested, pad+"  ")
	}
}

// rxMockupList prints one mockup-list line with its counts.
func rxMockupList(w io.Writer, ml *cxMockupList, pad string) {
	fmt.Fprintf(w, "   claimed  %s<mockup-list> — %d item(s), %d var(s)\n", pad, len(ml.Items), len(ml.Vars))
	cargoChildren(w, ml.Cargo, "mockup-list")
}

// cargoChildren lists what a claimed container still carries untyped — the
// map must never lose sight of it.
func cargoChildren(w io.Writer, cargo *cargoxml.CargoXml, owner string) {
	if cargo == nil {
		return
	}
	for _, c := range cargo.MoreChildren {
		fmt.Fprintf(w, "   cargo      child <%s> (untyped, inside <%s>)\n", c.Name.Local, owner)
	}
}

// commentNote renders the ", N comment(s)" tail of a claimed line — empty
// when there is nothing to tell.
func commentNote(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(", %d comment(s)", n)
}

// comments counts what rides this element's own cargo and its vars' — a
// comment before a claimed element travels in that element's cargo.
func (g *cxGlobals) comments() int {
	n := 0
	if g.Cargo != nil {
		n = countComments(g.Cargo.Trails)
	}
	for _, v := range g.Vars {
		if v.Cargo != nil {
			n += countComments(v.Cargo.Trails)
		}
	}
	return n
}

// countComments counts the comment trails in one list.
func countComments(trails *cargoxml.Trails) int {
	if trails == nil {
		return 0
	}
	n := 0
	for _, trail := range *trails {
		if trail.Type == cargoxml.TrailComment {
			n++
		}
	}
	return n
}

// genericComments counts the comments riding a whole untyped subtree — a
// comment before a child belongs to that child's trails, so the honest
// number walks the tree.
func genericComments(item *cargoxml.GenericXmlItem) int {
	n := countComments(item.Trails)
	for _, child := range item.Children {
		n += genericComments(child)
	}
	return n
}
