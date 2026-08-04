package miniskin

import (
	"bufio"
	"fmt"
	"strings"
)

// docBlockOp parses a "doc-block-OP:NAME" tag (op is "begin", "end", "toc", "content").
// Returns the buffer name and whether the tag matched.
func docBlockOp(tagStr, op string) (name string, ok bool) {
	prefix := "doc-block-" + op + ":"
	if !strings.HasPrefix(tagStr, prefix) {
		return "", false
	}
	name = strings.TrimSpace(strings.TrimPrefix(tagStr, prefix))
	if name == "" {
		return "", false
	}
	return name, true
}

// generateDocBlockTOC scans markdown content for H1/H2 headers and emits a
// nested unordered list with GitHub-compatible anchor links. Skips fenced
// code blocks. Header anchors are deduplicated with -1, -2, … suffixes.
func generateDocBlockTOC(content string) string {
	var b strings.Builder
	counts := make(map[string]int)
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	inFence := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		switch {
		case strings.HasPrefix(line, "## "):
			title := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			fmt.Fprintf(&b, "  - [%s](#%s)\n", title, uniqueAnchor(title, counts))
		case strings.HasPrefix(line, "# "):
			title := strings.TrimSpace(strings.TrimPrefix(line, "# "))
			fmt.Fprintf(&b, "- [%s](#%s)\n", title, uniqueAnchor(title, counts))
		}
	}
	return b.String()
}

// uniqueAnchor returns the slug for title, appending -N when the same slug has
// already been seen, mirroring GitHub's duplicate-header behavior.
func uniqueAnchor(title string, counts map[string]int) string {
	base := slugify(title)
	n := counts[base]
	counts[base] = n + 1
	if n == 0 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, n)
}

// slugify converts a header title to a GitHub-style anchor: lowercase,
// alphanumerics kept, spaces and hyphens collapsed to a single hyphen,
// everything else dropped.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastDash := true
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
