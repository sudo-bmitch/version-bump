// Package source is used to fetch the latest version information from upstream
package source

import (
	"fmt"
	"sync"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

type Source interface {
	// Get returns the version from upstream
	Get(data config.TemplateData) (string, error)
	Key(data config.TemplateData) (string, error)
}

var sourceTypes map[string]func(config.Source) Source = map[string]func(config.Source) Source{
	"custom": newCustom,
	"manual": newManual,
	// TODO: add url (headers, parse json, parse regex), docker tag, git tag, git release, git commit
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
