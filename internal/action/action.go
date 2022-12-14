// Package action processes the result of a scanner match, the source, and the
// configuration to take an action (log, modify the version).
package action

import (
	"fmt"

	"github.com/sudo-bmitch/version-bump/internal/config"
	"github.com/sudo-bmitch/version-bump/internal/lockfile"
	"github.com/sudo-bmitch/version-bump/internal/source"
)

// Opts specifies runtime configuration inputs and outputs
type Opts struct {
	Action  runAction       // which action to run
	DryRun  bool            // when set, lock file and scanned files are unchanged
	Locks   *lockfile.Locks // lock entries to use or set
	Changes []*Change       // results of the run
}

type runAction int

const (
	ActionScan   runAction = iota // scan: search files for versions and saves to lock
	ActionCheck                   // check: scans for versions and compares to source
	ActionSet                     // set: updates a version without checking the source
	ActionUpdate                  // update: modifies versions using sources
	ActionReset                   // reset: sets versions to the lock value without checking source
)

// Change lists changes found or made to scanned files
type Change struct {
	Filename string // filename modified
	Source   string // name of the source
	Scan     string // name of the scan
	Key      string // key of the scan
	Orig     string // previous version
	New      string // new version
}

type Action struct {
	opts *Opts
	conf config.Config
}

func New(opts *Opts, conf config.Config) *Action {
	return &Action{
		opts: opts,
		conf: conf,
	}
}

// Done should be called after all HandleMatch calls are finished.
// It will perform any final steps.
func (a *Action) Done() error {
	// TODO: is this needed?
	return nil
}

// HandleMatch processes a scan result, checking the sources and config, and returning the resulting action
// Output:
// - change bool: should the scan modify the version
// - version string: version the scan should use
// - err error: not nil on any failure
func (a *Action) HandleMatch(filename string, scan string, sourceName string, version string, data config.SourceTmplData) (bool, string, error) {
	if _, ok := a.conf.Sources[sourceName]; !ok {
		return false, "", fmt.Errorf("source not found: %s", sourceName)
	}
	s, err := source.Get(*a.conf.Sources[sourceName])
	if err != nil {
		return false, "", fmt.Errorf("could not get source %s: %w", sourceName, err)
	}
	data.SourceArgs = a.conf.Sources[sourceName].Args
	key, err := s.Key(data)
	if err != nil {
		return false, "", fmt.Errorf("could not get key for source %s: %w", sourceName, err)
	}
	// determine curVer
	var curVer string
	switch a.opts.Action {
	case ActionCheck, ActionUpdate:
		// query from source
		curVer, err = s.Get(data)
		if err != nil {
			return false, "", fmt.Errorf("could not get current version from source %s: %w", sourceName, err)
		}
	case ActionSet, ActionReset:
		// TODO: get curVer from lock, requires getting the key from the source

	case ActionScan:
		// scan doesn't change the version, set from file contents
		curVer = version
	}

	// store any changes when version != curVer
	if version != curVer {
		a.opts.Changes = append(a.opts.Changes, &Change{
			Filename: filename,
			Source:   sourceName,
			Scan:     scan,
			Key:      key,
			Orig:     version,
			New:      curVer,
		})
	}

	// for dry-run, never return a change
	if a.opts.DryRun || a.opts.Action == ActionCheck {
		return false, version, nil
	}

	// update lock file
	switch a.opts.Action {
	case ActionScan, ActionUpdate:
		err = a.opts.Locks.Set(sourceName, key, curVer)
		if err != nil {
			return false, "", fmt.Errorf("could not set lock for %s/%s: %w", sourceName, key, err)
		}
	}
	return version != curVer, curVer, nil
}
