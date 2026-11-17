package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

// Body blobs are immutable and shared by history and tabs. Editing a draft now
// rewrites only metadata rather than base64-encoding every old response again.
func (s *Store) snapshot() (State, error) {
	if s.bodies == nil {
		s.bodies = make(map[*httpclient.Response]string)
	}
	state := s.State
	state.Tabs = append([]Draft(nil), state.Tabs...)
	state.History = append([]HistoryItem(nil), state.History...)
	state.Draft.SessionOnly = state.Draft.SessionOnly || state.Draft.Request.HasEnabledExtractionRules()
	if state.Draft.SessionOnly {
		state.Draft.Response = nil
		state.Draft.Error = ""
	}
	for i := range state.Tabs {
		state.Tabs[i].SessionOnly = state.Tabs[i].SessionOnly || state.Tabs[i].Request.HasEnabledExtractionRules()
		if state.Tabs[i].SessionOnly {
			state.Tabs[i].Response = nil
			state.Tabs[i].Error = ""
		}
	}
	for i := range state.History {
		state.History[i].SessionOnly = state.History[i].SessionOnly || state.History[i].Request.HasEnabledExtractionRules()
		if state.History[i].SessionOnly {
			state.History[i].Response = nil
			state.History[i].ResolvedRequest = nil
			state.History[i].Error = ""
		}
	}
	snapshot := func(r **httpclient.Response) error {
		if *r == nil || len((*r).Body) == 0 {
			return nil
		}
		original := *r
		name, ok := s.bodies[original]
		if !ok {
			sum := sha256.Sum256(original.Body)
			name = hex.EncodeToString(sum[:]) + ".bin"
			path := filepath.Join(filepath.Dir(s.Path), "bodies", name)
			if err := writeAtomic(path, original.Body); err != nil {
				return fmt.Errorf("save response body: %w", err)
			}
			s.bodies[original] = name
		}
		copy := *original
		copy.Body = nil
		copy.BodyFile = name
		*r = &copy
		return nil
	}
	if err := snapshot(&state.Draft.Response); err != nil {
		return State{}, err
	}
	for i := range state.Tabs {
		if err := snapshot(&state.Tabs[i].Response); err != nil {
			return State{}, err
		}
	}
	for i := range state.History {
		if err := snapshot(&state.History[i].Response); err != nil {
			return State{}, err
		}
	}
	return state, nil
}

func (s *Store) loadBodies() error {
	s.bodies = make(map[*httpclient.Response]string)
	loaded := make(map[string][]byte)
	load := func(r *httpclient.Response) error {
		if r == nil || r.BodyFile == "" {
			return nil
		}
		name := r.BodyFile
		if len(name) != 68 || filepath.Ext(name) != ".bin" {
			return fmt.Errorf("invalid body reference")
		}
		if _, err := hex.DecodeString(name[:64]); err != nil {
			return fmt.Errorf("invalid body reference")
		}
		data, found := loaded[name]
		if !found {
			var err error
			data, err = os.ReadFile(filepath.Join(filepath.Dir(s.Path), "bodies", name))
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			if hex.EncodeToString(sum[:])+".bin" != name {
				return fmt.Errorf("stored response body is damaged")
			}
			loaded[name] = data
		}
		r.Body = data
		r.BodyFile = ""
		s.bodies[r] = name
		return nil
	}
	if err := load(s.State.Draft.Response); err != nil {
		return err
	}
	for i := range s.State.Tabs {
		if err := load(s.State.Tabs[i].Response); err != nil {
			return err
		}
	}
	for i := range s.State.History {
		if err := load(s.State.History[i].Response); err != nil {
			return err
		}
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".body-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func (s *Store) cleanupBodies(state State) error {
	used := make(map[string]bool)
	visit := func(r *httpclient.Response) {
		if r != nil && r.BodyFile != "" {
			used[r.BodyFile] = true
		}
	}
	visit(state.Draft.Response)
	for _, d := range state.Tabs {
		visit(d.Response)
	}
	for _, h := range state.History {
		visit(h.Response)
	}
	live := make(map[*httpclient.Response]string)
	keep := func(r *httpclient.Response) {
		if name, ok := s.bodies[r]; ok {
			live[r] = name
		}
	}
	keep(s.State.Draft.Response)
	for _, d := range s.State.Tabs {
		keep(d.Response)
	}
	for _, h := range s.State.History {
		keep(h.Response)
	}
	s.bodies = live
	dir := filepath.Join(filepath.Dir(s.Path), "bodies")
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, file := range files {
		name := file.Name()
		if len(name) != 68 || !strings.HasSuffix(name, ".bin") || used[name] {
			continue
		}
		if _, err := hex.DecodeString(name[:64]); err != nil {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("workspace saved, but old response cleanup failed: %w", err)
		}
	}
	return nil
}
