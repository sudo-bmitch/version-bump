package scan

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

func getVer10(curVer string, args map[string]string) (string, error) {
	return "10", nil
}

func TestRegexp(t *testing.T) {
	ctx := context.Background()

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
				Name: "test",
				Type: "regexp",
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
			r := bytes.NewReader(tt.in)
			outBuf := bytes.NewBuffer([]byte{})
			err := runREScan(ctx, tt.confScan, "test", r, outBuf, getVer10)
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
			out := outBuf.Bytes()
			if !bytes.Equal(tt.expOut, out) {
				t.Errorf("result does not match:\n--- expected ---\n%s\n--- received ---\n%s", tt.expOut, out)
			}
		})
	}
}
