package miniskin

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strings"
)

// escapeTypes maps escape prefix names to their escape functions.
var escapeTypes = map[string]func(string) string{
	"html": escapeHTML,
	"xml":  escapeXML,
	"url":  escapeURLEncode,
	"js":   escapeJS,
	"css":  escapeCSS,
	"json": escapeJSON,
	"sql":  escapeSQL,
	"sqlt": escapeSQLT,
}

// parseEscapeTag checks if a tag starts with an escape prefix (e.g. "url:varname").
// Returns the escape function and variable name if matched.
func parseEscapeTag(tagStr string) (escapeFn func(string) string, varName string, ok bool) {
	idx := strings.IndexByte(tagStr, ':')
	if idx < 0 {
		return nil, "", false
	}
	prefix := tagStr[:idx]
	fn, exists := escapeTypes[prefix]
	if !exists {
		return nil, "", false
	}
	varName = strings.TrimSpace(tagStr[idx+1:])
	return fn, varName, true
}

func escapeHTML(s string) string {
	return html.EscapeString(s)
}

func escapeXML(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func escapeURLEncode(s string) string {
	return url.QueryEscape(s)
}

func escapeJS(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '<':
			b.WriteString(`\x3c`)
		case '>':
			b.WriteString(`\x3e`)
		case '&':
			b.WriteString(`\x26`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func escapeCSS(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '"', '\'', '(', ')', ';', '{', '}', '<', '>':
			fmt.Fprintf(&b, "\\%x ", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func escapeJSON(s string) string {
	data, _ := json.Marshal(s)
	// Strip surrounding quotes
	return string(data[1 : len(data)-1])
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func escapeSQLT(s string) string {
	s = escapeSQL(s)
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// cascadeEscapeRules appends child rules after parent rules.
// Later rules override earlier ones when matched.
func cascadeEscapeRules(parent, child []xmlEscape) []xmlEscape {
	if len(child) == 0 {
		return parent
	}
	if len(parent) == 0 {
		return child
	}
	result := make([]xmlEscape, 0, len(parent)+len(child))
	result = append(result, parent...)
	result = append(result, child...)
	return result
}

// resolveDefaultEscape finds the escape function for a filename based on cascaded rules.
// If the item has an explicit escape attribute, it takes priority.
// Otherwise, rules are matched last-to-first (child overrides parent).
// Returns nil (no escaping) if no rule matches.
func resolveDefaultEscape(filename string, itemEscape string, rules []xmlEscape) func(string) string {
	// Item-level escape attribute takes priority
	if itemEscape != "" {
		if fn, ok := escapeTypes[itemEscape]; ok {
			return fn
		}
	}
	// Match rules last-to-first (later rules override)
	for i := len(rules) - 1; i >= 0; i-- {
		if matchesMuxPattern(filename, rules[i].Ext) {
			if fn, ok := escapeTypes[rules[i].As]; ok {
				return fn
			}
		}
	}
	return nil
}
