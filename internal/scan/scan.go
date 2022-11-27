// Package scan parses content for version data from a file (or ReadCloser)
package scan

import (
	"fmt"
	"io"

	"github.com/sudo-bmitch/version-bump/internal/action"
	"github.com/sudo-bmitch/version-bump/internal/config"
)

// - input is a reader, returns a reader
// - when read is called, load(read) enough to scan the next chunk
// - scan is provided the configuration (options) and mode (scan, update, dry-run)
// - on update, call the source with the scan match to check the version, and modify buffer before returning
// - always track each match and update state in a lock file

type Scan interface {
	io.ReadCloser
}

type newScan func(config.Scan, io.ReadCloser, *action.Action) (Scan, error)

var scanTypes map[string]newScan = map[string]newScan{
	"regexp": newREScan,
}

// New creates a new scan of a given type
func New(conf config.Scan, rc io.ReadCloser, a *action.Action) (Scan, error) {
	if s, ok := scanTypes[conf.Type]; ok {
		return s(conf, rc, a)
	}
	return nil, fmt.Errorf("scan type not known: %s", conf.Type)
}
