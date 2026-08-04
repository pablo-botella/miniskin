package miniskin

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// --- XML structures

type xmlMiniskin struct {
	XMLName       xml.Name          `xml:"miniskin"`
	SkinDir       string            `xml:"skin-dir,attr,omitempty"`
	Log           string            `xml:"log,attr,omitempty"`
	MuxInclude    string            `xml:"mux-include,attr,omitempty"`
	MuxExclude    string            `xml:"mux-exclude,attr,omitempty"`
	Globals       []xmlVar          `xml:"globals>var,omitempty"`
	Escapes       []xmlEscape       `xml:"escape,omitempty"`
	BucketList    *xmlBucketList    `xml:"bucket-list,omitempty"`
	ResourceLists []xmlResourceList `xml:"resource-list,omitempty"`
	MockupList    *xmlMockupList    `xml:"mockup-list,omitempty"`
	Origins       []xmlOrigin       `xml:"origin,omitempty"`
	External      *xmlExternal      `xml:"external,omitempty"`
}

type xmlOrigin struct {
	Name   string     `xml:"name,attr"`
	Local  *xmlSource `xml:"local,omitempty"`
	Remote *xmlSource `xml:"remote,omitempty"`
}

// xmlSource is one side of an origin: a base path (local dir) or base URL
// (remote), plus the resources it offers, each keyed by id (default id = src).
type xmlSource struct {
	Base      string        `xml:"base,attr,omitempty"`
	Resources []xmlResource `xml:"resource,omitempty"`
}

type xmlResource struct {
	ID         string `xml:"id,attr,omitempty"`
	Src        string `xml:"src,attr"`
	Sha256     string `xml:"sha256,attr,omitempty"`
	IgnoreHash string `xml:"ignorehash,attr,omitempty"`
}

type xmlExternal struct {
	Items []xmlExternalItem `xml:"external-item"`
}

type xmlExternalItem struct {
	Origin  string `xml:"origin,attr"`
	ID      string `xml:"id,attr"`
	Dstfile string `xml:"dstfile,attr"`
}

type xmlMockupList struct {
	SkinDir  string          `xml:"skin-dir,attr,omitempty"`
	SaveMode string          `xml:"save-mode,attr,omitempty"`
	LineMode string          `xml:"line-mode,attr,omitempty"`
	Vars     []xmlVar        `xml:"var,omitempty"`
	Items    []xmlMockupItem `xml:"item,omitempty"`
}

type xmlMockupItem struct {
	Src      string   `xml:"src,attr"`
	Negative string   `xml:"negative,attr,omitempty"`
	SaveMode string   `xml:"save-mode,attr,omitempty"`
	Vars     []xmlVar `xml:"var,omitempty"`
}

type xmlVar struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type xmlEscape struct {
	Ext string `xml:"ext,attr"`
	As  string `xml:"as,attr"`
}

type xmlBucketList struct {
	Filename    string      `xml:"filename,attr"`
	Module      string      `xml:"module,attr"`
	Import      string      `xml:"import,attr,omitempty"`
	Template    string      `xml:"template,attr,omitempty"`
	ProjectRoot string      `xml:"project-root,attr,omitempty"`
	BlobOut     string      `xml:"blob-out,attr,omitempty"`
	MuxInclude  string      `xml:"mux-include,attr,omitempty"`
	MuxExclude  string      `xml:"mux-exclude,attr,omitempty"`
	Omit        string      `xml:"omit,attr,omitempty"`
	Escapes     []xmlEscape `xml:"escape,omitempty"`
	Buckets     []xmlBucket `xml:"bucket"`
}

type xmlBucket struct {
	Src                string             `xml:"src,attr"`
	Dst                string             `xml:"dst,attr"`
	ModuleName         string             `xml:"module-name,attr"`
	RecurseFolder      string             `xml:"recurse-folder,attr,omitempty"`
	Template           string             `xml:"template,attr,omitempty"`
	SkinDir            string             `xml:"skin-dir,attr,omitempty"`
	MuxInclude         string             `xml:"mux-include,attr,omitempty"`
	MuxExclude         string             `xml:"mux-exclude,attr,omitempty"`
	TemplateFunctionMap string            `xml:"template-function-map,attr,omitempty"`
	Escapes            []xmlEscape        `xml:"escape,omitempty"`
	ResourceLists      []xmlResourceList  `xml:"resource-list,omitempty"`
	MockupLists        []xmlMockupList    `xml:"mockup-list,omitempty"`
}

