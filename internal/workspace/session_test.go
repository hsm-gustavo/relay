package workspace

import (
	"sync"
	"testing"

	"github.com/hsm-gustavo/relay/internal/extraction"
	"github.com/hsm-gustavo/relay/internal/httpclient"
)

func TestSessionPrecedenceIsolationAndFailure(t *testing.T) {
	var session SessionVariables
	local := &Environment{ID: "local", Variables: map[string]string{"token": "manual", "base_url": "http://localhost"}}
	production := &Environment{ID: "production", Variables: map[string]string{"token": "production"}}
	request := httpclient.Request{URL: "{{base_url}}", Auth: httpclient.Auth{Type: "Bearer", Token: "{{token}}"}}
	results := extraction.Evaluate(200, []byte(`{"token":"first"}`), true, []extraction.Rule{{Path: "$.token", Variable: "token"}})
	session.Apply(local.ID, results)
	resolved, sensitive, err := session.Resolve(local, request)
	if err != nil || !sensitive || resolved.Auth.Token != "first" || local.Variables["token"] != "manual" {
		t.Fatal("session did not override manual value without modifying it")
	}
	session.Apply(local.ID, extraction.Evaluate(200, []byte(`{"token":"second"}`), true, []extraction.Rule{{Path: "$.token", Variable: "token"}}))
	session.Apply(local.ID, extraction.Evaluate(200, []byte(`{}`), true, []extraction.Rule{{Path: "$.token", Variable: "token"}}))
	resolved, _, _ = session.Resolve(local, request)
	if resolved.Auth.Token != "second" {
		t.Fatal("failed extraction erased previous value")
	}
	other, tainted, err := session.Resolve(production, httpclient.Request{URL: "https://example.com", Auth: request.Auth})
	if err != nil || tainted || other.Auth.Token != "production" {
		t.Fatal("session leaked between environments")
	}
	session.Clear(local.ID)
	resolved, sensitive, err = session.Resolve(local, request)
	if err != nil || sensitive || resolved.Auth.Token != "manual" {
		t.Fatal("clear did not reveal original manual value")
	}
	good := extraction.Evaluate(200, []byte(`{"token":"s"}`), true, []extraction.Rule{{Path: "$.token", Variable: "bad name"}})
	if session.Apply(local.ID, good)[0].Status != extraction.Failed {
		t.Fatal("invalid variable name was stored")
	}
	good = extraction.Evaluate(200, []byte(`{"token":"s"}`), true, []extraction.Rule{{Path: "$.token", Variable: "token"}})
	if session.Apply("", good)[0].Status != extraction.Skipped {
		t.Fatal("no-environment extraction was stored")
	}
}

func TestSessionConcurrentAccess(t *testing.T) {
	var session SessionVariables
	env := &Environment{ID: "local", Variables: map[string]string{}}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				results := extraction.Evaluate(200, []byte(`{"id":42}`), true, []extraction.Rule{{Path: "$.id", Variable: "user_id"}})
				session.Apply(env.ID, results)
				if _, _, err := session.Resolve(env, httpclient.Request{URL: "https://example.com/{{user_id}}"}); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}
