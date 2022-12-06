package scan

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/sudo-bmitch/version-bump/internal/action"
	"github.com/sudo-bmitch/version-bump/internal/config"
	"github.com/sudo-bmitch/version-bump/internal/lockfile"
)

var confBytes = []byte(`
files:
  "**/*.sh":
    scans:
      - test
scans:
  "test":
    type: "regexp"
    source: "test10"
    args:
      regexp: "^testVer=(?P<Version>\\d+)"
sources:
  "test10":
    type: "custom"
    args:
      cmd: "echo 10"
`)

func TestRegexp(t *testing.T) {
	conf, err := config.LoadReader(bytes.NewReader(confBytes))
	if err != nil {
		t.Errorf("failed to load config: %v", err)
		return
	}
	a := action.New(&action.Opts{
		Action: action.ActionUpdate,
		DryRun: false,
		Locks:  lockfile.New(),
	}, *conf)

	tests := []struct {
		name     string
		confScan config.Scan
		in       []byte
		expError error
		expOut   []byte
	}{
		{
			name: "Replace version",
			confScan: config.Scan{
				Name:   "test",
				Type:   "regexp",
				Source: "test10",
				Args: map[string]string{
					"regexp": "^testVer=(?P<Version>\\d+)",
				},
			},
			in:     []byte("testVer=42"),
			expOut: []byte("testVer=10"),
		},
		// TODO: test failing exp, multi-line, version at start of regexp, content after version
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdr := bytes.NewReader(tt.in)
			scan, err := newREScan(tt.confScan, io.NopCloser(rdr), a, "test")
			if tt.expError != nil {
				if err == nil {
					t.Errorf("newREScan did not fail")
				} else if !errors.Is(err, tt.expError) && err.Error() != tt.expError.Error() {
					t.Errorf("newREScan unexpected error, expected %v, received %v", tt.expError, err)
				}
				return
			} else if err != nil {
				t.Errorf("newREScan failed: %v", err)
				return
			}
			out, err := io.ReadAll(scan)
			if err != nil {
				t.Errorf("failed to read from scan: %v", err)
				return
			}
			if !bytes.Equal(tt.expOut, out) {
				t.Errorf("result does not match:\n--- expected ---\n%s\n--- received ---\n%s", tt.expOut, out)
			}
		})
	}
}
