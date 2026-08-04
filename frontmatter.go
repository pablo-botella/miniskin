package miniskin

import (
	"fmt"
	"strings"
)

// parseFrontMatter splits a file into front-matter key-value pairs, directives, and body content.
// Front-matter is delimited by --- lines. Keys and values are separated by :.
// Lines starting with @ are directives (e.g. @minify, @eol, @ltrim).
// Returns nil vars and the full content if no front-matter is present.
func parseFrontMatter(content string) (vars map[string]string, directives map[string]string, body string, err error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")

	if !strings.HasPrefix(content, "---\n") {
		return nil, nil, content, nil
	}

	end := strings.Index(content[4:], "\n---\n")
	if end == -1 {
		// check for --- at the very end (no trailing newline)
		end = strings.Index(content[4:], "\n---")
		if end == -1 || end+4+4 != len(content) {
			return nil, nil, "", fmt.Errorf("unclosed front-matter: missing closing ---")
		}
	}

	header := content[4 : 4+end]
	body = content[4+end+4:]
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}

	vars = make(map[string]string)
	directives = make(map[string]string)
	for lineNum, line := range strings.Split(header, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "@") {
			d := line[1:]
			if idx := strings.IndexByte(d, ':'); idx >= 0 {
				directives[strings.TrimSpace(d[:idx])] = strings.TrimSpace(d[idx+1:])
			} else {
				directives[d] = ""
			}
			continue
		}
		colon := strings.Index(line, ":")
		if colon == -1 {
			return nil, nil, "", fmt.Errorf("front-matter line %d: missing colon in %q", lineNum+1, line)
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		if key == "" {
			return nil, nil, "", fmt.Errorf("front-matter line %d: empty key", lineNum+1)
		}
		vars[key] = val
	}

	return vars, directives, body, nil
}
