// Package cache provides a persistent, TTL-aware metadata cache stored on disk.
package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultTTL is the default cache entry lifetime (7 days).
const DefaultTTL = 7 * 24 * time.Hour

// PartialTTL is the TTL for incomplete/partial cache entries (1 day).
const PartialTTL = 24 * time.Hour

// Entry wraps cached data with a timestamp for TTL checking.
type Entry struct {
	FetchedAt time.Time       `json:"fetched_at"`
	Data      json.RawMessage `json:"data"`
}

// PartialEntry represents an incomplete cache with pagination state.
type PartialEntry struct {
	FetchedAt  time.Time       `json:"fetched_at"`
	NextCursor string          `json:"next_cursor"`
	Complete   bool            `json:"complete"`
	Count      int             `json:"count"`
	Data       json.RawMessage `json:"data"`
}

// Store manages cache files under a base directory.
type Store struct {
	BasePath string
	TTL      time.Duration
	// Clock allows injecting a custom time source for testing.
	Clock func() time.Time
}

// New creates a Store rooted at basePath with the given TTL.
// If ttl is zero, DefaultTTL is used.
func New(basePath string, ttl time.Duration) *Store {
	if ttl == 0 {
		ttl = DefaultTTL
	}
	return &Store{
		BasePath: basePath,
		TTL:      ttl,
		Clock:    time.Now,
	}
}

// DefaultStore returns a Store using the standard cache directory (~/.config/slack-cli/cache).
func DefaultStore() (*Store, error) {
	base, err := defaultBasePath()
	if err != nil {
		return nil, err
	}
	return New(base, DefaultTTL), nil
}

// Load reads a cached entry by key and unmarshals it into v.
// Returns true if found and still valid, false otherwise.
// If the entry is expired or missing, v is left unchanged.
func (s *Store) Load(key string, v interface{}) (bool, error) {
	path := s.filePath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read cache %s: %w", key, err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		// Corrupted entry; treat as miss and remove
		_ = os.Remove(path)
		return false, nil
	}

	now := s.now()
	if now.Sub(entry.FetchedAt) > s.TTL {
		// Expired; treat as miss
		return false, nil
	}

	if err := json.Unmarshal(entry.Data, v); err != nil {
		return false, fmt.Errorf("unmarshal cache data %s: %w", key, err)
	}
	return true, nil
}

// Save writes v to the cache under key using atomic write (temp + rename).
func (s *Store) Save(key string, v interface{}) error {
	if err := os.MkdirAll(s.BasePath, 0o700); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal cache data: %w", err)
	}

	entry := Entry{
		FetchedAt: s.now(),
		Data:      payload,
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache entry: %w", err)
	}

	path := s.filePath(key)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write cache tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename cache tmp: %w", err)
	}
	return nil
}

// Expire removes the cache file for the given key.
func (s *Store) Expire(key string) error {
	path := s.filePath(key)
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("expire cache %s: %w", key, err)
	}
	return nil
}

// ExpireAll removes all cache files matching the given prefix.
func (s *Store) ExpireAll(prefix string) error {
	entries, err := os.ReadDir(s.BasePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read cache dir: %w", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".json") {
			_ = os.Remove(filepath.Join(s.BasePath, e.Name()))
		}
	}
	return nil
}

func (s *Store) filePath(key string) string {
	return filepath.Join(s.BasePath, key+".json")
}

func (s *Store) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}

func defaultBasePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "slack-cli", "cache"), nil
}

// PartialState represents the current state of a partial cache.
type PartialState struct {
	FetchedAt  time.Time
	NextCursor string
	Complete   bool
	Count      int
}

// LoadPartial reads a partial cache entry and unmarshals data into v.
// Returns the pagination state and whether valid data was found.
func (s *Store) LoadPartial(key string, v interface{}) (PartialState, bool, error) {
	path := s.filePath(key + "_partial")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return PartialState{}, false, nil
		}
		return PartialState{}, false, fmt.Errorf("read partial cache %s: %w", key, err)
	}

	var entry PartialEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		_ = os.Remove(path)
		return PartialState{}, false, nil
	}

	// Partial entries expire faster (1 day)
	now := s.now()
	if now.Sub(entry.FetchedAt) > PartialTTL {
		_ = os.Remove(path)
		return PartialState{}, false, nil
	}

	if err := json.Unmarshal(entry.Data, v); err != nil {
		return PartialState{}, false, fmt.Errorf("unmarshal partial cache data %s: %w", key, err)
	}

	state := PartialState{
		FetchedAt:  entry.FetchedAt,
		NextCursor: entry.NextCursor,
		Complete:   entry.Complete,
		Count:      entry.Count,
	}
	return state, true, nil
}

// SavePartial writes a partial cache entry with pagination state.
func (s *Store) SavePartial(key string, v interface{}, cursor string, complete bool, count int) error {
	if err := os.MkdirAll(s.BasePath, 0o700); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal partial cache data: %w", err)
	}

	entry := PartialEntry{
		FetchedAt:  s.now(),
		NextCursor: cursor,
		Complete:   complete,
		Count:      count,
		Data:       payload,
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal partial cache entry: %w", err)
	}

	path := s.filePath(key + "_partial")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write partial cache tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename partial cache tmp: %w", err)
	}
	return nil
}

// PromotePartial moves a complete partial cache to the main cache.
func (s *Store) PromotePartial(key string, v interface{}) error {
	if err := s.Save(key, v); err != nil {
		return err
	}
	// Remove partial file
	_ = s.Expire(key + "_partial")
	return nil
}

// ExpirePartial removes the partial cache file for the given key.
func (s *Store) ExpirePartial(key string) error {
	return s.Expire(key + "_partial")
}

// Status returns information about cache entries for a given key.
type CacheStatus struct {
	Key        string
	Complete   bool
	Count      int
	FetchedAt  time.Time
	NextCursor string
	Expired    bool
}

// GetStatus returns the status of a cache key (checks both complete and partial).
func (s *Store) GetStatus(key string) (CacheStatus, bool) {
	// Check complete cache first
	path := s.filePath(key)
	if data, err := os.ReadFile(path); err == nil {
		var entry Entry
		if json.Unmarshal(data, &entry) == nil {
			expired := s.now().Sub(entry.FetchedAt) > s.TTL
			var items []json.RawMessage
			_ = json.Unmarshal(entry.Data, &items)
			return CacheStatus{
				Key:       key,
				Complete:  true,
				Count:     len(items),
				FetchedAt: entry.FetchedAt,
				Expired:   expired,
			}, true
		}
	}

	// Check partial cache
	partialPath := s.filePath(key + "_partial")
	if data, err := os.ReadFile(partialPath); err == nil {
		var entry PartialEntry
		if json.Unmarshal(data, &entry) == nil {
			expired := s.now().Sub(entry.FetchedAt) > PartialTTL
			return CacheStatus{
				Key:        key,
				Complete:   entry.Complete,
				Count:      entry.Count,
				FetchedAt:  entry.FetchedAt,
				NextCursor: entry.NextCursor,
				Expired:    expired,
			}, true
		}
	}

	return CacheStatus{Key: key}, false
}
