package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CacheVersion of plugins.json; another version is an empty cache.
const CacheVersion = 1

// Cache keeps each plugin's spec, keyed by its path, while the executable
// keeps its size and modification time: the CLI then starts without
// running every plugin. It lives in <user cache dir>/devtooltui/plugins.json.
type Cache struct {
	Version int                   `json:"version"`
	Entries map[string]cacheEntry `json:"entries"`
	changed bool
}

type cacheEntry struct {
	stamp
	Spec SpecMessage `json:"spec"`
}

type stamp struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

func stampOf(path string) stamp {
	info, err := os.Stat(path)
	if err != nil {
		return stamp{}
	}
	return stamp{Size: info.Size(), ModTime: info.ModTime().UTC()}
}

// CachePath returns <user cache dir>/devtooltui/plugins.json.
func CachePath(userCacheDir func() (string, error)) (string, error) {
	dir, err := userCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "devtooltui", "plugins.json"), nil
}

// LoadCache reads the cache; a missing, broken or old file is empty.
func LoadCache(path string) *Cache {
	c := &Cache{Version: CacheVersion, Entries: map[string]cacheEntry{}}
	data, err := os.ReadFile(path) //nolint:gosec // our own cache file
	if err != nil {
		return c
	}
	var disk Cache
	if json.Unmarshal(data, &disk) == nil && disk.Version == CacheVersion && disk.Entries != nil {
		c.Entries = disk.Entries
	}
	return c
}

func (c *Cache) get(path string, s stamp) (SpecMessage, bool) {
	if c == nil || s == (stamp{}) {
		return SpecMessage{}, false
	}
	e, ok := c.Entries[path]
	if !ok || e.Size != s.Size || !e.ModTime.Equal(s.ModTime) {
		return SpecMessage{}, false
	}
	return e.Spec, true
}

func (c *Cache) put(path string, s stamp, msg SpecMessage) {
	if c == nil || s == (stamp{}) {
		return
	}
	if e, ok := c.Entries[path]; ok && e.stamp == s {
		return
	}
	c.Entries[path] = cacheEntry{stamp: s, Spec: msg}
	c.changed = true
}

// keep drops the entries of executables that are no longer in the PATH.
func (c *Cache) keep(cands []Candidate) {
	if c == nil {
		return
	}
	live := map[string]bool{}
	for _, cand := range cands {
		live[cand.Path] = true
	}
	for path := range c.Entries {
		if !live[path] {
			delete(c.Entries, path)
			c.changed = true
		}
	}
}

// Save writes the cache atomically with mode 0600.
func (c *Cache) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".plugins-*.json") // created with mode 0600
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	c.changed = false
	return nil
}
