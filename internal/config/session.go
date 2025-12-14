package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Session represents a saved cleanup session
type Session struct {
	ID                 string    `json:"id"`
	Timestamp          time.Time `json:"timestamp"`
	SelectedCategories []string  `json:"selected_categories"`
	SelectedFiles      []string  `json:"selected_files"`
	TotalSize          int64     `json:"total_size"`
	TotalCount         int       `json:"total_count"`
	Notes              string    `json:"notes"`
}

// SessionManager manages session persistence
type SessionManager struct {
	sessionsDir string
}

// NewSessionManager creates a new session manager
func NewSessionManager() (*SessionManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".config", "cleanup-cache", "sessions")

	// Create sessions directory if it doesn't exist
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &SessionManager{
		sessionsDir: sessionsDir,
	}, nil
}

// Save saves a session to disk
func (sm *SessionManager) Save(session *Session) error {
	// Generate ID if not set
	if session.ID == "" {
		session.ID = generateSessionID()
	}

	// Set timestamp if not set
	if session.Timestamp.IsZero() {
		session.Timestamp = time.Now()
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Write to file
	filename := filepath.Join(sm.sessionsDir, session.ID+".json")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// Load loads a session from disk by ID
func (sm *SessionManager) Load(id string) (*Session, error) {
	filename := filepath.Join(sm.sessionsDir, id+".json")

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// List returns all saved sessions
func (sm *SessionManager) List() ([]*Session, error) {
	entries, err := os.ReadDir(sm.sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		session, err := sm.Load(id)
		if err != nil {
			// Skip invalid sessions
			continue
		}

		sessions = append(sessions, session)
	}

	// Sort by timestamp (newest first)
	for i := 0; i < len(sessions)-1; i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[i].Timestamp.Before(sessions[j].Timestamp) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	return sessions, nil
}

// Delete deletes a session by ID
func (sm *SessionManager) Delete(id string) error {
	filename := filepath.Join(sm.sessionsDir, id+".json")
	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("failed to delete session file: %w", err)
	}
	return nil
}

// GetLatest returns the most recent session
func (sm *SessionManager) GetLatest() (*Session, error) {
	sessions, err := sm.List()
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found")
	}

	return sessions[0], nil
}

// SaveCurrent saves the current selection as a session
func SaveCurrentSession(categories []string, files []string, totalSize int64, notes string) error {
	sm, err := NewSessionManager()
	if err != nil {
		return err
	}

	session := &Session{
		SelectedCategories: categories,
		SelectedFiles:      make([]string, len(files)),
		TotalSize:          totalSize,
		TotalCount:         len(files),
		Notes:              notes,
	}

	// Store just file paths
	copy(session.SelectedFiles, files)

	return sm.Save(session)
}

// LoadLatestSession loads the most recent session
func LoadLatestSession() (*Session, error) {
	sm, err := NewSessionManager()
	if err != nil {
		return nil, err
	}

	return sm.GetLatest()
}

// Helper functions

func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().Unix())
}

// CleanOldSessions removes sessions older than specified days
func (sm *SessionManager) CleanOldSessions(days int) error {
	sessions, err := sm.List()
	if err != nil {
		return err
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	for _, session := range sessions {
		if session.Timestamp.Before(cutoff) {
			if err := sm.Delete(session.ID); err != nil {
				// Log error but continue
				continue
			}
		}
	}

	return nil
}

// GetSessionsDir returns the sessions directory path
func (sm *SessionManager) GetSessionsDir() string {
	return sm.sessionsDir
}
