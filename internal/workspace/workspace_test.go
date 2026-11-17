package workspace

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

func TestWorkspaceSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay", "workspace.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	collectionID, err := store.State.AddCollection("Local API")
	if err != nil {
		t.Fatal(err)
	}
	request := httpclient.Request{Method: "POST", URL: "localhost:8080/api", Params: "tag=one\ntag=two", Body: `{"hello":"world"}`}
	item, err := store.State.SaveRequest(collectionID, "", "Create item", request)
	if err != nil {
		t.Fatal(err)
	}
	response := &httpclient.Response{Status: "201 Created", Duration: 25 * time.Millisecond, Headers: http.Header{"X-Test": {"one", "two"}}, Body: []byte{0, 255, '\n'}}
	id := store.State.AddHistory(item)
	store.State.History[0].SentAt = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	store.State.CompleteHistory(id, response, "")
	store.State.Draft = Draft{Item: item, CollectionID: collectionID, Response: response, Raw: true}
	store.State.SelectedCollectionID = collectionID
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.State, reopened.State) {
		t.Fatalf("reopened workspace differs:\n%+v\n%+v", store.State, reopened.State)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %o", info.Mode().Perm())
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".workspace-*"))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

func TestEditMoveAndDeleteSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	store, _ := Open(path)
	first, _ := store.State.AddCollection("First")
	second, _ := store.State.AddCollection("Second")
	item, _ := store.State.SaveRequest(first, "", "Original", httpclient.Request{Method: "GET", URL: "example.com"})
	historyID := store.State.AddHistory(item)
	store.State.CompleteHistory(historyID, nil, "connection refused")
	store.State.Draft = Draft{Item: item, CollectionID: first}
	updated, err := store.State.SaveRequest(second, item.ID, "Renamed", httpclient.Request{Method: "DELETE", URL: "example.com/item"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != item.ID || len(store.State.Collections[0].Requests) != 0 || len(store.State.Collections[1].Requests) != 1 {
		t.Fatal("move should preserve ID and remove original")
	}
	if err := store.State.RenameCollection(second, "Renamed collection"); err != nil {
		t.Fatal(err)
	}
	_, err = store.State.SaveRequest(second, item.ID, "Edited again", updated.Request)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.State.Collections[1].Requests) != 1 {
		t.Fatal("edit created a duplicate")
	}
	if store.State.History[0].Name != "Original" || store.State.History[0].Request.Method != "GET" {
		t.Fatal("history must retain its snapshot")
	}
	store.State.DeleteCollection(first)
	if store.State.Draft.ID != "" || store.State.Draft.Request.URL != "example.com" {
		t.Fatal("deleting collection must detach but preserve draft")
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if store.State.Collections[0].Name != "Renamed collection" || store.State.Collections[0].Requests[0].Name != "Edited again" || store.State.History[0].Error != "connection refused" {
		t.Fatal("edits were not persisted")
	}
	store.State.DeleteRequest(second, item.ID)
	store.State.DeleteHistory(historyID)
	// A response finishing after deletion must not resurrect the history item.
	store.State.CompleteHistory(historyID, &httpclient.Response{Status: "200 OK"}, "")
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.State.Collections[0].Requests) != 0 || len(store.State.History) != 0 {
		t.Fatal("deleted items returned after reopen")
	}
}

func TestInvalidNames(t *testing.T) {
	var state State
	id, _ := state.AddCollection("API")
	for _, name := range []string{" ", "api", " API "} {
		if _, err := state.AddCollection(name); err == nil {
			t.Errorf("accepted collection name %q", name)
		}
	}
	other, _ := state.AddCollection("Other")
	if err := state.RenameCollection(other, "API"); err == nil {
		t.Error("accepted duplicate name")
	}
	if err := state.RenameCollection(id, " "); err == nil {
		t.Error("accepted empty name")
	}
	if _, err := state.SaveRequest(id, "", " ", httpclient.Request{}); err == nil {
		t.Error("accepted empty request name")
	}
	if _, err := state.SaveRequest("missing", "", "Request", httpclient.Request{}); err == nil {
		t.Error("accepted missing collection")
	}
}

func TestInvalidStorageIsNotOverwritten(t *testing.T) {
	for _, data := range []string{"{broken", `{"Version":42}`, `{}`} {
		t.Run(data, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(path); err == nil {
				t.Fatal("expected load error")
			}
			got, _ := os.ReadFile(path)
			if string(got) != data {
				t.Fatal("original data was changed")
			}
		})
	}
}

func TestSaveFailurePreservesPreviousFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	store, _ := Open(path)
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(path)
	// A path beneath a regular file cannot be created, even when tests run as root.
	store.Path = filepath.Join(path, "child.json")
	if err := store.Save(); err == nil {
		t.Fatal("expected write error")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Fatal("failed save changed original data")
	}
	store.Path = path
	if _, err := store.State.AddCollection("Recovery"); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	if !strings.Contains(string(got), "Recovery") {
		t.Fatal("subsequent save failed")
	}
}

func TestInterruptedHistoryIsRestored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	store, _ := Open(path)
	store.State.AddHistory(Item{Name: "Pending", Request: httpclient.Request{Method: "GET", URL: "localhost:8080"}})
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	item := reopened.State.History[0]
	if item.Pending || item.Error != "Request interrupted when Relay closed" {
		t.Fatalf("interrupted history = %+v", item)
	}
}
