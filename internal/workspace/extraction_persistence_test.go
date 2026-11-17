package workspace

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/hsm-gustavo/relay/internal/extraction"
	"github.com/hsm-gustavo/relay/internal/httpclient"
)

func TestRulesPersistWithoutSessionSecrets(t *testing.T) {
	store, _ := Open(filepath.Join(t.TempDir(), "workspace.json"))
	environment, _ := store.State.SaveEnvironment("", "Local", map[string]string{"token": "manual-fallback"})
	store.State.ActiveEnvironmentID = environment
	rules := []extraction.Rule{{Path: "$.access_token", Variable: "token"}, {Path: "$.user.id", Variable: "user_id", Disabled: true}}
	request := httpclient.Request{Method: "POST", URL: "http://localhost/auth/login", Extract: rules}
	collection, _ := store.State.AddCollection("Auth")
	item, err := store.State.SaveRequest(collection, "", "Login", request)
	if err != nil {
		t.Fatal(err)
	}
	secret := "UNIQUE-SESSION-SECRET-DO-NOT-SAVE"
	response := &httpclient.Response{Status: "200 OK", Headers: http.Header{"X-Token": {secret}}, Body: []byte(`{"access_token":"` + secret + `"}`)}
	store.State.Draft = Draft{Item: item, Response: response, SessionOnly: true}
	store.State.Tabs = []Draft{store.State.Draft}
	id := store.State.AddHistory(item)
	store.State.History[0].SessionOnly = true
	store.State.History[0].ResolvedRequest = &httpclient.Request{URL: "http://localhost?token=" + secret, Auth: httpclient.Auth{Type: "Bearer", Token: secret}}
	store.State.History[0].Error = "failed: " + secret
	store.State.CompleteHistory(id, response, store.State.History[0].Error)
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(filepath.Dir(store.Path), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte(secret)) {
			t.Errorf("session secret leaked into %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.State.Collections[0].Requests[0].Request.Extract) != 2 || !reopened.State.Draft.Request.Extract[1].Disabled {
		t.Fatal("rules did not survive restart")
	}
	if reopened.State.Draft.Response != nil || reopened.State.History[0].Response != nil || reopened.State.History[0].ResolvedRequest != nil || reopened.State.History[0].Error != "" {
		t.Fatal("session-only data was restored from disk")
	}
	if store.State.Draft.Response != response || string(store.State.Draft.Response.Body) == "" {
		t.Fatal("saving mutated live response")
	}
	var fresh SessionVariables
	resolved, tainted, err := fresh.Resolve(reopened.State.ActiveEnvironment(), httpclient.Request{Auth: httpclient.Auth{Type: "Bearer", Token: "{{token}}"}})
	if err != nil || tainted || resolved.Auth.Token != "manual-fallback" {
		t.Fatal("session did not expire on restart")
	}
}

func TestEnabledRulesDefaultToVolatileResponses(t *testing.T) {
	store, _ := Open(filepath.Join(t.TempDir(), "workspace.json"))
	store.State.Draft.Request.Extract = []extraction.Rule{{Path: "$.token", Variable: "token"}}
	store.State.Draft.Response = &httpclient.Response{Body: []byte(`{"token":"must-not-persist"}`)}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State.Draft.Response != nil || !reopened.State.Draft.SessionOnly {
		t.Fatal("enabled rule response was persisted by default")
	}
}

func TestDisabledRulesKeepNormalPersistence(t *testing.T) {
	store, _ := Open(filepath.Join(t.TempDir(), "workspace.json"))
	store.State.Draft.Request.Extract = []extraction.Rule{{Path: "$.id", Variable: "id", Disabled: true}}
	store.State.Draft.Response = &httpclient.Response{Status: "200 OK", Body: []byte(`{"id":42}`)}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State.Draft.SessionOnly || reopened.State.Draft.Response == nil {
		t.Fatal("disabled rules changed normal response persistence")
	}
}
