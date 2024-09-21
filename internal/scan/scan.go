// Package scan parses content for version data from a file (or ReadCloser)
package scan

import (
	"context"
	"fmt"
	"io"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

// - input is a reader, returns a reader
// - when read is called, load(read) enough to scan the next chunk
// - scan is provided the configuration (options) and mode (scan, update, dry-run)
// - on update, call the source with the scan match to check the version, and modify buffer before returning
// - always track each match and update state in a lock file

type Scan interface {
	Scan(ctx context.Context, filename string, r io.Reader, w io.Writer, getVer func(curVer string, args map[string]string) string) error
}

type runScan func(ctx context.Context, conf config.Scan, filename string, r io.Reader, w io.Writer, getVer func(curVer string, args map[string]string) (string, error)) error

var scanTypes map[string]runScan = map[string]runScan{
	"regexp": runREScan,
}

// Run executes the selected scanner.
func Run(ctx context.Context, conf config.Scan, filename string, r io.Reader, w io.Writer, getVer func(curVer string, args map[string]string) (string, error)) error {
	if rs, ok := scanTypes[conf.Type]; ok {
		return rs(ctx, conf, filename, r, w, getVer)
	}
	return fmt.Errorf("scan type not known: %s", conf.Type)
}
