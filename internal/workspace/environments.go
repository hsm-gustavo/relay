package workspace

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

type Environment struct {
	ID        string
	Name      string
	Variables map[string]string
}

func ParseVariables(text string) (map[string]string, error) {
	variables := make(map[string]string)
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !httpclient.ValidVariableName(key) {
			return nil, fmt.Errorf("line %d: use name=value; names start with a letter or underscore and may include digits, dots, or hyphens", i+1)
		}
		if _, exists := variables[key]; exists {
			return nil, fmt.Errorf("line %d: duplicate variable %q", i+1, key)
		}
		variables[key] = value
	}
	return variables, nil
}

func (s *State) SaveEnvironment(id, name string, variables map[string]string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "No environment") {
		return "", fmt.Errorf("enter an environment name other than No environment")
	}
	for _, env := range s.Environments {
		if env.ID != id && strings.EqualFold(env.Name, name) {
			return "", fmt.Errorf("an environment with that name already exists")
		}
	}
	values := make(map[string]string, len(variables))
	for key, value := range variables {
		if !httpclient.ValidVariableName(key) {
			return "", fmt.Errorf("invalid variable name %q", key)
		}
		values[key] = value
	}
	if id == "" {
		id = rand.Text()
		s.Environments = append(s.Environments, Environment{ID: id, Name: name, Variables: values})
		return id, nil
	}
	for i := range s.Environments {
		if s.Environments[i].ID == id {
			s.Environments[i] = Environment{ID: id, Name: name, Variables: values}
			return id, nil
		}
	}
	return "", fmt.Errorf("environment no longer exists")
}

func (s *State) DeleteEnvironment(id string) {
	for i, env := range s.Environments {
		if env.ID == id {
			s.Environments = append(s.Environments[:i], s.Environments[i+1:]...)
			break
		}
	}
	if s.ActiveEnvironmentID == id {
		s.ActiveEnvironmentID = ""
	}
}

func (s *State) ActiveEnvironment() *Environment {
	for i := range s.Environments {
		if s.Environments[i].ID == s.ActiveEnvironmentID {
			return &s.Environments[i]
		}
	}
	return nil
}
