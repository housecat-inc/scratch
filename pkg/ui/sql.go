package ui

import (
	"fmt"
	"path/filepath"
	"strings"
)

func dbLabel(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

func tableSelect(name string) string {
	return fmt.Sprintf("SELECT * FROM %q LIMIT 100", strings.ReplaceAll(name, `"`, `""`))
}
