package responseview

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

type Node struct {
	ID       string
	Name     string
	Kind     string
	Value    string
	Children []string
}

// BuildTree preserves object order, duplicate property names and JSON numbers.
// Node and depth budgets prevent large or adversarial structures reaching the UI.
func BuildTree(body []byte) (map[string]Node, string) {
	if len(body) > TreeLimit {
		return nil, "Tree unavailable: JSON exceeds 256 KiB."
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	nodes := make(map[string]Node)
	var parse func(string, int) (string, error)
	parse = func(name string, depth int) (string, error) {
		if depth > MaxDepth || len(nodes) >= MaxNodes {
			return "", fmt.Errorf("Tree limit reached (5,000 nodes / 64 levels).")
		}
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		id := strconv.Itoa(len(nodes))
		node := Node{ID: id, Name: name}
		nodes[id] = node
		switch value := token.(type) {
		case json.Delim:
			switch value {
			case '{':
				node.Kind = "object"
				for decoder.More() {
					key, err := decoder.Token()
					if err != nil {
						return "", err
					}
					property, ok := key.(string)
					if !ok {
						return "", fmt.Errorf("invalid object property")
					}
					child, err := parse(property, depth+1)
					if err != nil {
						return "", err
					}
					node.Children = append(node.Children, child)
				}
			case '[':
				node.Kind = "array"
				for i := 0; decoder.More(); i++ {
					child, err := parse("["+strconv.Itoa(i)+"]", depth+1)
					if err != nil {
						return "", err
					}
					node.Children = append(node.Children, child)
				}
			default:
				return "", fmt.Errorf("unexpected JSON delimiter")
			}
			if _, err := decoder.Token(); err != nil {
				return "", err
			}
		case string:
			node.Kind = "string"
			encoded, _ := json.Marshal(value)
			node.Value = string(encoded)
		case json.Number:
			node.Kind = "number"
			node.Value = value.String()
		case bool:
			node.Kind = "boolean"
			node.Value = strconv.FormatBool(value)
		case nil:
			node.Kind = "null"
			node.Value = "null"
		default:
			return "", fmt.Errorf("unsupported JSON value")
		}
		nodes[id] = node
		return id, nil
	}
	if _, err := parse("", 0); err != nil {
		return nil, err.Error()
	}
	if !json.Valid(body) {
		return nil, "Invalid JSON; Tree unavailable."
	}
	return nodes, ""
}

func (n Node) Label() string {
	name := n.Name
	if name != "" {
		name += ": "
	}
	if n.Kind == "object" || n.Kind == "array" {
		return name + n.Kind + " [" + strconv.Itoa(len(n.Children)) + "]"
	}
	return name + n.Value
}
