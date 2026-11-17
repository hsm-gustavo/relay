// Package workspace stores Relay's local collections, history, and current editor.
package workspace

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

type Item struct {
	ID      string
	Name    string
	Request httpclient.Request
}

type Collection struct {
	ID       string
	Name     string
	Requests []Item
}

type HistoryItem struct {
	SessionOnly     bool `json:",omitempty"`
	EnvironmentName string
	ResolvedRequest *httpclient.Request
	Item
	SentAt   time.Time
	Pending  bool
	Response *httpclient.Response
	Error    string
}

type Draft struct {
	ResponseMode    string `json:",omitempty"`
	SessionOnly     bool   `json:",omitempty"`
	EnvironmentName string
	TabID           string
	Raw             bool
	Item
	CollectionID string
	Response     *httpclient.Response
	Error        string
}

type State struct {
	Environments         []Environment
	ActiveEnvironmentID  string
	Tabs                 []Draft
	ActiveTabID          string
	SelectedCollectionID string
	Version              int
	Collections          []Collection
	History              []HistoryItem
	Draft                Draft
}

type Store struct {
	Path    string
	Warning string
	State   State
	bodies  map[*httpclient.Response]string
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "relay", "workspace.json"), nil
}

func Open(path string) (*Store, error) {
	s := &Store{Path: path, State: State{Version: 2, Draft: Draft{Item: Item{Request: httpclient.Request{Method: "GET"}}}}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read workspace: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("read workspace: %w", err)
	}
	if state.Version != 1 && state.Version != 2 {
		return nil, fmt.Errorf("unsupported workspace version %d", state.Version)
	}
	state.Version = 2
	s.State = state
	if err := s.loadBodies(); err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	for i := range s.State.History {
		if s.State.History[i].Pending {
			s.State.History[i].Pending = false
			s.State.History[i].Error = "Request interrupted when Relay closed"
		}
	}
	return s, nil
}

// Save replaces the previous file only after the complete new contents are synced.
func (s *Store) Save() error {
	state, err := s.snapshot()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".workspace-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), s.Path); err != nil {
		return err
	}
	s.Warning = ""
	if err := s.cleanupBodies(state); err != nil {
		s.Warning = err.Error()
	}
	return nil
}

func (s *State) AddCollection(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("enter a collection name")
	}
	for _, c := range s.Collections {
		if strings.EqualFold(c.Name, name) {
			return "", fmt.Errorf("a collection with that name already exists")
		}
	}
	id := rand.Text()
	s.Collections = append(s.Collections, Collection{ID: id, Name: name})
	return id, nil
}

func (s *State) RenameCollection(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("enter a collection name")
	}
	for _, c := range s.Collections {
		if c.ID != id && strings.EqualFold(c.Name, name) {
			return fmt.Errorf("a collection with that name already exists")
		}
	}
	for i := range s.Collections {
		if s.Collections[i].ID == id {
			s.Collections[i].Name = name
			return nil
		}
	}
	return fmt.Errorf("collection no longer exists")
}

func (s *State) DeleteCollection(id string) {
	for i, c := range s.Collections {
		if c.ID == id {
			s.Collections = append(s.Collections[:i], s.Collections[i+1:]...)
			break
		}
	}
	for i := range s.Tabs {
		if s.Tabs[i].CollectionID == id {
			s.Tabs[i].CollectionID = ""
			s.Tabs[i].ID = ""
		}
	}
	if s.Draft.CollectionID == id {
		s.Draft.CollectionID = ""
		s.Draft.ID = ""
	}
}

func (s *State) SaveRequest(collectionID, id, name string, request httpclient.Request) (Item, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Item{}, fmt.Errorf("enter a request name")
	}
	target := -1
	for i, c := range s.Collections {
		if c.ID == collectionID {
			target = i
			break
		}
	}
	if target < 0 {
		return Item{}, fmt.Errorf("choose a collection before saving")
	}
	if id == "" {
		id = rand.Text()
	}
	item := Item{ID: id, Name: name, Request: request}
	// Removing an existing ID also allows moving a saved request between collections.
	for i := range s.Collections {
		for j, old := range s.Collections[i].Requests {
			if old.ID == id {
				if i == target {
					s.Collections[i].Requests[j] = item
					return item, nil
				}
				s.Collections[i].Requests = append(s.Collections[i].Requests[:j], s.Collections[i].Requests[j+1:]...)
				break
			}
		}
	}
	s.Collections[target].Requests = append(s.Collections[target].Requests, item)
	return item, nil
}

func (s *State) DeleteRequest(collectionID, id string) {
	for i := range s.Collections {
		if s.Collections[i].ID != collectionID {
			continue
		}
		for j, item := range s.Collections[i].Requests {
			if item.ID == id {
				s.Collections[i].Requests = append(s.Collections[i].Requests[:j], s.Collections[i].Requests[j+1:]...)
				break
			}
		}
	}
	for i := range s.Tabs {
		if s.Tabs[i].ID == id {
			s.Tabs[i].ID = ""
		}
	}
	if s.Draft.ID == id {
		s.Draft.ID = ""
	}
}

func (s *State) AddHistory(item Item) string {
	item.ID = rand.Text()
	if strings.TrimSpace(item.Name) == "" {
		item.Name = item.Request.URL
	}
	s.History = append([]HistoryItem{{Item: item, SentAt: time.Now(), Pending: true}}, s.History...)
	return item.ID
}

func (s *State) CompleteHistory(id string, response *httpclient.Response, requestError string) {
	for i := range s.History {
		if s.History[i].ID == id {
			s.History[i].Pending = false
			s.History[i].Response = response
			s.History[i].Error = requestError
			return
		}
	}
}

func (s *State) DeleteHistory(id string) {
	for i, item := range s.History {
		if item.ID == id {
			s.History = append(s.History[:i], s.History[i+1:]...)
			return
		}
	}
}
