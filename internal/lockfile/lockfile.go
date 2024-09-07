// Package lockfile is used to manage the lockfile of managed versions
package lockfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"golang.org/x/exp/maps"
)

// Lock stores known versions from a scan or source
type Lock struct {
	Name    string `json:"name"`    // name for a group of locks, e.g. git versions
	Key     string `json:"key"`     // key for a specific lock, e.g. repo and branch
	Version string `json:"version"` // version of the lock, e.g. commit hash
	used    bool   // tracks if a lock was used
}

type Locks struct {
	mu       sync.Mutex
	Filename string
	Lock     map[string]map[string]*Lock // Lock[Name][Key] = *Lock
}

func New() *Locks {
	return &Locks{
		Lock: map[string]map[string]*Lock{},
	}
}

func (l *Locks) Get(name, key string) (*Lock, error) {
	if l == nil || l.Lock == nil {
		return nil, fmt.Errorf("cannot Get from a nil pointer")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.Lock[name]; !ok {
		return nil, fmt.Errorf("not found")
	}
	entry, ok := l.Lock[name][key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	entry.used = true
	return entry, nil
}

func (l *Locks) Set(name, key, version string) error {
	if l == nil || l.Lock == nil {
		return fmt.Errorf("cannot Set to a nil pointer")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.Lock[name]; !ok {
		l.Lock[name] = map[string]*Lock{}
	}
	l.Lock[name][key] = &Lock{
		Name:    name,
		Key:     key,
		Version: version,
		used:    true,
	}
	return nil
}

func LoadReader(rdr io.Reader) (*Locks, error) {
	decode := json.NewDecoder(rdr)
	l := New()
	var err error
	for {
		entry := &Lock{}
		err = decode.Decode(&entry)
		if err != nil {
			break
		}
		if _, ok := l.Lock[entry.Name]; !ok {
			l.Lock[entry.Name] = map[string]*Lock{}
		}
		l.Lock[entry.Name][entry.Key] = entry
	}
	if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	return l, nil
}

func LoadFile(filename string) (*Locks, error) {
	fh, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file %s: %w", filename, err)
	}
	defer fh.Close()
	return LoadReader(fh)
}

func (l *Locks) Save(used bool) error {
	if l == nil || l.Lock == nil {
		return fmt.Errorf("cannot save nil locks")
	}
	return l.SaveFile(l.Filename, used)
}

// SaveWriter outputs the locks to the writer.
// If used is true, only the locks that were marked as used (with a Get or Set) are output.
func (l *Locks) SaveWriter(write io.Writer, used bool) error {
	if l == nil || l.Lock == nil {
		return fmt.Errorf("cannot save nil locks")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	// sort to keep the file deterministic
	names := maps.Keys(l.Lock)
	sort.Strings(names)
	for _, name := range names {
		keys := maps.Keys(l.Lock[name])
		sort.Strings(keys)
		for _, key := range keys {
			if used && !l.Lock[name][key].used {
				continue
			}
			if err := json.NewEncoder(write).Encode(l.Lock[name][key]); err != nil {
				return fmt.Errorf("failed to encode lockfile content: %w", err)
			}
		}
	}
	return nil
}

func (l *Locks) SaveFile(filename string, used bool) error {
	if l == nil || l.Lock == nil {
		return fmt.Errorf("cannot save nil locks")
	}
	// write to a temp file
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(filename))
	if err != nil {
		return fmt.Errorf("unable to create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	err = l.SaveWriter(tmp, used)
	tmp.Close()
	defer func() {
		if err != nil {
			os.Remove(tmpName)
		}
	}()
	if err != nil {
		return fmt.Errorf("failed to save lock file %s: %w", tmpName, err)
	}
	// update permissions to match existing file or 0644
	mode := os.FileMode(0644)
	stat, err := os.Stat(filename)
	if err == nil && stat.Mode().IsRegular() {
		mode = stat.Mode()
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("failed to change permission on lockfile %s: %w", tmpName, err)
	}
	// move temp file to target filename
	if err := os.Rename(tmpName, filename); err != nil {
		return fmt.Errorf("failed to replace lockfile %s: %w", filename, err)
	}
	return nil
}
