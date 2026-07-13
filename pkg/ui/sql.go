package ui

import (
	"fmt"
	"strings"
)

func tableSelect(name string) string {
	return fmt.Sprintf("SELECT * FROM %q LIMIT 100", strings.ReplaceAll(name, `"`, `""`))
}
