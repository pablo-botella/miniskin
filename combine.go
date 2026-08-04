package miniskin

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// dirNode represents a directory in the combine tree.
type dirNode struct {
	name     string // directory name (leaf, not full path)
	parsed   *xmlMiniskin
	xmlPath  string // original XML file path (for removal)
	children []*dirNode
}

// CombineDir reads all *.miniskin.xml files under targetDir recursively,
// builds a nested structure, and writes a single combined XML at targetDir.
// The old XML files in subdirectories are removed.
func CombineDir(targetDir string) error {
	targetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}

	// Collect all *.miniskin.xml files
	type xmlEntry struct {
		relDir string // relative directory from targetDir ("." for root)
		path   string // absolute path to XML file
		parsed *xmlMiniskin
	}
	var entries []xmlEntry

	err = filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".miniskin.xml") {
			parsed, parseErr := parseMiniskinXML(path)
			if parseErr != nil {
				return parseErr
			}
			relDir, _ := filepath.Rel(targetDir, filepath.Dir(path))
			relDir = filepath.ToSlash(relDir)
			entries = append(entries, xmlEntry{relDir: relDir, path: path, parsed: parsed})
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return fmt.Errorf("no *.miniskin.xml files found in %s", targetDir)
	}

	// Sort by depth (shallowest first)
	sort.Slice(entries, func(i, j int) bool {
		di := strings.Count(entries[i].relDir, "/")
		dj := strings.Count(entries[j].relDir, "/")
		if di != dj {
			return di < dj
		}
		return entries[i].relDir < entries[j].relDir
	})

	// Build directory tree
	root := &dirNode{name: "."}
	nodeMap := map[string]*dirNode{".": root}

	for _, e := range entries {
		node := ensureNode(nodeMap, root, e.relDir)
		if node.parsed == nil {
			node.parsed = e.parsed
			node.xmlPath = e.path
		} else {
			// Merge resource-lists from multiple XMLs in the same directory
			node.parsed.ResourceLists = append(node.parsed.ResourceLists, e.parsed.ResourceLists...)
			if e.parsed.MockupList != nil {
				if node.parsed.MockupList == nil {
					node.parsed.MockupList = e.parsed.MockupList
				} else {
					node.parsed.MockupList.Items = append(node.parsed.MockupList.Items, e.parsed.MockupList.Items...)
				}
			}
		}
	}

	// Build combined XML
	combined := &xmlMiniskin{}

	// Take root-level properties from root XML if it exists
	if root.parsed != nil {
		combined.SkinDir = root.parsed.SkinDir
		combined.Log = root.parsed.Log
		combined.MuxInclude = root.parsed.MuxInclude
		combined.MuxExclude = root.parsed.MuxExclude
		combined.Globals = root.parsed.Globals
		combined.Escapes = root.parsed.Escapes
		combined.BucketList = root.parsed.BucketList
	}

	// Check for duplicate items across all XMLs
	if err := checkDuplicateItems(root, ""); err != nil {
		return err
	}

	// Build nested resource-lists from tree
	combined.ResourceLists = buildResourceLists(root, true)

	// Collect mockup-lists with adjusted paths
	mockups := collectMockups(root, "")
	if len(mockups) == 1 {
		combined.MockupList = &mockups[0]
	} else if len(mockups) > 1 {
		// Merge all mockup-lists into one
		merged := xmlMockupList{}
		for _, ml := range mockups {
			merged.Items = append(merged.Items, ml.Items...)
			merged.Vars = append(merged.Vars, ml.Vars...)
			if merged.SaveMode == "" {
				merged.SaveMode = ml.SaveMode
			}
			if merged.SkinDir == "" {
				merged.SkinDir = ml.SkinDir
			}
		}
		combined.MockupList = &merged
	}

	// Marshal to XML
	output, err := xml.MarshalIndent(combined, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling combined XML: %w", err)
	}

	// Determine output filename
	outputPath := filepath.Join(targetDir, filepath.Base(targetDir)+".miniskin.xml")
	// If root had an XML, use that name
	if root.xmlPath != "" {
		outputPath = root.xmlPath
	}

	xmlContent := xml.Header + string(output) + "\n"
	if err := os.WriteFile(outputPath, []byte(xmlContent), 0644); err != nil {
		return err
	}

	// Remove old XML files (except the output file)
	for _, e := range entries {
		if e.path != outputPath {
			os.Remove(e.path)
		}
	}

	return nil
}

