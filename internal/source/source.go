// Package source is used to fetch the latest version information from upstream
package source

import "fmt"

type Source interface {
	// Get returns the version from upstream
	// TODO: change input to the config for source and the scan match
	Get(args map[string]string) (string, error)
}

var sourceTypes map[string]Source = map[string]Source{
	"custom": custom{},
	// TODO: add url (headers, parse json, parse regex), docker tag, git tag, git release, git commit
}

// Get a named source
func Get(t string) (Source, error) {
	if s, ok := sourceTypes[t]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("source type not known: %s", t)
}
