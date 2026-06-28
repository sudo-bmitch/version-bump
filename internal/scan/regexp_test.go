// Copyright the version-bump contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