type xmlResourceList struct {
	Src                 string             `xml:"src,attr,omitempty"`
	URLBase             string             `xml:"urlbase,attr,omitempty"`
	SkinDir             string             `xml:"skin-dir,attr,omitempty"`
	BlobName            string             `xml:"blob-name,attr,omitempty"`
	PreserveBlobID      string             `xml:"preserve-blob-if-id,attr,omitempty"`
	BlobAttach          string             `xml:"blob-attach,attr,omitempty"`
	MuxInclude          string             `xml:"mux-include,attr,omitempty"`
	MuxExclude          string             `xml:"mux-exclude,attr,omitempty"`
	TemplateFunctionMap string             `xml:"template-function-map,attr,omitempty"`
	Escapes             []xmlEscape        `xml:"escape,omitempty"`
	Items               []xmlItem          `xml:"item,omitempty"`
	ResourceLists       []xmlResourceList  `xml:"resource-list,omitempty"`
}

type xmlItem struct {
	Type                string `xml:"type,attr,omitempty"`
	File                string `xml:"file,attr,omitempty"`
	Src                 string `xml:"src,attr,omitempty"`
	URL                 string `xml:"url,attr,omitempty"`
	AltURL              string `xml:"alt-url-abs,attr,omitempty"`
	Key                 string `xml:"key,attr,omitempty"`
	Escape              string `xml:"escape,attr,omitempty"`
	TemplateFunctionMap string `xml:"template-function-map,attr,omitempty"`
}

// --- Parsed types

// BucketList holds the embed generation config from the root miniskin.xml.
type BucketList struct {
	Filename    string
	Module      string
	Import      string
	Template    string // custom template file for embed generation
	ProjectRoot string // project root relative to contentPath (for resolving dst)
	BlobOut     string // output dir for .blob files (relative to project-root), default "blob"
	Omit        string // comma/space-separated codegen outputs to skip ("embed", "module")
	Buckets     []Bucket
}

// OmitsCodegen reports whether the given codegen output is listed in the Omit attribute.
// Recognised keys: "embed" (skip the embed file), "module" (skip per-bucket module files).
func (bl BucketList) OmitsCodegen(key string) bool {
	return slices.Contains(strings.FieldsFunc(bl.Omit, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	}), key)
}

// Bucket represents a content bucket.
type Bucket struct {
	Src        string
	Dst        string
	ModuleName string
	Template   string // custom template file for bucket generation
	TemplateFunctionMap string // expression for template.FuncMap (e.g. "MyFuncMap()")
	recurse             bool
	skinDir             string
	muxInclude          string             // resolved cascaded mux-include pattern (default: "*")
	muxExclude          string             // resolved cascaded mux-exclude pattern (default: "")
	escapeRules         []xmlEscape        // cascaded escape rules
	inlineResourceLists []xmlResourceList  // resource-lists defined inline in the bucket element
	inlineMockupLists   []xmlMockupList    // mockup-lists defined inline in the bucket element
}

// Item represents a resource item from a miniskin.xml.
type Item struct {
	Type      string
	File      string
	Src       string // source file; if set, item needs processing
	URL       string
	AltURL    string
	Key       string
	Index     int    // position in the global embed list
	EmbedPath    string // relative path for go:embed, computed during processing
	BlobName       string // if set, item is packed into this .blob instead of embedded
	PreserveBlobID string // preserve-blob-if-id: trust the existing blob iff its guid equals this
	BlobAttach     string // blob-attach: which exe structures to wire the blob into (assets/mux/templates/*)
	XMLSrc       string // absolute path of the .miniskin.xml that declared this item
	XMLLine   int    // line number in XMLSrc where this item is declared
	TemplateFunctionMap string // cascaded expression for template.FuncMap
	urlBase     string
	skinDir     string
	dir         string
	escapeRules []xmlEscape // cascaded escape rules
	escape      string      // item-level escape override
}

// findItemLine returns the 1-based line number of the first <item ... file="filename" ...> in data.
func findItemLine(data []byte, filename string) int {
	needle := []byte(`file="` + filename + `"`)
	line := 1
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			line++
			continue
		}
		if data[i] == needle[0] && i+len(needle) <= len(data) {
			match := true
			for j := 1; j < len(needle); j++ {
				if data[i+j] != needle[j] {
					match = false
					break
				}
			}
			if match {
				return line
			}
		}
	}
	return 0
}

// NeedsProcessing returns true if the item has a src attribute.
func (it *Item) NeedsProcessing() bool {
	return it.Src != ""
}

func (it *Item) filePath() string {
	return filepath.Join(it.dir, it.File)
}

func (it *Item) srcPath() string {
	return filepath.Join(it.dir, it.Src)
}

// RouteURL returns the URL for serving this item.
// Uses Key if set, otherwise URLBase + "/" + File.
func (it *Item) RouteURL() string {
	if it.Key != "" {
		return it.Key
	}
	if it.urlBase != "" {
		return it.urlBase + "/" + it.File
	}
	return "/" + it.File
}

