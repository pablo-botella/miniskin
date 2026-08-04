package miniskin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// originFilename is the per-developer override registry (gitignored): it redefines
// committed origins for this machine, typically supplying a <local> that wins.
const originFilename = "miniskin-origin.xml"

// resourceHashFilename is the tool-managed cache of computed hashes for remote
// resources that carry no inline sha256 contract. It can be gitignored; delete it
// by hand to force a fresh re-fetch of every non-contract resource.
const resourceHashFilename = "miniskin-resource-hash.xml"

// Origin is a named source with a Local side (a directory on this machine) and/or
// a Remote side (a base URL). Resolution is per resource id; Local wins over Remote.
type Origin struct {
	Name   string
	Local  *OriginSource
	Remote *OriginSource
}

// OriginSource is one side of an origin: a base plus resources keyed by id.
type OriginSource struct {
	Base      string
	Resources map[string]OriginResource
}

// OriginResource is one offered resource. Src is the file name under the source's
// base (id defaults to it). Sha256, set on a remote resource, is the author's
// "contract"; IgnoreHash skips verification (local dev builds).
type OriginResource struct {
	ID         string
	Src        string
	Sha256     string
	IgnoreHash bool
}

func (s *OriginSource) resource(id string) (OriginResource, bool) {
	if s == nil {
		return OriginResource{}, false
	}
	r, ok := s.Resources[id]
	return r, ok
}

// xmlTruthy reads an XML attribute value as a boolean (absent/falsey → false).
func xmlTruthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// parseSource builds an OriginSource, keying resources by id (id defaults to src).
func parseSource(xs *xmlSource, where string) (*OriginSource, error) {
	if xs == nil {
		return nil, nil
	}
	src := &OriginSource{Base: xs.Base, Resources: make(map[string]OriginResource, len(xs.Resources))}
	for _, xr := range xs.Resources {
		if xr.Src == "" {
			return nil, fmt.Errorf("origin %s: resource with empty src", where)
		}
		id := xr.ID
		if id == "" {
			id = xr.Src
		}
		src.Resources[id] = OriginResource{ID: id, Src: xr.Src, Sha256: xr.Sha256, IgnoreHash: xmlTruthy(xr.IgnoreHash)}
	}
	return src, nil
}

// mergeOrigin folds an xmlOrigin into dst by name; a Local/Remote side replaces an
// earlier one of the same kind (so miniskin-origin.xml's <local> augments/overrides
// the committed origin's <remote>).
func mergeOrigin(dst map[string]Origin, xo xmlOrigin) error {
	if xo.Name == "" {
		return fmt.Errorf("origin with empty name")
	}
	o := dst[xo.Name]
	o.Name = xo.Name
	if xo.Local != nil {
		s, err := parseSource(xo.Local, xo.Name+" <local>")
		if err != nil {
			return err
		}
		o.Local = s
	}
	if xo.Remote != nil {
		s, err := parseSource(xo.Remote, xo.Name+" <remote>")
		if err != nil {
			return err
		}
		o.Remote = s
	}
	dst[xo.Name] = o
	return nil
}

