// Package extraction implements a restricted, side-effect-free JSON path extractor.
package extraction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Rule struct {
	Path     string
	Variable string
	Disabled bool
}
type Result struct {
	Rule    Rule
	Status  string
	Message string
	Value   string `json:"-"`
}

const (
	Success  = "Success"
	Failed   = "Failed"
	Skipped  = "Skipped"
	Disabled = "Disabled"
)

type step struct {
	key   string
	index int
	array bool
}

func parsePath(path string) ([]step, error) {
	path = strings.TrimSpace(path)
	if path == "" || path[0] != '$' {
		return nil, fmt.Errorf("path must start with $")
	}
	var steps []step
	for i := 1; i < len(path); {
		switch path[i] {
		case '.':
			i++
			start := i
			for i < len(path) && path[i] != '.' && path[i] != '[' {
				r := rune(path[i])
				if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
					return nil, fmt.Errorf("only dot properties and numeric [index] steps are supported")
				}
				i++
			}
			if i == start {
				return nil, fmt.Errorf("empty property name")
			}
			steps = append(steps, step{key: path[start:i]})
		case '[':
			start := i + 1
			end := strings.IndexByte(path[start:], ']')
			if end < 0 {
				return nil, fmt.Errorf("unclosed array index")
			}
			end += start
			digits := path[start:end]
			if digits == "" {
				return nil, fmt.Errorf("array index must be a nonnegative integer")
			}
			for _, r := range digits {
				if r < '0' || r > '9' {
					return nil, fmt.Errorf("array index must be a nonnegative integer")
				}
			}
			n, err := strconv.Atoi(digits)
			if err != nil {
				return nil, fmt.Errorf("array index is too large")
			}
			steps = append(steps, step{array: true, index: n})
			i = end + 1
		default:
			return nil, fmt.Errorf("only dot properties and numeric [index] steps are supported")
		}
	}
	return steps, nil
}

func scalar(root any, path string) (string, error) {
	steps, err := parsePath(path)
	if err != nil {
		return "", err
	}
	value := root
	for _, step := range steps {
		if step.array {
			items, ok := value.([]any)
			if !ok || step.index >= len(items) {
				return "", fmt.Errorf("array element does not exist")
			}
			value = items[step.index]
		} else {
			object, ok := value.(map[string]any)
			if !ok {
				return "", fmt.Errorf("property does not exist")
			}
			value, ok = object[step.key]
			if !ok {
				return "", fmt.Errorf("property does not exist")
			}
		}
	}
	switch value := value.(type) {
	case string:
		return value, nil
	case json.Number:
		return value.String(), nil
	case bool:
		return strconv.FormatBool(value), nil
	case nil:
		return "", fmt.Errorf("null cannot be extracted; previous value kept")
	default:
		return "", fmt.Errorf("objects and arrays cannot be extracted; choose a primitive property")
	}
}

// Evaluate only extracts complete 2xx JSON responses. Failures never produce values.
func Evaluate(status int, body []byte, complete bool, rules []Rule) []Result {
	if len(rules) == 0 {
		return nil
	}
	results := make([]Result, len(rules))
	var root any
	enabled := false
	for _, rule := range rules {
		if !rule.Disabled {
			enabled = true
			break
		}
	}
	valid := enabled && complete && status >= 200 && status < 300 && json.Valid(body)
	if valid {
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		valid = decoder.Decode(&root) == nil
	}
	for i, rule := range rules {
		result := Result{Rule: rule}
		switch {
		case rule.Disabled:
			result.Status = Disabled
			result.Message = "Rule disabled"
		case !complete:
			result.Status = Skipped
			result.Message = "No complete response; previous value kept"
		case status < 200 || status >= 300:
			result.Status = Skipped
			result.Message = "Response is not 2xx; previous value kept"
		case !valid:
			result.Status = Failed
			result.Message = "Response is not valid JSON; previous value kept"
		default:
			value, err := scalar(root, rule.Path)
			if err != nil {
				result.Status = Failed
				result.Message = err.Error()
			} else {
				result.Status = Success
				result.Value = value
				result.Message = "Available for this session"
			}
		}
		results[i] = result
	}
	return results
}
