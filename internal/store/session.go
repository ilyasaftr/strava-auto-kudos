package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Session struct {
	Cookies     map[string]string `json:"cookies"`
	ExpiresAt   int64             `json:"expires_at,omitempty"`
	RefreshedAt int64             `json:"refreshed_at,omitempty"`
}

func (s Session) SessionCookie() string {
	if s.Cookies == nil {
		return ""
	}
	return s.Cookies["_strava4_session"]
}

type SessionFile struct {
	mu   sync.Mutex
	path string
}

func NewSessionFile(path string) *SessionFile {
	return &SessionFile{path: path}
}

func (f *SessionFile) Load() (Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return Session{}, os.ErrNotExist
	}
	if err != nil {
		return Session{}, fmt.Errorf("read session: %w", err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return Session{}, fmt.Errorf("parse session: %w", err)
	}
	if sess.Cookies == nil {
		sess.Cookies = map[string]string{}
	}
	return sess, nil
}

func (f *SessionFile) Save(sess Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	if sess.Cookies == nil {
		sess.Cookies = map[string]string{}
	}
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	if err := os.WriteFile(f.path, data, 0o600); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}
