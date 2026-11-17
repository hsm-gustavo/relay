package curlimport

import (
	"fmt"
	"strings"
	"unicode"
)

// words parses POSIX-style quoting; it never invokes a shell or expands values.
func words(command string) ([]string, error) {
	var result []string
	var word strings.Builder
	quote := rune(0)
	started := false
	input := []rune(strings.ReplaceAll(command, "\r\n", "\n"))
	flush := func() {
		if started {
			result = append(result, word.String())
			word.Reset()
			started = false
		}
	}
	for i := 0; i < len(input); i++ {
		r := input[i]
		if quote == '\'' {
			if r == '\'' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '\\' {
			if i+1 >= len(input) {
				return nil, fmt.Errorf("unfinished backslash escape")
			}
			next := input[i+1]
			if next == '\n' {
				i++
				continue
			}
			if quote == '"' && !strings.ContainsRune("$`\"\\", next) {
				word.WriteRune(r)
				started = true
				continue
			}
			i++
			word.WriteRune(next)
			started = true
			continue
		}
		if r == '$' || r == '`' {
			return nil, fmt.Errorf("shell expansion is not supported; replace it with a literal value or a Relay {{variable}}")
		}
		if quote == '"' {
			if r == '"' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			started = true
			continue
		}
		if unicode.IsSpace(r) {
			flush()
			continue
		}
		if strings.ContainsRune(";|&<>()", r) {
			return nil, fmt.Errorf("shell operators are not supported; quote URLs and values containing special characters")
		}
		word.WriteRune(r)
		started = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	flush()
	return result, nil
}
