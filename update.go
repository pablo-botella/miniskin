package miniskin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// tagInfo records a percent-tag's position and content within a string.
type tagInfo struct {
	start   int    // byte offset of the opening '<'
	end     int    // byte offset after the closing '>'
	content string // tag body between delimiters
}

// UpdateImports walks all mockup source files and refreshes the inline content
// of mockup-import blocks. Single import tags are promoted to block tags
// (import + content + end). Existing blocks get their content replaced.
func (ms *Miniskin) UpdateImports() error {
	_, bl, err := ms.init()
	if err != nil {
		return err
	}

	ms.logf("=== UpdateImports ===")
	for _, bucket := range bl.Buckets {
		bucketSrc := filepath.Join(ms.contentPath, filepath.FromSlash(bucket.Src))
		if err := ms.walkBucket(bucket, func(parsed *xmlMiniskin, dir string, _ string) error {
			if parsed.MockupList == nil {
				return nil
			}
			for _, mi := range parsed.MockupList.Items {
				srcPath := absPath(filepath.Join(dir, mi.Src))
				data, err := os.ReadFile(srcPath)
				if err != nil {
					return fmt.Errorf("reading %s: %w", srcPath, err)
				}

				_, imports := scanExportsImports(string(data))
				if len(imports) == 0 {
					continue
				}

				updated, err := refreshImports(string(data), bucketSrc, filepath.Dir(srcPath))
				if err != nil {
					return fmt.Errorf("updating %s: %w", srcPath, err)
				}

				if updated != string(data) {
					if err := os.WriteFile(srcPath, []byte(updated), 0644); err != nil {
						return fmt.Errorf("writing %s: %w", srcPath, err)
					}
					ms.logf("  updated: %s", mi.Src)
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// CleanImports walks all mockup source files and removes the inline content
// of mockup-import blocks, leaving the import and end tags with nothing between them.
func (ms *Miniskin) CleanImports() error {
	_, bl, err := ms.init()
	if err != nil {
		return err
	}

	ms.logf("=== CleanImports ===")
	for _, bucket := range bl.Buckets {
		if err := ms.walkBucket(bucket, func(parsed *xmlMiniskin, dir string, _ string) error {
			if parsed.MockupList == nil {
				return nil
			}
			for _, mi := range parsed.MockupList.Items {
				srcPath := absPath(filepath.Join(dir, mi.Src))
				data, err := os.ReadFile(srcPath)
				if err != nil {
					return fmt.Errorf("reading %s: %w", srcPath, err)
				}

				_, imports := scanExportsImports(string(data))
				if len(imports) == 0 {
					continue
				}

				cleaned := cleanImports(string(data))

				if cleaned != string(data) {
					if err := os.WriteFile(srcPath, []byte(cleaned), 0644); err != nil {
						return fmt.Errorf("writing %s: %w", srcPath, err)
					}
					ms.logf("  cleaned: %s", mi.Src)
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// cleanImports removes the inline content of mockup-import blocks,
// leaving import and end tags directly adjacent (with just a newline).
// It operates on full lines so that CSS wrappers like /* */ are preserved.
func cleanImports(content string) string {
	tags := findTags(content)
	if len(tags) == 0 {
		return content
	}

	var out strings.Builder
	pos := 0

	for i := 0; i < len(tags); {
		trimmed := strings.TrimSpace(tags[i].content)
		_, ok := isMockupImport(trimmed)
		if !ok {
			i++
			continue
		}

		// Check if next tag is "end" (existing block)
		hasBlock := false
		if i+1 < len(tags) {
			nextTrimmed := strings.TrimSpace(tags[i+1].content)
			if nextTrimmed == "end" || nextTrimmed == "end-mockup-export" || nextTrimmed == "end-mockup-import" {
				hasBlock = true
			}
		}

		if hasBlock {
			// Emit up to end of line containing the import tag
			endOfLine := tags[i].end
			for endOfLine < len(content) && content[endOfLine] != '\n' {
				endOfLine++
			}
			if endOfLine < len(content) {
				endOfLine++ // include the \n
			}
			out.WriteString(content[pos:endOfLine])

			// Skip to start of line containing the end tag
			startOfLine := tags[i+1].start
			for startOfLine > 0 && content[startOfLine-1] != '\n' {
				startOfLine--
			}
			pos = startOfLine
			i += 2
		} else {
			// Single tag — nothing to clean
			i++
		}
	}

	if pos < len(content) {
		out.WriteString(content[pos:])
	}

	return out.String()
}

// refreshImports replaces the inline content of mockup-import blocks
// with the current content of the referenced files.
//
// Single tags:  <!--%%mockup-import:/path%%-->
//            →  <!--%%mockup-import:/path%%-->\ncontent\n<!--%%end-mockup-import%%-->
//
// Block tags:   <!--%%mockup-import:/path%%-->old<!--%%end-mockup-import%%-->
//            →  <!--%%mockup-import:/path%%-->\nnew\n<!--%%end-mockup-import%%-->
func refreshImports(content, contentPath, fileDir string) (string, error) {
	tags := findTags(content)
	if len(tags) == 0 {
		return content, nil
	}

	var out strings.Builder
	pos := 0

	for i := 0; i < len(tags); {
		trimmed := strings.TrimSpace(tags[i].content)
		imf, ok := isMockupImport(trimmed)
		if !ok {
			i++
			continue
		}

		// Read the referenced file
		filePath := absPath(importFilePath(imf.filename, contentPath, fileDir))
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("refreshing mockup-import %s: %w", filePath, err)
		}

		// Check if next tag is "end" (existing block)
		hasBlock := false
		if i+1 < len(tags) {
			nextTrimmed := strings.TrimSpace(tags[i+1].content)
			if nextTrimmed == "end" || nextTrimmed == "end-mockup-export" || nextTrimmed == "end-mockup-import" {
				hasBlock = true
			}
		}

		// Emit up to end of line containing the import tag
		endOfLine := tags[i].end
		for endOfLine < len(content) && content[endOfLine] != '\n' {
			endOfLine++
		}
		if endOfLine < len(content) {
			endOfLine++ // include the \n
		}
		out.WriteString(content[pos:endOfLine])
		indentStr := resolveIndent(imf.indent)
		if indentStr != "" {
			data = applyIndent(data, indentStr)
		}
		out.Write(data)
		out.WriteString("\n")

		if hasBlock {
			// Skip to start of line containing the end tag, emit from there
			startOfLine := tags[i+1].start
			for startOfLine > 0 && content[startOfLine-1] != '\n' {
				startOfLine--
			}
			endOfEndLine := tags[i+1].end
			for endOfEndLine < len(content) && content[endOfEndLine] != '\n' {
				endOfEndLine++
			}
			if endOfEndLine < len(content) {
				endOfEndLine++
			}
			out.WriteString(content[startOfLine:endOfEndLine])
			pos = endOfEndLine
			i += 2
		} else {
			// Single tag → promote to block
			out.WriteString("<!--%%end-mockup-import%%-->\n")
			pos = endOfLine
			i++
		}
	}

	if pos < len(content) {
		out.WriteString(content[pos:])
	}

	return out.String(), nil
}

// findTags extracts all percent-tags with their byte positions.
func findTags(content string) []tagInfo {
	var tags []tagInfo
	var tag strings.Builder
	state := stText
	tagStart := 0

	for i := 0; i < len(content); i++ {
		c := content[i]
		switch state {
		case stText:
			// /*<% — JS-comment wrapper opener: tagStart includes the /*
			if c == '/' && i+3 < len(content) && content[i+1] == '*' && content[i+2] == '<' && content[i+3] == '%' {
				tagStart = i
				i += 2
				state = stLT
				continue
			}
			if c == '<' {
				tagStart = i
				state = stLT
			}
		case stLT:
			switch c {
			case '%':
				state = stLTPct
			case '!':
				state = stCmtBang
			default:
				state = stText
			}
		case stLTPct:
			switch c {
			case '%':
				state = stLTPctPct
			default:
				tag.Reset()
				tag.WriteByte(c)
				state = stSingle
			}
		case stSingle:
			switch c {
			case '%':
				state = stSingleClose
			default:
				tag.WriteByte(c)
			}
		case stSingleClose:
			switch c {
			case '>':
				// %>*/ — JS-comment wrapper closer: end includes the */
				if i+2 < len(content) && content[i+1] == '*' && content[i+2] == '/' {
					tags = append(tags, tagInfo{start: tagStart, end: i + 3, content: tag.String()})
					i += 2
				} else {
					tags = append(tags, tagInfo{start: tagStart, end: i + 1, content: tag.String()})
				}
				state = stText
			case '-':
				state = stSCmtD1
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stSingle
			}
		case stLTPctPct:
			tag.Reset()
			tag.WriteByte(c)
			state = stDouble
		case stDouble:
			switch c {
			case '%':
				state = stDoubleClose1
			default:
				tag.WriteByte(c)
			}
		case stDoubleClose1:
			switch c {
			case '%':
				state = stDoubleClose2
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stDouble
			}
		case stDoubleClose2:
			switch c {
			case '>':
				// %%>*/ — JS-comment wrapper closer: end includes the */
				if i+2 < len(content) && content[i+1] == '*' && content[i+2] == '/' {
					tags = append(tags, tagInfo{start: tagStart, end: i + 3, content: tag.String()})
					i += 2
				} else {
					tags = append(tags, tagInfo{start: tagStart, end: i + 1, content: tag.String()})
				}
				state = stText
			case '-':
				state = stDCmtD1
			default:
				tag.WriteByte('%')
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stDouble
			}
		case stCmtBang:
			switch c {
			case '-':
				state = stCmtDash1
			default:
				state = stText
			}
		case stCmtDash1:
			switch c {
			case '-':
				state = stCmtDash2
			default:
				state = stText
			}
		case stCmtDash2:
			switch c {
			case '%':
				state = stCmtPct1
			default:
				state = stText
			}
		case stCmtPct1:
			switch c {
			case '%':
				tag.Reset()
				state = stCmtTag
			default:
				tag.Reset()
				tag.WriteByte(c)
				state = stCmtSingle
			}
		case stCmtSingle:
			switch c {
			case '%':
				state = stCmtSClose1
			default:
				tag.WriteByte(c)
			}
		case stCmtSClose1:
			switch c {
			case '>':
				tags = append(tags, tagInfo{start: tagStart, end: i + 1, content: tag.String()})
				state = stText
			case '-':
				state = stCmtSClose2
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stCmtSingle
			}
		case stCmtSClose2:
			switch c {
			case '-':
				state = stCmtSClose3
			default:
				tag.WriteString("%-")
				tag.WriteByte(c)
				state = stCmtSingle
			}
		case stCmtSClose3:
			switch c {
			case '>':
				tags = append(tags, tagInfo{start: tagStart, end: i + 1, content: tag.String()})
				state = stText
			default:
				tag.WriteString("%--")
				tag.WriteByte(c)
				state = stCmtSingle
			}
		case stCmtTag:
			switch c {
			case '%':
				state = stCmtClose1
			default:
				tag.WriteByte(c)
			}
		case stCmtClose1:
			switch c {
			case '%':
				state = stCmtClose2
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stCmtTag
			}
		case stCmtClose2:
			switch c {
			case '>':
				tags = append(tags, tagInfo{start: tagStart, end: i + 1, content: tag.String()})
				state = stText
			case '-':
				state = stCmtClose3
			default:
				tag.WriteByte('%')
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stCmtTag
			}
		case stCmtClose3:
			switch c {
			case '-':
				state = stCmtClose4
			default:
				tag.WriteString("%%-")
				tag.WriteByte(c)
				state = stCmtTag
			}
		case stCmtClose4:
			switch c {
			case '>':
				tags = append(tags, tagInfo{start: tagStart, end: i + 1, content: tag.String()})
				state = stText
			default:
				tag.WriteString("%%--")
				tag.WriteByte(c)
				state = stCmtTag
			}

		// <%...%--> (single open, comment close)
		case stSCmtD1:
			switch c {
			case '-':
				state = stSCmtD2
			default:
				tag.WriteString("%-")
				tag.WriteByte(c)
				state = stSingle
			}
		case stSCmtD2:
			switch c {
			case '>':
				tags = append(tags, tagInfo{start: tagStart, end: i + 1, content: tag.String()})
				state = stText
			default:
				tag.WriteString("%--")
				tag.WriteByte(c)
				state = stSingle
			}

		// <%%...%%--> (double open, comment close)
		case stDCmtD1:
			switch c {
			case '-':
				state = stDCmtD2
			default:
				tag.WriteString("%%-")
				tag.WriteByte(c)
				state = stDouble
			}
		case stDCmtD2:
			switch c {
			case '>':
				tags = append(tags, tagInfo{start: tagStart, end: i + 1, content: tag.String()})
				state = stText
			default:
				tag.WriteString("%%--")
				tag.WriteByte(c)
				state = stDouble
			}
		}
	}
	return tags
}
