package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
)

type RanksIn struct {
	Classes  []string
	CSSEntry string
}

func Ranks(in RanksIn) (map[string]int, error) {
	if len(in.Classes) == 0 {
		return map[string]int{}, nil
	}

	entry, err := os.ReadFile(in.CSSEntry)
	if err != nil {
		return nil, errors.Wrap(err, "read CSS entry")
	}

	var source strings.Builder
	for line := range strings.Lines(string(entry)) {
		if strings.HasPrefix(strings.TrimSpace(line), "@source ") {
			continue
		}
		source.WriteString(line)
	}
	fmt.Fprintf(&source, "\n@source inline(%q);\n", strings.Join(in.Classes, " "))
	css := strings.Replace(source.String(), `@import "tailwindcss";`, `@import "tailwindcss" source(none);`, 1)

	cmd := exec.Command("tailwindcss", "-i", "-")
	cmd.Dir = filepath.Dir(in.CSSEntry)
	cmd.Stdin = strings.NewReader(css)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "run tailwindcss: %s", stderr.String())
	}

	type rankedClass struct {
		class string
		index int
	}
	selectors := outerSelectorRanges(out)
	var ranked []rankedClass
	for _, class := range in.Classes {
		if index := selectorIndex(out, selectors, class); index >= 0 {
			ranked = append(ranked, rankedClass{class: class, index: index})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].index < ranked[j].index })

	ranks := make(map[string]int, len(ranked))
	for rank, entry := range ranked {
		ranks[entry.class] = rank
	}
	return ranks, nil
}

type byteRange struct {
	end   int
	start int
}

func selectorIndex(css []byte, selectors []byteRange, class string) int {
	selector := []byte("." + escapeCSSIdentifier(class))
	for _, candidate := range selectors {
		header := css[candidate.start:candidate.end]
		offset := 0
		for {
			index := bytes.Index(header[offset:], selector)
			if index < 0 {
				break
			}
			index += offset
			end := index + len(selector)
			if end == len(header) || !continuesCSSIdentifier(header[end]) {
				return candidate.start + index
			}
			offset = end
		}
	}
	return -1
}

func outerSelectorRanges(css []byte) []byteRange {
	var out []byteRange
	var rules []bool
	start := 0
	ruleDepth := 0
	for i := 0; i < len(css); i++ {
		switch css[i] {
		case '\\':
			i++
		case '/':
			commentStart := i
			before, _ := trimSpaceRange(css, start, commentStart)
			comment := i+1 < len(css) && css[i+1] == '*'
			i = skipCSSValue(css, i)
			if comment && before == commentStart {
				start = i + 1
			}
		case '\'', '"':
			i = skipCSSValue(css, i)
		case ';':
			start = i + 1
		case '{':
			headerStart, headerEnd := trimSpaceRange(css, start, i)
			isRule := headerStart < headerEnd && css[headerStart] != '@'
			if isRule && ruleDepth == 0 {
				out = append(out, byteRange{end: headerEnd, start: headerStart})
			}
			rules = append(rules, isRule)
			if isRule {
				ruleDepth++
			}
			start = i + 1
		case '}':
			if len(rules) > 0 {
				if rules[len(rules)-1] {
					ruleDepth--
				}
				rules = rules[:len(rules)-1]
			}
			start = i + 1
		}
	}
	return out
}

func skipCSSValue(css []byte, start int) int {
	if css[start] == '/' && start+1 < len(css) && css[start+1] == '*' {
		if end := bytes.Index(css[start+2:], []byte("*/")); end >= 0 {
			return start + 2 + end + 1
		}
		return len(css) - 1
	}
	if css[start] != '\'' && css[start] != '"' {
		return start
	}
	quote := css[start]
	for i := start + 1; i < len(css); i++ {
		if css[i] == '\\' {
			i++
			continue
		}
		if css[i] == quote {
			return i
		}
	}
	return len(css) - 1
}

func trimSpaceRange(value []byte, start, end int) (int, int) {
	for start < end && (value[start] == ' ' || value[start] == '\n' || value[start] == '\r' || value[start] == '\t') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\n' || value[end-1] == '\r' || value[end-1] == '\t') {
		end--
	}
	return start, end
}

func escapeCSSIdentifier(value string) string {
	var out strings.Builder
	for i, r := range value {
		switch {
		case r == 0:
			out.WriteRune('\uFFFD')
		case r <= 0x1f || r == 0x7f || i == 0 && r >= '0' && r <= '9' || i == 1 && value[0] == '-' && r >= '0' && r <= '9':
			fmt.Fprintf(&out, "\\%x ", r)
		case i == 0 && r == '-' && len(value) == 1:
			out.WriteString(`\-`)
		case r >= 0x80 || r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
			out.WriteRune(r)
		default:
			out.WriteByte('\\')
			out.WriteRune(r)
		}
	}
	return out.String()
}

func continuesCSSIdentifier(b byte) bool {
	return b == '-' || b == '_' || b == '\\' || b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= 0x80
}
