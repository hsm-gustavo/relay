package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

func TestBodyBlobsRoundTripAndCleanup(t *testing.T) {
	store, _ := Open(filepath.Join(t.TempDir(), "workspace.json"))
	body := strings.Repeat("response body", 100000)
	response := &httpclient.Response{Status: "200 OK", Body: []byte(body)}
	store.State.Draft.Response = response
	id := store.State.AddHistory(Item{Name: "large"})
	store.State.CompleteHistory(id, response, "")
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	metadata, _ := os.ReadFile(store.Path)
	if len(metadata) > 4096 {
		t.Fatal("body was embedded in metadata")
	}
	blobs, _ := filepath.Glob(filepath.Join(filepath.Dir(store.Path), "bodies", "*.bin"))
	if len(blobs) != 1 {
		t.Fatal("shared response was not deduplicated")
	}
	info, _ := os.Stat(blobs[0])
	store.State.Draft.Name = "editing without rewriting bodies"
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	updated, _ := os.Stat(blobs[0])
	if !updated.ModTime().Equal(info.ModTime()) {
		t.Fatal("body rewritten on draft edit")
	}
	reopened, err := Open(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(reopened.State.Draft.Response.Body) != body || string(reopened.State.History[0].Response.Body) != body {
		t.Fatal("body was not restored")
	}
	reopened.State.Draft.Response = nil
	reopened.State.DeleteHistory(id)
	if err := reopened.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(blobs[0]); !os.IsNotExist(err) {
		t.Fatal("unreferenced body was not removed")
	}
}

func TestMissingOrDamagedBlobIsReported(t *testing.T) {
	store, _ := Open(filepath.Join(t.TempDir(), "workspace.json"))
	store.State.Draft.Response = &httpclient.Response{Body: []byte("original")}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	blobs, _ := filepath.Glob(filepath.Join(filepath.Dir(store.Path), "bodies", "*.bin"))
	if err := os.WriteFile(blobs[0], []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(store.Path); err == nil {
		t.Fatal("damaged blob accepted")
	}
	if err := os.Remove(blobs[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(store.Path); err == nil {
		t.Fatal("missing blob accepted")
	}
}

func TestLegacyWorkspaceMigratesToBodyStorage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	// Version 1 stores response bytes directly in metadata (base64).
	if err := os.WriteFile(path, []byte(`{"Version":1,"Draft":{"Response":{"Status":"200 OK","Body":"bGVnYWN5"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if store.State.Version != 2 || string(store.State.Draft.Response.Body) != "legacy" {
		t.Fatal("legacy response was lost")
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(reopened.State.Draft.Response.Body) != "legacy" {
		t.Fatal("migrated response was lost")
	}
}