// SplitXML reads a combined XML file and splits nested resource-lists
// into separate *.miniskin.xml files per subdirectory.
func SplitXML(xmlPath string) error {
	xmlPath, err := filepath.Abs(xmlPath)
	if err != nil {
		return err
	}

	parsed, err := parseMiniskinXML(xmlPath)
	if err != nil {
		return err
	}

	baseDir := filepath.Dir(xmlPath)

	// Extract nested resource-lists with src into separate files
	var topLevel []xmlResourceList
	for _, rl := range parsed.ResourceLists {
		splitResourceList(&rl, baseDir, &topLevel)
	}
	parsed.ResourceLists = topLevel

	// Split mockup-list items by directory
	if parsed.MockupList != nil {
		splitMockupList(parsed.MockupList, baseDir)
	}

	// Rewrite original XML without nested content
	output, err := xml.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling split XML: %w", err)
	}

	xmlContent := xml.Header + string(output) + "\n"
	return os.WriteFile(xmlPath, []byte(xmlContent), 0644)
}

// --- helpers ---

// checkDuplicateItems verifies no two items resolve to the same file path.
func checkDuplicateItems(node *dirNode, prefix string) error {
	seen := map[string]string{} // resolved path → source XML
	return checkDuplicatesRecursive(node, prefix, seen)
}

func checkDuplicatesRecursive(node *dirNode, prefix string, seen map[string]string) error {
	if node.parsed != nil {
		for _, rl := range node.parsed.ResourceLists {
			if err := checkRLDuplicates(&rl, prefix, seen, node.xmlPath); err != nil {
				return err
			}
		}
	}
	for _, child := range node.children {
		childPrefix := child.name
		if prefix != "" {
			childPrefix = prefix + "/" + child.name
		}
		if err := checkDuplicatesRecursive(child, childPrefix, seen); err != nil {
			return err
		}
	}
	return nil
}

func checkRLDuplicates(rl *xmlResourceList, prefix string, seen map[string]string, source string) error {
	dir := prefix
	if rl.Src != "" {
		if dir != "" {
			dir = dir + "/" + rl.Src
		} else {
			dir = rl.Src
		}
	}
	for _, item := range rl.Items {
		resolved := item.File
		if dir != "" {
			resolved = dir + "/" + item.File
		}
		if prev, exists := seen[resolved]; exists {
			return fmt.Errorf("duplicate item %q: found in %s and %s", resolved, prev, source)
		}
		seen[resolved] = source
	}
	for i := range rl.ResourceLists {
		if err := checkRLDuplicates(&rl.ResourceLists[i], dir, seen, source); err != nil {
			return err
		}
	}
	return nil
}

// ensureNode creates or returns the node for a given relative path.
func ensureNode(nodeMap map[string]*dirNode, root *dirNode, relDir string) *dirNode {
	if relDir == "." {
		return root
	}
	if n, ok := nodeMap[relDir]; ok {
		return n
	}
	parts := strings.Split(relDir, "/")
	parent := root
	for i, part := range parts {
		key := strings.Join(parts[:i+1], "/")
		if n, ok := nodeMap[key]; ok {
			parent = n
			continue
		}
		node := &dirNode{name: part}
		parent.children = append(parent.children, node)
		nodeMap[key] = node
		parent = node
	}
	return parent
}

// buildResourceLists converts the directory tree to nested resource-lists.
func buildResourceLists(node *dirNode, isRoot bool) []xmlResourceList {
	var childRLs []xmlResourceList
	for _, child := range node.children {
		childRLs = append(childRLs, buildResourceLists(child, false)...)
	}

	if isRoot {
		var result []xmlResourceList
		if node.parsed != nil {
			result = append(result, node.parsed.ResourceLists...)
		}
		result = append(result, childRLs...)
		return result
	}

	// Non-root: wrap under src=name
	var ownRLs []xmlResourceList
	if node.parsed != nil {
		ownRLs = node.parsed.ResourceLists
	}

	if len(ownRLs) <= 1 {
		// Single (or zero) own resource-list: merge into one node with src
		var merged xmlResourceList
		if len(ownRLs) == 1 {
			merged = ownRLs[0]
		}
		merged.Src = node.name
		merged.ResourceLists = append(merged.ResourceLists, childRLs...)
		return []xmlResourceList{merged}
	}

	// Multiple own resource-lists: need a wrapper
	wrapper := xmlResourceList{Src: node.name}
	wrapper.ResourceLists = append(ownRLs, childRLs...)
	return []xmlResourceList{wrapper}
}

