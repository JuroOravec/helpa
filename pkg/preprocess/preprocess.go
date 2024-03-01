package preprocess

import (
	"regexp"
	"strings"

	eris "github.com/rotisserie/eris"
)

// Remove leading/trailing empty lines
func TrimTemplate(tmpl string) (string, error) {
	for _, pattern := range []string{`^(?:\s*\n)+`, `(?:\n\s*)+$`} {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return "", eris.Wrap(err, "failed to compile regexp")
		}
		tmpl = re.ReplaceAllLiteralString(tmpl, "")
	}
	return tmpl, nil
}

// Unindent takes a string and un-indents all lines by the smallest number
// of leading spaces across all lines.
func Unindent(input string) string {
	lines := strings.Split(input, "\n")

	// Find the smallest number of leading spaces across all lines.
	smallestIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue // Skip empty or whitespace-only lines
		}
		currentIndent := len(line) - len(strings.TrimLeft(line, " "))
		if smallestIndent == -1 || currentIndent < smallestIndent {
			smallestIndent = currentIndent
		}
	}

	// If there are no indents (or only empty lines), return the input as is.
	if smallestIndent == -1 {
		return input
	}

	// Remove the smallest number of leading spaces from each line.
	for i, line := range lines {
		if len(line) >= smallestIndent {
			lines[i] = line[smallestIndent:]
		}
	}

	// Join the lines back together.
	return strings.Join(lines, "\n")
}
