// Package source is used to fetch the latest version information from upstream
package source

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/sudo-bmitch/version-bump/internal/config"
	"github.com/sudo-bmitch/version-bump/internal/template"
)

type Source interface {
	// Get returns the version from upstream
	Get(data config.SourceTmplData) (string, error)
	Key(data config.SourceTmplData) (string, error)
}

var sourceTypes map[string]func(config.Source) Source = map[string]func(config.Source) Source{
	"custom":   newCustom,
	"git":      newGit,
	"manual":   newManual,
	"registry": newRegistry,
	// TODO: add url (headers, parse json/yaml, parse regex), github release
}

var mu sync.Mutex
var sourceCache map[string]Source = map[string]Source{}

// Get a named source
func Get(confSrc config.Source) (Source, error) {
	mu.Lock()
	defer mu.Unlock()
	if s, ok := sourceCache[confSrc.Name]; ok {
		return s, nil
	}
	if newFn, ok := sourceTypes[confSrc.Type]; ok {
		s := newFn(confSrc)
		sourceCache[confSrc.Name] = s
		return s, nil
	}
	return nil, fmt.Errorf("source type not known: %s", confSrc.Type)
}

func procResult(confExp config.Source, data config.VersionTmplData) (string, error) {
	// TODO: filter, move regexp filtering here instead of per implementation
	// sort
	if len(data.VerMap) > 0 {
		keys := make([]string, 0, len(data.VerMap))
		for k := range data.VerMap {
			keys = append(keys, k)
		}
		switch confExp.Sort.Method {
		case "semver":
			vers := make([]*semver.Version, 0, len(keys))
			for _, k := range keys {
				sv, err := semver.NewVersion(k)
				if err != nil {
					continue // ignore versions that do not compile
				}
				vers = append(vers, sv)
			}
			if len(vers) == 0 {
				return "", fmt.Errorf("no valid semver versions found in %v", keys)
			}
			if confExp.Sort.Asc {
				sort.Sort(semver.Collection(vers))
			} else {
				sort.Sort(sort.Reverse(semver.Collection(vers)))
			}
			// rebuild keys from parsed semver
			keys = make([]string, len(vers))
			for i, sv := range vers {
				keys[i] = sv.Original()
			}
		case "numeric":
			keyInts := make([]int, 0, len(data.VerMap))
			orig := map[int]string{} // map from int back to original value
			for _, k := range keys {
				// parse numbers from keys
				i, err := strconv.Atoi(k)
				if err != nil {
					continue // ignore versions that are not numeric
				}
				keyInts = append(keyInts, i)
				orig[i] = k
			}
			if len(keyInts) == 0 {
				return "", fmt.Errorf("no valid numeric versions found in %v", keys)
			}
			if confExp.Sort.Asc {
				sort.Sort(sort.IntSlice(keyInts))
			} else {
				sort.Sort(sort.Reverse(sort.IntSlice(keyInts)))
			}
			// rebuild keys from parsed semver
			keys = make([]string, len(keyInts))
			for i, iv := range keyInts {
				keys[i] = orig[iv]
			}
		default:
			if confExp.Sort.Asc {
				sort.Sort(sort.StringSlice(keys))
			} else {
				sort.Sort(sort.Reverse(sort.StringSlice(keys)))
			}
		}
		// select the requested offset
		if len(keys) <= int(confExp.Sort.Offset) {
			return "", fmt.Errorf("requested offset is too large, %d matching versions found: %v", len(keys), keys)
		}
		data.Version = data.VerMap[keys[confExp.Sort.Offset]]
		data.VerList = keys
	}
	// template
	if confExp.Template != "" {
		return template.String(confExp.Template, data)
	}
	return data.Version, nil
}
