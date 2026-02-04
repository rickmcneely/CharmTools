package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"charmtool/internal/models"

	"github.com/google/uuid"
)

// FileStore manages session-based file storage
type FileStore struct {
	baseDir    string
	maxAge     time.Duration
	mu         sync.RWMutex
	sessions   map[string]*sessionData
	stats      *Stats
}

// Stats tracks usage statistics
type Stats struct {
	TotalUsers      int `json:"totalUsers"`
	TotalPOSUploads int `json:"totalPosUploads"`
}

type sessionData struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
	XFile     *models.XFile
}

// NewFileStore creates a new file store
func NewFileStore(baseDir string, maxAge time.Duration) (*FileStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	store := &FileStore{
		baseDir:  baseDir,
		maxAge:   maxAge,
		sessions: make(map[string]*sessionData),
		stats:    &Stats{},
	}

	// Load stats from disk
	if err := store.loadStats(); err != nil {
		fmt.Printf("Warning: could not load stats: %v\n", err)
	}

	// Load existing sessions from disk
	if err := store.loadSessions(); err != nil {
		// Log but don't fail - start fresh
		fmt.Printf("Warning: could not load existing sessions: %v\n", err)
	}

	return store, nil
}

// loadStats loads stats from disk
func (fs *FileStore) loadStats() error {
	statsPath := filepath.Join(fs.baseDir, "stats.json")
	data, err := os.ReadFile(statsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No stats file yet
		}
		return err
	}
	return json.Unmarshal(data, fs.stats)
}

// saveStats saves stats to disk (caller must hold lock)
func (fs *FileStore) saveStats() error {
	data, err := json.MarshalIndent(fs.stats, "", "  ")
	if err != nil {
		return err
	}
	statsPath := filepath.Join(fs.baseDir, "stats.json")
	return os.WriteFile(statsPath, data, 0644)
}

// GetStats returns current stats
func (fs *FileStore) GetStats() Stats {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return *fs.stats
}

// IncrementPOSUploads increments the POS upload counter
func (fs *FileStore) IncrementPOSUploads() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.stats.TotalPOSUploads++
	fs.saveStats()
}

// loadSessions loads all existing session files from disk
func (fs *FileStore) loadSessions() error {
	entries, err := os.ReadDir(fs.baseDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionID := entry.Name()[:len(entry.Name())-5] // Remove .json
		data, err := os.ReadFile(filepath.Join(fs.baseDir, entry.Name()))
		if err != nil {
			continue
		}

		var xf models.XFile
		if err := json.Unmarshal(data, &xf); err != nil {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fs.sessions[sessionID] = &sessionData{
			ID:        sessionID,
			CreatedAt: xf.Metadata.Created,
			UpdatedAt: info.ModTime(),
			XFile:     &xf,
		}
	}

	return nil
}

// CreateSession creates a new session and returns its ID
func (fs *FileStore) CreateSession() (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	sessionID := uuid.New().String()
	xf := models.NewXFile()

	session := &sessionData{
		ID:        sessionID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		XFile:     xf,
	}

	fs.sessions[sessionID] = session

	if err := fs.saveSession(sessionID); err != nil {
		delete(fs.sessions, sessionID)
		return "", err
	}

	// Increment user count
	fs.stats.TotalUsers++
	fs.saveStats()

	return sessionID, nil
}

// TouchSession updates the session's UpdatedAt timestamp to restart the 10-day expiry
func (fs *FileStore) TouchSession(sessionID string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	session, ok := fs.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.UpdatedAt = time.Now()
	return fs.saveSession(sessionID)
}

// GetSession retrieves a session by ID
func (fs *FileStore) GetSession(sessionID string) (*models.XFile, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	session, ok := fs.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session.XFile, nil
}

// SessionExists checks if a session exists
func (fs *FileStore) SessionExists(sessionID string) bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	_, ok := fs.sessions[sessionID]
	return ok
}

// UpdateSession updates the XFile for a session
func (fs *FileStore) UpdateSession(sessionID string, xf *models.XFile) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	session, ok := fs.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	xf.Metadata.Modified = time.Now()
	session.XFile = xf
	session.UpdatedAt = time.Now()

	return fs.saveSession(sessionID)
}

// saveSession saves a session to disk (caller must hold lock)
func (fs *FileStore) saveSession(sessionID string) error {
	session, ok := fs.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	data, err := json.MarshalIndent(session.XFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal XFile: %w", err)
	}

	filePath := filepath.Join(fs.baseDir, sessionID+".json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// DeleteSession removes a session
func (fs *FileStore) DeleteSession(sessionID string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, ok := fs.sessions[sessionID]; !ok {
		return nil // Already deleted
	}

	delete(fs.sessions, sessionID)

	filePath := filepath.Join(fs.baseDir, sessionID+".json")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove session file: %w", err)
	}

	return nil
}

// Cleanup removes sessions older than maxAge
func (fs *FileStore) Cleanup() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	cutoff := time.Now().Add(-fs.maxAge)
	var toDelete []string

	for id, session := range fs.sessions {
		if session.UpdatedAt.Before(cutoff) {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		delete(fs.sessions, id)
		filePath := filepath.Join(fs.baseDir, id+".json")
		os.Remove(filePath) // Ignore errors during cleanup
	}

	if len(toDelete) > 0 {
		fmt.Printf("Cleaned up %d expired sessions\n", len(toDelete))
	}

	return nil
}
