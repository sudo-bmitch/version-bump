package source

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

func TestSource(t *testing.T) {
	tests := []struct {
		name       string
		conf       config.Source
		err        error
		expect     Results
		exactMatch bool
	}{
		{
			name: "unknown type",
			conf: config.Source{
				Type: "unknown",
			},
			err: fmt.Errorf("source type not found: unknown"),
		},
		{
			name: "custom",
			conf: config.Source{
				Name: "custom",
				Type: "custom",
				Args: map[string]string{
					"cmd": "echo 1.2.3.4",
				},
			},
			expect: Results{
				VerMap: map[string]string{
					"1.2.3.4": "1.2.3.4",
				},
			},
			exactMatch: true,
		},
		{
			name: "manual",
			conf: config.Source{
				Name: "manual",
				Type: "manual",
				Args: map[string]string{
					"Version": "4.3.2.1",
				},
			},
			expect: Results{
				VerMap: map[string]string{
					"4.3.2.1": "4.3.2.1",
				},
			},
			exactMatch: true,
		},
		{
			name: "git tags",
			conf: config.Source{
				Name: "git tags",
				Type: "git",
				Args: map[string]string{
					// TODO: switch to version-bump repo after it has enough tags
					"url":  "https://github.com/regclient/regclient.git",
					"type": "tag",
				},
			},
			expect: Results{
				VerMap: map[string]string{
					"v0.0.1": "v0.0.1",
					"v0.1.0": "v0.1.0",
					"v0.3.0": "v0.3.0",
					"v0.4.0": "v0.4.0",
					"v0.4.1": "v0.4.1",
				},
			},
			exactMatch: false, // only a partial list of expected results
		},
		{
			name: "git ref",
			conf: config.Source{
				Name: "git ref",
				Type: "git",
				Args: map[string]string{
					// TODO: switch to version-bump repo after it has enough tags
					"url":  "https://github.com/regclient/regclient.git",
					"type": "ref",
				},
			},
			expect: Results{
				VerMap: map[string]string{
					"v0.4.0": "9546658ede6901191b9692a7f720c37150940ddd",
					"v0.4.1": "4442cd773c348d7d5e6bd2b9a0cb58e2bce81d67",
					"v0.4.2": "c8125cd51a02bbff6a002f77eb8458b7a7753b63",
					"v0.4.3": "b0ac3e9413b1079c8b14df5c201a2a2129d9d9e1",
				},
			},
			exactMatch: false, // only a partial list of expected results
		},
		{
			name: "registry tags",
			conf: config.Source{
				Name: "registry tags",
				Type: "registry",
				Args: map[string]string{
					// TODO: switch to version-bump repo after it has enough tags
					"repo": "ghcr.io/regclient/regctl",
					"type": "tag",
				},
			},
			expect: Results{
				VerMap: map[string]string{
					"v0.4.0": "v0.4.0",
					"v0.4.1": "v0.4.1",
					"v0.4.2": "v0.4.2",
					"v0.4.3": "v0.4.3",
					"v0.4.4": "v0.4.4",
					"v0.4.5": "v0.4.5",
				},
			},
			exactMatch: false, // only a partial list of expected results
		},
		{
			name: "registry digest",
			conf: config.Source{
				Name: "registry digest",
				Type: "registry",
				Args: map[string]string{
					// TODO: switch to version-bump repo after it has enough tags
					"image": "ghcr.io/regclient/regctl:v0.4.3",
					"type":  "digest",
				},
			},
			expect: Results{
				VerMap: map[string]string{
					"sha256:b76626b3eb7e2380183b29f550bea56dea67685907d4ec61b56ff770ae2d7138": "sha256:b76626b3eb7e2380183b29f550bea56dea67685907d4ec61b56ff770ae2d7138",
				},
			},
			exactMatch: true,
		},
		{
			name: "github release version",
			conf: config.Source{
				Name: "github release",
				Type: "gh-release",
				Args: map[string]string{
					// TODO: switch to version-bump repo after it has enough tags
					"repo": "regclient/regclient",
				},
			},
			expect: Results{
				VerMap: map[string]string{
					"v0.4.0": "v0.4.0",
					"v0.4.1": "v0.4.1",
					"v0.4.2": "v0.4.2",
					"v0.4.3": "v0.4.3",
					"v0.4.4": "v0.4.4",
					"v0.4.5": "v0.4.5",
				},
			},
			exactMatch: false, // only a partial list of expected results
		},
		{
			name: "github release artifact",
			conf: config.Source{
				Name: "github artifact",
				Type: "gh-release",
				Args: map[string]string{
					// TODO: switch to version-bump repo after it has enough tags
					"repo":     "regclient/regclient",
					"type":     "artifact",
					"artifact": "regctl-linux-amd64",
				},
			},
			expect: Results{
				VerMap: map[string]string{
					"v0.4.0": `https://github.com/regclient/regclient/releases/download/v0.4.0/regctl-linux-amd64`,
					"v0.4.1": `https://github.com/regclient/regclient/releases/download/v0.4.1/regctl-linux-amd64`,
					"v0.4.5": `https://github.com/regclient/regclient/releases/download/v0.4.5/regctl-linux-amd64`,
				},
			},
			exactMatch: false, // only a partial list of expected results
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := Get(tt.conf)
			if tt.err != nil {
				if err == nil {
					t.Errorf("get source did not fail")
				} else if !errors.Is(err, tt.err) && err.Error() != tt.err.Error() {
					t.Errorf("unexpected error, expected %v, received %v", tt.err, err)
				}
				return
			} else if err != nil {
				t.Errorf("get source failed: %v", err)
				return
			}
			if tt.expect.VerMap != nil && results.VerMap == nil {
				t.Errorf("results.VerMap is nil")
			} else {
				for k, v := range tt.expect.VerMap {
					if results.VerMap[k] != v {
						t.Errorf("results.VerMap[%s] expect %s, received %s", k, v, results.VerMap[k])
					}
				}
			}
			if tt.exactMatch && len(tt.expect.VerMap) != len(results.VerMap) {
				t.Errorf("results.VerMap is not an exact match: %v != %v", tt.expect.VerMap, results.VerMap)
			}
			if tt.expect.VerMeta != nil && results.VerMeta == nil {
				t.Errorf("results.VerMeta is nil")
			} else {
				for k, v := range tt.expect.VerMeta {
					if results.VerMeta[k] != v {
						t.Errorf("results.VerMeta[%s] expect %s, received %s", k, v, results.VerMeta[k])
					}
				}
			}
			if tt.exactMatch && len(tt.expect.VerMeta) != len(results.VerMeta) {
				t.Errorf("results.VerMeta is not an exact match: %v != %v", tt.expect.VerMeta, results.VerMeta)
			}
		})
	}

}
