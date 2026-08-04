package miniskin

import (
	"fmt"
	"os"
	"path/filepath"
)

// applySkin loads the skin file, sets the "content" variable to the processed body,
// merges front-matter vars, and resolves all percent tags in the skin.
func (ms *Miniskin) applySkin(skinName string, body string, fmVars map[string]string, skinDir string) (string, error) {
	base := ms.bucketSrc
	if base == "" {
		base = ms.contentPath
	}
	skinPath := absPath(filepath.Join(base, skinDir, skinName+".html"))

	data, err := os.ReadFile(skinPath)
	if err != nil {
		return "", fmt.Errorf("loading skin %q: %w", skinName, err)
	}

	vars := make(map[string]string)
	// Global vars first
	for k, v := range ms.globals {
		vars[k] = v
	}
	// Front-matter vars override globals
	for k, v := range fmVars {
		vars[k] = v
	}
	// content is the processed body
	vars["content"] = body

	chain := []string{skinPath}
	result, err := ms.resolvePercent(string(data), vars, chain)
	if err != nil {
		return "", fmt.Errorf("processing skin %q: %w", skinName, err)
	}

	return result, nil
}