// HasFlag returns true if the item's Type contains the given flag.
func (it *Item) HasFlag(flag string) bool {
	for _, f := range strings.Split(it.Type, ",") {
		if strings.TrimSpace(f) == flag {
			return true
		}
	}
	return false
}

// --- Mux cascade helpers

// cascadeMux returns child if non-empty, otherwise parent.
func cascadeMux(parent, child string) string {
	if child != "" {
		return child
	}
	return parent
}

// matchesMuxPattern checks if filename matches a comma-separated list of glob patterns.
func matchesMuxPattern(filename, patterns string) bool {
	if patterns == "" {
		return false
	}
	if patterns == "*" {
		return true
	}
	for _, p := range strings.Split(patterns, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if matched, _ := filepath.Match(p, filename); matched {
			return true
		}
	}
	return false
}

// isExcludedByMux returns true if the file should get the nomux flag
// based on resolved mux-include and mux-exclude patterns.
func isExcludedByMux(filename, muxInclude, muxExclude string) bool {
	if !matchesMuxPattern(filename, muxInclude) {
		return true
	}
	if muxExclude != "" && matchesMuxPattern(filename, muxExclude) {
		return true
	}
	return false
}

// hasTypeFlag checks if a comma-separated type string contains a specific flag.
func hasTypeFlag(typ, flag string) bool {
	for _, f := range strings.Split(typ, ",") {
		if strings.TrimSpace(f) == flag {
			return true
		}
	}
	return false
}

// --- Parsing

// parseMiniskinXML parses one catalog through the cargoxml path (every
// foreign attribute, element and comment preserved in the cx tree) and
// bridges it to the xml* DTOs the engine consumes. The encoding/xml
// Unmarshal is gone; the structs' xml tags are vestigial.
func parseMiniskinXML(path string) (*xmlMiniskin, error) {
	path = absPath(path)
	root, err := parseMiniskinCargo(path)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return root.toXml(), nil
}

func parseBucketList(xbl *xmlBucketList, defaultSkinDir string, parentMuxInclude, parentMuxExclude string, parentEscapeRules []xmlEscape) BucketList {
	// Cascade mux: parent (miniskin) → bucket-list
	blInclude := cascadeMux(parentMuxInclude, xbl.MuxInclude)
	blExclude := cascadeMux(parentMuxExclude, xbl.MuxExclude)
	blEscapes := cascadeEscapeRules(parentEscapeRules, xbl.Escapes)

	bl := BucketList{
		Filename:    xbl.Filename,
		Module:      xbl.Module,
		Import:      xbl.Import,
		Template:    xbl.Template,
		ProjectRoot: xbl.ProjectRoot,
		BlobOut:     xbl.BlobOut,
		Omit:        xbl.Omit,
		Buckets:     make([]Bucket, len(xbl.Buckets)),
	}
	for i, b := range xbl.Buckets {
		skinDir := b.SkinDir
		if skinDir == "" {
			skinDir = defaultSkinDir
		}
		bl.Buckets[i] = Bucket{
			Src:                 b.Src,
			Dst:                 b.Dst,
			ModuleName:          b.ModuleName,
			Template:            b.Template,
			TemplateFunctionMap: b.TemplateFunctionMap,
			recurse:             b.RecurseFolder == "all",
			skinDir:             skinDir,
			muxInclude:          cascadeMux(blInclude, b.MuxInclude),
			muxExclude:          cascadeMux(blExclude, b.MuxExclude),
			escapeRules:         cascadeEscapeRules(blEscapes, b.Escapes),
			inlineResourceLists: b.ResourceLists,
			inlineMockupLists:   b.MockupLists,
		}
	}
	return bl
}

func parseGlobals(vars []xmlVar) map[string]string {
	m := make(map[string]string, len(vars))
	for _, v := range vars {
		m[v.Name] = v.Value
	}
	return m
}