// collectMockups gathers all mockup-lists with path-adjusted items.
func collectMockups(node *dirNode, prefix string) []xmlMockupList {
	var result []xmlMockupList
	if node.parsed != nil && node.parsed.MockupList != nil {
		ml := *node.parsed.MockupList
		// Deep copy items to avoid modifying originals
		items := make([]xmlMockupItem, len(ml.Items))
		copy(items, ml.Items)
		for i := range items {
			if prefix != "" {
				items[i].Src = prefix + "/" + items[i].Src
				if items[i].Negative != "" {
					items[i].Negative = prefix + "/" + items[i].Negative
				}
			}
		}
		ml.Items = items
		result = append(result, ml)
	}
	for _, child := range node.children {
		childPrefix := child.name
		if prefix != "" {
			childPrefix = prefix + "/" + child.name
		}
		result = append(result, collectMockups(child, childPrefix)...)
	}
	return result
}

// splitResourceList recursively extracts nested resource-lists with src
// into separate XML files.
func splitResourceList(rl *xmlResourceList, baseDir string, topLevel *[]xmlResourceList) {
	if rl.Src != "" {
		// This resource-list belongs in a subdirectory
		dir := filepath.Join(baseDir, filepath.FromSlash(rl.Src))
		os.MkdirAll(dir, 0755)

		// Build XML for this subdirectory
		subXML := &xmlMiniskin{}

		// Own items become a resource-list without src
		if len(rl.Items) > 0 || rl.URLBase != "" || rl.SkinDir != "" || len(rl.Escapes) > 0 {
			ownRL := xmlResourceList{
				URLBase:    rl.URLBase,
				SkinDir:    rl.SkinDir,
				MuxInclude: rl.MuxInclude,
				MuxExclude: rl.MuxExclude,
				Escapes:    rl.Escapes,
				Items:      rl.Items,
			}
			subXML.ResourceLists = append(subXML.ResourceLists, ownRL)
		}

		// Recurse children - they might need further splitting relative to this dir
		for i := range rl.ResourceLists {
			child := &rl.ResourceLists[i]
			if child.Src != "" {
				splitResourceList(child, dir, &subXML.ResourceLists)
			} else {
				subXML.ResourceLists = append(subXML.ResourceLists, *child)
			}
		}

		// Write subdirectory XML
		xmlName := filepath.Base(dir) + ".miniskin.xml"
		xmlPath := filepath.Join(dir, xmlName)

		output, _ := xml.MarshalIndent(subXML, "", "  ")
		xmlContent := xml.Header + string(output) + "\n"
		os.WriteFile(xmlPath, []byte(xmlContent), 0644)
	} else {
		// No src - stays at top level
		clean := *rl
		clean.ResourceLists = nil
		// Recurse children
		for i := range rl.ResourceLists {
			splitResourceList(&rl.ResourceLists[i], baseDir, &clean.ResourceLists)
		}
		*topLevel = append(*topLevel, clean)
	}
}

// splitMockupList splits mockup items by directory prefix into separate files.
func splitMockupList(ml *xmlMockupList, baseDir string) {
	// Group items by directory prefix
	groups := map[string][]xmlMockupItem{}
	var rootItems []xmlMockupItem

	for _, item := range ml.Items {
		dir := filepath.Dir(filepath.ToSlash(item.Src))
		if dir == "." {
			rootItems = append(rootItems, item)
		} else {
			// Strip directory prefix from item paths
			adjusted := item
			adjusted.Src = filepath.Base(item.Src)
			if adjusted.Negative != "" {
				adjusted.Negative = filepath.Base(item.Negative)
			}
			groups[dir] = append(groups[dir], adjusted)
		}
	}

	// Write mockup-lists to subdirectory XMLs
	for dir, items := range groups {
		subDir := filepath.Join(baseDir, filepath.FromSlash(dir))
		xmlName := filepath.Base(subDir) + ".miniskin.xml"
		xmlPath := filepath.Join(subDir, xmlName)

		// Read existing XML or create new
		var subXML *xmlMiniskin
		if existing, err := parseMiniskinXML(xmlPath); err == nil {
			subXML = existing
		} else {
			subXML = &xmlMiniskin{}
		}

		subML := xmlMockupList{
			SaveMode: ml.SaveMode,
			SkinDir:  ml.SkinDir,
			Items:    items,
		}
		subXML.MockupList = &subML

		output, _ := xml.MarshalIndent(subXML, "", "  ")
		os.MkdirAll(subDir, 0755)
		xmlContent := xml.Header + string(output) + "\n"
		os.WriteFile(xmlPath, []byte(xmlContent), 0644)
	}

	// Keep only root items in original
	ml.Items = rootItems
	if len(rootItems) == 0 {
		ml.Items = nil
	}
}
