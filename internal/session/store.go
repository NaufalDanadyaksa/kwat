package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Store handles persisting and retrieving sessions from disk.
type Store struct {
	baseDir string
}

// NewStore initializes a session store.
func NewStore(customDir string) (*Store, error) {
	dir := customDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			dir = ".kwat/sessions"
		} else {
			dir = filepath.Join(home, ".kwat", "sessions")
		}
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	return &Store{baseDir: dir}, nil
}

// Save writes a session to disk atomically.
func (s *Store) Save(sess *Session) error {
	if sess == nil || sess.ID == "" {
		return fmt.Errorf("cannot save nil or empty session")
	}

	sess.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	targetPath := filepath.Join(s.baseDir, sess.ID+".json")
	tmpPath := targetPath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tmp session file: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit session file: %w", err)
	}

	return nil
}

// Load reads a session from disk by its ID.
func (s *Store) Load(id string) (*Session, error) {
	cleanID := strings.TrimSuffix(id, ".json")
	filePath := filepath.Join(s.baseDir, cleanID+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session %s: %w", id, err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("corrupted session data for %s: %w", id, err)
	}

	return &sess, nil
}

// GetLatest finds the most recently updated session.
func (s *Store) GetLatest() (*Session, error) {
	sessions, err := s.List()
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no previous sessions found")
	}
	return sessions[0], nil
}

// List returns all stored sessions sorted by UpdatedAt descending.
func (s *Store) List() ([]*Session, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read session dir: %w", err)
	}

	var list []*Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		sess, err := s.Load(entry.Name())
		if err == nil {
			list = append(list, sess)
		}
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})

	return list, nil
}