func parseResourceList(xrl *xmlResourceList, dir string, xmlFile string, xmlData []byte, defaultSkinDir string, parentMuxInclude, parentMuxExclude string, parentEscapeRules []xmlEscape, parentTemplateFunctionMap, parentBlobName, parentPreserveBlob, parentBlobAttach string) []Item {
	// Resolve dir: if this resource-list has a src, adjust dir
	if xrl.Src != "" {
		dir = filepath.Join(dir, xrl.Src)
	}

	skinDir := xrl.SkinDir
	if skinDir == "" {
		skinDir = defaultSkinDir
	}
	// Cascade mux: parent → resource-list
	rlInclude := cascadeMux(parentMuxInclude, xrl.MuxInclude)
	rlExclude := cascadeMux(parentMuxExclude, xrl.MuxExclude)
	rlEscapes := cascadeEscapeRules(parentEscapeRules, xrl.Escapes)
	rlTemplateFunctionMap := cascadeMux(parentTemplateFunctionMap, xrl.TemplateFunctionMap)
	rlBlobName := cascadeMux(parentBlobName, xrl.BlobName)
	rlPreserveBlobID := cascadeMux(parentPreserveBlob, xrl.PreserveBlobID)
	rlBlobAttach := cascadeMux(parentBlobAttach, xrl.BlobAttach)

	var items []Item
	for _, xi := range xrl.Items {
		itemType := xi.Type
		// Apply nomux automatically if item is excluded by mux patterns
		if !hasTypeFlag(itemType, "nomux") && isExcludedByMux(xi.File, rlInclude, rlExclude) {
			if itemType == "" {
				itemType = "nomux"
			} else {
				itemType += ",nomux"
			}
		}
		items = append(items, Item{
			Type:                itemType,
			File:                xi.File,
			Src:                 xi.Src,
			URL:                 xi.URL,
			AltURL:              xi.AltURL,
			Key:                 xi.Key,
			BlobName:            rlBlobName,
			PreserveBlobID:      rlPreserveBlobID,
			BlobAttach:          rlBlobAttach,
			XMLSrc:              xmlFile,
			XMLLine:             findItemLine(xmlData, xi.File),
			TemplateFunctionMap: cascadeMux(rlTemplateFunctionMap, xi.TemplateFunctionMap),
			urlBase:             xrl.URLBase,
			skinDir:             skinDir,
			dir:                 dir,
			escapeRules:         rlEscapes,
			escape:              xi.Escape,
		})
	}

	// Recurse into nested resource-lists
	for i := range xrl.ResourceLists {
		childItems := parseResourceList(&xrl.ResourceLists[i], dir, xmlFile, xmlData, skinDir, rlInclude, rlExclude, rlEscapes, rlTemplateFunctionMap, rlBlobName, rlPreserveBlobID, rlBlobAttach)
		items = append(items, childItems...)
	}

	return items
}

// walkBucket walks a bucket's source directory, finds all *.miniskin.xml files,
// parses them, and calls fn for each parsed result.
func (ms *Miniskin) walkBucket(bucket Bucket, fn func(parsed *xmlMiniskin, dir string, xmlFile string) error) error {
	srcDir := resolveSrcPath(bucket.Src, ms.contentPath, ms.contentPath)

	// Process inline resource-lists and mockup-lists from the bucket element
	if len(bucket.inlineResourceLists) > 0 || len(bucket.inlineMockupLists) > 0 {
		synthetic := &xmlMiniskin{
			ResourceLists: bucket.inlineResourceLists,
		}
		if len(bucket.inlineMockupLists) > 0 {
			synthetic.MockupList = &bucket.inlineMockupLists[0]
		}
		if err := fn(synthetic, srcDir, ""); err != nil {
			return err
		}
		for i := 1; i < len(bucket.inlineMockupLists); i++ {
			ml := bucket.inlineMockupLists[i]
			s := &xmlMiniskin{MockupList: &ml}
			if err := fn(s, srcDir, ""); err != nil {
				return err
			}
		}
	}

	visitFile := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".miniskin.xml") {
			parsed, err := parseMiniskinXML(path)
			if err != nil {
				return err
			}
			return fn(parsed, filepath.Dir(path), absPath(path))
		}
		return nil
	}

	if bucket.recurse {
		return filepath.Walk(srcDir, visitFile)
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", absPath(srcDir), err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".miniskin.xml") {
			info, err := e.Info()
			if err != nil {
				return err
			}
			path := filepath.Join(srcDir, e.Name())
			if err := visitFile(path, info, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// collectItems walks the bucket's source directory and returns all resource-list items.
// Mux-include/mux-exclude patterns from the bucket are cascaded to each resource-list.
func (ms *Miniskin) collectItems(bucket Bucket) ([]Item, error) {
	var result []Item
	err := ms.walkBucket(bucket, func(parsed *xmlMiniskin, dir string, xmlFile string) error {
		var xmlData []byte
		if xmlFile != "" {
			xmlData, _ = os.ReadFile(xmlFile)
		}
		for i := range parsed.ResourceLists {
			items := parseResourceList(&parsed.ResourceLists[i], dir, xmlFile, xmlData, bucket.skinDir, bucket.muxInclude, bucket.muxExclude, bucket.escapeRules, bucket.TemplateFunctionMap, "", "", "")
			result = append(result, items...)
		}
		return nil
	})
	return result, err
}