// loadOrigins builds the origin map: committed origins from the root content file
// (typically the <remote> defaults) merged with the per-developer overrides in
// miniskin-origin.xml (typically <local>, which wins).
func (ms *Miniskin) loadOrigins(rootOrigins []xmlOrigin) (map[string]Origin, error) {
	origins := map[string]Origin{}
	for _, xo := range rootOrigins {
		if err := mergeOrigin(origins, xo); err != nil {
			return nil, err
		}
	}
	path := filepath.Join(ms.contentPath, originFilename)
	if _, err := os.Stat(path); err == nil {
		parsed, err := parseMiniskinXML(path)
		if err != nil {
			return nil, err
		}
		for _, xo := range parsed.Origins {
			if err := mergeOrigin(origins, xo); err != nil {
				return nil, fmt.Errorf("in %s: %w", absPath(path), err)
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %s: %w", absPath(path), err)
	}
	return origins, nil
}

// processExternals walks all buckets, finds <external> blocks in any .miniskin.xml,
// and materialises each <external-item> (copy from a local origin, or download from
// a remote origin) into place.
func (ms *Miniskin) processExternals(bl BucketList, origins map[string]Origin) error {
	count := 0
	for _, bucket := range bl.Buckets {
		err := ms.walkBucket(bucket, func(parsed *xmlMiniskin, dir string, xmlFile string) error {
			if parsed.External == nil {
				return nil
			}
			for _, ext := range parsed.External.Items {
				if err := ms.resolveExternal(ext, dir, xmlFile, origins); err != nil {
					return err
				}
				count++
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if count > 0 {
		ms.logVerbose("externals processed: %d", count)
	}
	return nil
}

// resolveExternal materialises one external-item to its dstfile. Local wins over
// remote per resource id; a declared-but-missing local, or a failed fetch, is an
// error.
func (ms *Miniskin) resolveExternal(ext xmlExternalItem, dir, xmlFile string, origins map[string]Origin) error {
	if ext.Origin == "" {
		return fmt.Errorf("external-item missing origin attribute in %s", xmlFile)
	}
	if ext.ID == "" {
		return fmt.Errorf("external-item missing id attribute in %s", xmlFile)
	}
	origin, ok := origins[ext.Origin]
	if !ok {
		return fmt.Errorf("external-item origin %q not found\n\t(declared in %s)", ext.Origin, xmlFile)
	}
	dst := absPath(filepath.Join(dir, ext.Dstfile))

	// Local wins.
	if res, ok := origin.Local.resource(ext.ID); ok {
		local := filepath.Join(origin.Local.Base, res.Src)
		if !filepath.IsAbs(local) {
			local = filepath.Join(ms.contentPath, local)
		}
		if _, err := os.Stat(local); err != nil {
			return fmt.Errorf("external %s/%s: local resource missing: %s\n\t(declared in %s)", ext.Origin, ext.ID, absPath(local), xmlFile)
		}
		changed, err := copyIfChanged(local, dst)
		if err != nil {
			return fmt.Errorf("external %s/%s: %w\n\t(declared in %s)", ext.Origin, ext.ID, err, xmlFile)
		}
		if changed {
			ms.logf("  external: %s/%s (local) → %s", ext.Origin, ext.ID, ext.Dstfile)
		} else {
			ms.logVerbose("  external: %s/%s (local, unchanged)", ext.Origin, ext.ID)
		}
		return nil
	}

	// Otherwise remote.
	if res, ok := origin.Remote.resource(ext.ID); ok {
		return ms.fetchRemote(ext, res, origin.Remote.Base, dst, xmlFile)
	}

	return fmt.Errorf("external-item: origin %s has no resource id %q\n\t(declared in %s)", ext.Origin, ext.ID, xmlFile)
}

// fetchRemote downloads a remote resource into dst, verifying it against its inline
// sha256 contract, or (contract-less) against the cached hash — recording the hash
// on first use. It only downloads when needed (dst missing, contract mismatch, or
// no cache entry).
func (ms *Miniskin) fetchRemote(ext xmlExternalItem, res OriginResource, base, dst, xmlFile string) error {
	url := joinURL(base, res.Src)

	// Expected hash: the inline contract, else the cache (same url).
	expected := res.Sha256
	contract := expected != ""
	if !contract && !res.IgnoreHash {
		if h, ok := ms.resourceHashes[ext.ID]; ok && h.URL == url {
			expected = h.Sha256
		}
	}

	// Skip when dst already satisfies expectations.
	if !res.IgnoreHash {
		if contract {
			if sum, err := fileSha256Hex(dst); err == nil && strings.EqualFold(sum, expected) {
				ms.logVerbose("  external: %s/%s (remote, contract ok)", ext.Origin, ext.ID)
				return nil
			}
		} else if _, err := os.Stat(dst); err == nil {
			// no contract: only re-fetch if the cache entry is gone (manual cleanup) or dst missing
			if _, ok := ms.resourceHashes[ext.ID]; ok {
				ms.logVerbose("  external: %s/%s (remote, cached)", ext.Origin, ext.ID)
				return nil
			}
		}
	}

	data, sum, err := downloadURL(url)
	if err != nil {
		return fmt.Errorf("external %s/%s: fetch %s: %w\n\t(declared in %s)", ext.Origin, ext.ID, url, err, xmlFile)
	}
	if !res.IgnoreHash && expected != "" && !strings.EqualFold(sum, expected) {
		from := "sha256 contract"
		if !contract {
			from = resourceHashFilename
		}
		return fmt.Errorf("external %s/%s: sha256 mismatch for %s\n\texpected %s (%s)\n\tgot      %s\n\t(an official versioned resource changed — possible tampering)", ext.Origin, ext.ID, url, expected, from, sum)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("external %s/%s: mkdir %s: %w", ext.Origin, ext.ID, absPath(filepath.Dir(dst)), err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("external %s/%s: writing %s: %w", ext.Origin, ext.ID, absPath(dst), err)
	}
	ms.logf("  external: %s/%s %s → %s (%d bytes)", ext.Origin, ext.ID, url, ext.Dstfile, len(data))

	// Cache the hash for contract-less resources (trust-on-first-use).
	if !contract && !res.IgnoreHash {
		if cur, ok := ms.resourceHashes[ext.ID]; !ok || cur.URL != url || cur.Sha256 != sum {
			ms.resourceHashes[ext.ID] = hashEntry{URL: url, Sha256: sum}
			ms.resourceHashesDirty = true
		}
	}
	return nil
}

// --- resource hash cache (the "check" file, for contract-less remotes)

type hashEntry struct {
	URL    string
	Sha256 string
}

type xmlHash struct {
	ID     string `xml:"id,attr"`
	URL    string `xml:"url,attr"`
	Sha256 string `xml:"sha256,attr"`
}

type xmlResourceHashFile struct {
	XMLName xml.Name  `xml:"resource-hash"`
	Hashes  []xmlHash `xml:"hash"`
}

// loadResourceHashes reads the cache (miniskin-resource-hash.xml) if present.
func (ms *Miniskin) loadResourceHashes() error {
	ms.resourceHashes = map[string]hashEntry{}
	ms.resourceHashesDirty = false
	path := filepath.Join(ms.contentPath, resourceHashFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", absPath(path), err)
	}
	var f xmlResourceHashFile
	if err := xml.Unmarshal([]byte(stripBOM(string(data))), &f); err != nil {
		return fmt.Errorf("parsing %s: %w", absPath(path), err)
	}
	for _, h := range f.Hashes {
		if h.ID == "" {
			continue
		}
		ms.resourceHashes[h.ID] = hashEntry{URL: h.URL, Sha256: h.Sha256}
	}
	return nil
}

// flushResourceHashes rewrites the cache when it changed during the build.
func (ms *Miniskin) flushResourceHashes() error {
	if !ms.resourceHashesDirty {
		return nil
	}
	ids := make([]string, 0, len(ms.resourceHashes))
	for id := range ms.resourceHashes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var f xmlResourceHashFile
	for _, id := range ids {
		h := ms.resourceHashes[id]
		f.Hashes = append(f.Hashes, xmlHash{ID: id, URL: h.URL, Sha256: h.Sha256})
	}
	body, err := xml.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	out := append([]byte(xml.Header), body...)
	out = append(out, '\n')
	path := filepath.Join(ms.contentPath, resourceHashFilename)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", absPath(path), err)
	}
	ms.resourceHashesDirty = false
	ms.logVerbose("resource hashes written: %s", absPath(path))
	return nil
}

// --- helpers

// joinURL joins a base URL and a relative src.
func joinURL(base, src string) string {
	if base == "" {
		return src
	}
	if strings.HasSuffix(base, "/") {
		return base + src
	}
	return base + "/" + src
}

var fetchClient = &http.Client{Timeout: 120 * time.Second}

// downloadURL GETs url (following redirects) and returns the body plus its
// lowercase hex sha256.
func downloadURL(url string) ([]byte, string, error) {
	resp, err := fetchClient.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return data, sha256Hex(data), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fileSha256Hex(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

// copyIfChanged copies src to dst if dst is missing or differs in size/mtime from
// src. Returns true if a copy occurred. Creates dst's parent directory.
func copyIfChanged(src, dst string) (bool, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false, fmt.Errorf("source not found: %w", err)
	}
	if srcInfo.IsDir() {
		return false, fmt.Errorf("source is a directory: %s", absPath(src))
	}
	if dstInfo, err := os.Stat(dst); err == nil {
		if dstInfo.Size() == srcInfo.Size() && dstInfo.ModTime().Equal(srcInfo.ModTime()) {
			return false, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", absPath(filepath.Dir(dst)), err)
	}
	in, err := os.Open(src)
	if err != nil {
		return false, err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return false, err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return false, err
	}
	if err := out.Close(); err != nil {
		return false, err
	}
	if err := os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		return false, err
	}
	return true, nil
}
