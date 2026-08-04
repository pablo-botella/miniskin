package miniskin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablo-botella/mskblob"
)

// BlobIndex is what codegen needs to wire a blob into the generated binary. The
// entries themselves live in the blob file (read at runtime by the blob
// package), so only the identity/wiring is carried here.
type BlobIndex struct {
	Name   string // blob name (matches blob-name / the data file stem)
	Base   string // URL base the blob is served under (from urlbase)
	ID     string // guid sync token
	Attach string // blob-attach value (assets/mux/templates/* or "")
}

// splitAttach splits a blob-attach expression into its tokens. The expression is
// comma/space separated, so a single blob can attach to several targets at once
// (e.g. "mux,templates" serves its static entries and parses its template ones).
func splitAttach(attach string) []string {
	return strings.FieldsFunc(attach, func(r rune) bool { return r == ',' || r == ' ' })
}

// validateBlobAttach rejects unknown blob-attach tokens up front (a typo would
// otherwise pack the blob but silently wire nothing). An empty expression is
// valid: the blob is stored and the app wires it itself via Blob(id).
func validateBlobAttach(attach string) error {
	for _, t := range splitAttach(attach) {
		switch t {
		case "mux", "assets", "templates":
		default:
			return fmt.Errorf("unknown blob-attach %q (valid: mux, assets, templates)", t)
		}
	}
	return nil
}

// restypeFromType maps an item's comma-separated type= into blob restype flags.
func restypeFromType(t string) mskblob.RestType {
	var r mskblob.RestType
	for _, f := range strings.Split(t, ",") {
		switch strings.TrimSpace(f) {
		case "static":
			r |= mskblob.Static
		case "html-template":
			r |= mskblob.HTMLTemplate
		case "parse":
			r |= mskblob.Parse
		case "response":
			r |= mskblob.Response
		case "nomux":
			r |= mskblob.Nomux
		}
	}
	return r
}

// blobBase is the URL base a blob is served under: the item's urlBase with a
// trailing slash (entry URLs are stored relative to it).
func blobBase(it Item) string {
	p := it.urlBase
	if p == "" {
		p = "/"
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// packBlobs separates blob-packed items from the normal embed items, packs each
// blob into blob-out/<name> via the blob package, and returns the remaining
// embed items plus the blob indexes for codegen.
func (ms *Miniskin) packBlobs(bl BucketList, items []Item) (embedItems []Item, blobs []BlobIndex, err error) {
	type group struct {
		base     string
		pinnedID string
		attach   string
		inputs   []mskblob.Item
	}
	var order []string
	groups := map[string]*group{}
	for _, it := range items {
		if it.BlobName == "" {
			embedItems = append(embedItems, it)
			continue
		}
		if err := validateBlobAttach(it.BlobAttach); err != nil {
			return nil, nil, fmt.Errorf("blob %q: %w", it.BlobName, err)
		}
		base := blobBase(it)
		route := it.RouteURL()
		if !strings.HasPrefix(route, base) {
			return nil, nil, fmt.Errorf("blob %q: route %q is not under the blob base %q", it.BlobName, route, base)
		}
		g := groups[it.BlobName]
		if g == nil {
			g = &group{base: base, pinnedID: it.PreserveBlobID, attach: it.BlobAttach}
			groups[it.BlobName] = g
			order = append(order, it.BlobName)
		}
		g.inputs = append(g.inputs, mskblob.Item{
			Key:      it.Key,
			URL:      strings.TrimPrefix(route, base),
			Filename: it.File,
			RestType: restypeFromType(it.Type),
			Src:      absPath(it.filePath()),
		})
	}
	if len(order) == 0 {
		return embedItems, nil, nil
	}

	blobOut := bl.BlobOut
	if blobOut == "" {
		blobOut = "blob"
	}
	blobDir := absPath(filepath.Join(ms.contentPath, filepath.FromSlash(blobOut)))

	for _, name := range order {
		g := groups[name]
		idx, err := ms.packOneBlob(filepath.Join(blobDir, name), name, g.base, g.pinnedID, g.attach, g.inputs)
		if err != nil {
			return nil, nil, err
		}
		blobs = append(blobs, idx)
	}
	return embedItems, blobs, nil
}

// packOneBlob returns a blob's codegen index. With preserve-blob-if-id the guid
// is strict: the on-disk blob must carry exactly that guid (cache hit — only the
// header is read, no source files); a mismatch or missing blob is an error.
// Without a pin the blob is (re)built with a fresh guid.
func (ms *Miniskin) packOneBlob(dst, name, base, pinnedID, attach string, inputs []mskblob.Item) (BlobIndex, error) {
	if pinnedID != "" {
		hdr, err := mskblob.ReadHeader(dst)
		if err != nil {
			return BlobIndex{}, fmt.Errorf("preserve-blob-if-id %q=%s: cannot read blob %s: %w (build it without the pin first)", name, pinnedID, dst, err)
		}
		if hdr.ID != pinnedID {
			return BlobIndex{}, fmt.Errorf("preserve-blob-if-id %q: blob %s has id %s, expected %s (regenerate without the pin, then update the id)", name, dst, hdr.ID, pinnedID)
		}
		ms.logf("blob %s: cache hit (id %s) — prebuild skipped", name, pinnedID)
		return BlobIndex{Name: name, Base: base, ID: pinnedID, Attach: attach}, nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return BlobIndex{}, fmt.Errorf("creating blob dir for %q: %w", name, err)
	}
	id, err := mskblob.Write(dst, inputs, mskblob.Options{})
	if err != nil {
		return BlobIndex{}, err
	}
	ms.logf("blob %s: rebuilt %d entries (id %s) -> %s", name, len(inputs), id, dst)
	return BlobIndex{Name: name, Base: base, ID: id, Attach: attach}, nil
}
