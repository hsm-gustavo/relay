package workspace

import (
	"path/filepath"
	"testing"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

func TestEnvironmentsPersistAndDelete(t *testing.T) {
	store, _ := Open(filepath.Join(t.TempDir(), "workspace.json"))
	local, err := store.State.SaveEnvironment("", "Local", map[string]string{"base_url": "http://localhost:8080"})
	if err != nil {
		t.Fatal(err)
	}
	production, _ := store.State.SaveEnvironment("", "Production", map[string]string{"base_url": "https://api.example.com"})
	store.State.ActiveEnvironmentID = local
	_, err = store.State.SaveEnvironment(local, "Development", map[string]string{"base_url": "http://localhost:9000"})
	if err != nil {
		t.Fatal(err)
	}
	store.State.Draft.Request = httpclient.Request{Method: "GET", URL: "{{base_url}}/users"}
	history := store.State.AddHistory(Item{Request: store.State.Draft.Request})
	resolved, _ := store.State.Draft.Request.Resolve(store.State.ActiveEnvironment().Variables)
	store.State.History[0].EnvironmentName = "Development"
	store.State.History[0].ResolvedRequest = &resolved
	store.State.CompleteHistory(history, nil, "")
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State.ActiveEnvironment().Name != "Development" || reopened.State.ActiveEnvironment().Variables["base_url"] != "http://localhost:9000" {
		t.Fatal("active environment was not restored")
	}
	if reopened.State.History[0].ResolvedRequest.URL != "http://localhost:9000/users" || reopened.State.History[0].Request.URL != "{{base_url}}/users" {
		t.Fatal("history lost template or resolved snapshot")
	}
	reopened.State.DeleteEnvironment(local)
	if reopened.State.ActiveEnvironment() != nil || reopened.State.ActiveEnvironmentID != "" || len(reopened.State.Environments) != 1 || reopened.State.Environments[0].ID != production {
		t.Fatal("deleting active environment did not clear selection")
	}
	if reopened.State.Draft.Request.URL != "{{base_url}}/users" {
		t.Fatal("deletion changed request template")
	}
	if err := reopened.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err = Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State.ActiveEnvironmentID != "" || len(reopened.State.Environments) != 1 {
		t.Fatal("deleted environment returned")
	}
}

func TestEnvironmentValidation(t *testing.T) {
	for _, text := range []string{"missing equal", "=value", "a=one\na=two", "bad name=value", "1name=value"} {
		if _, err := ParseVariables(text); err == nil {
			t.Errorf("accepted variables %q", text)
		}
	}
	values, err := ParseVariables("base_url=http://localhost:8080\r\ntoken=a=b=c\nempty=\n\n")
	if err != nil || values["token"] != "a=b=c" || values["empty"] != "" {
		t.Fatal("valid values were not preserved")
	}
	var state State
	id, _ := state.SaveEnvironment("", "Local", values)
	values["base_url"] = "mutated"
	if state.Environments[0].Variables["base_url"] == "mutated" {
		t.Fatal("environment retained mutable caller map")
	}
	for _, name := range []string{"", " ", "local", "No environment"} {
		if _, err := state.SaveEnvironment("", name, nil); err == nil {
			t.Errorf("accepted name %q", name)
		}
	}
	if _, err := state.SaveEnvironment(id, "Renamed", nil); err != nil {
		t.Fatal(err)
	}
}
