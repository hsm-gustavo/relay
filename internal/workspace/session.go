package workspace

import (
	"sync"

	"github.com/hsm-gustavo/relay/internal/extraction"
	"github.com/hsm-gustavo/relay/internal/httpclient"
)

// SessionVariables is intentionally separate from State and Store; it is never persisted.
// Values are keyed by stable environment ID, not the currently selected environment.
type SessionVariables struct {
	mu     sync.RWMutex
	values map[string]map[string]string
}

func (s *SessionVariables) Resolve(env *Environment, request httpclient.Request) (httpclient.Request, bool, error) {
	variables := make(map[string]string)
	if env != nil {
		for key, value := range env.Variables {
			variables[key] = value
		}
	}
	s.mu.RLock()
	session := make(map[string]string)
	if env != nil {
		for key, value := range s.values[env.ID] {
			variables[key] = value
			session[key] = value
		}
	}
	s.mu.RUnlock()
	resolved, used, err := request.ResolveTracked(variables)
	sensitive := false
	for name := range used {
		if _, ok := session[name]; ok {
			sensitive = true
		}
	}
	return resolved, sensitive, err
}

func (s *SessionVariables) Apply(environmentID string, results []extraction.Result) []extraction.Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.values == nil {
		s.values = make(map[string]map[string]string)
	}
	for i := range results {
		result := &results[i]
		if result.Status != extraction.Success {
			continue
		}
		if environmentID == "" {
			result.Status = extraction.Skipped
			result.Message = "Select an environment to store extracted values"
			result.Value = ""
			continue
		}
		if !httpclient.ValidVariableName(result.Rule.Variable) {
			result.Status = extraction.Failed
			result.Message = "Invalid destination variable name; previous value kept"
			result.Value = ""
			continue
		}
		if s.values[environmentID] == nil {
			s.values[environmentID] = make(map[string]string)
		}
		s.values[environmentID][result.Rule.Variable] = result.Value
	}
	return results
}

func (s *SessionVariables) Clear(environmentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, environmentID)
}
