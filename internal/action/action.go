// Package action processes the result of a scanner match, the source, and the
// configuration to take an action (log, modify the version).
package action

import (
	"fmt"
	"os"

	"github.com/sudo-bmitch/version-bump/internal/config"
	"github.com/sudo-bmitch/version-bump/internal/source"
)

type Action struct {
	run     config.Run
	conf    config.Config
	sources map[string]source.Source
	// TODO: add logging or output
}

func New(run config.Run, conf config.Config) *Action {
	return &Action{
		run:     run,
		conf:    conf,
		sources: map[string]source.Source{},
	}
}

// HandleMatch processes a scan result, checking the sources and config, and returning the resulting action
// Output:
// - change bool: should the scan modify the version
// - version string: version the scan should use
// - err error: not nil on any failure
func (a *Action) HandleMatch(filename string, scan string, sourceName string, version string, data interface{}) (bool, string, error) {
	// check with source for the version
	if _, ok := a.conf.Sources[sourceName]; !ok {
		return false, "", fmt.Errorf("source not found: %s", sourceName)
	}
	s, err := source.Get(*a.conf.Sources[sourceName])
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not get the source: %v\n", err)
		return false, "", err
	}
	curVer, err := s.Get(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not get the current version: %v\n", err)
		return false, "", err
	}
	// TODO: fix all kinds of things:
	// - track/output matches
	// - manage lock file
	// - only get version when action isn't set to use lock file only
	// - only return true when not in check or dry run modes
	if version != curVer {
		fmt.Printf("Version changed: filename=%s, source=%s, scan=%s, old=%s, new=%s\n", filename, sourceName, scan, version, curVer)
		return true, curVer, nil
	}
	return false, version, nil
}
