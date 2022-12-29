package source

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

func TestSource(t *testing.T) {
	tests := []struct {
		name         string
		confSrc      config.Source
		data         config.SourceTmplData
		errGetSource error
		errGet       error
		errKey       error
		expectGet    string
		expectKey    string
	}{
		{
			name: "unknown type",
			confSrc: config.Source{
				Type: "unknown",
			},
			errGetSource: fmt.Errorf("source type not known: unknown"),
		},
		{
			name: "custom",
			confSrc: config.Source{
				Name: "custom",
				Type: "custom",
				Key:  "custom-test",
				Args: map[string]string{
					"cmd": "echo 1.2.3.4",
				},
			},
			data: config.SourceTmplData{
				Filename:  "/dev/null",
				ScanArgs:  map[string]string{},
				ScanMatch: map[string]string{},
				SourceArgs: map[string]string{
					"cmd": "echo 1.2.3.4",
				},
			},
			expectGet: "1.2.3.4",
			expectKey: "custom-test",
		},
		{
			name: "manual",
			confSrc: config.Source{
				Name: "manual",
				Type: "manual",
				Key:  "{{ .ScanArgs.Key }}",
				Args: map[string]string{},
			},
			data: config.SourceTmplData{
				Filename: "/dev/null",
				ScanArgs: map[string]string{
					"Key": "manual-test",
				},
				ScanMatch: map[string]string{
					"Version": "4.3.2.1",
				},
				SourceArgs: map[string]string{},
			},
			expectGet: "4.3.2.1",
			expectKey: "manual-test",
		},
		{
			name: "git tags",
			confSrc: config.Source{
				Name: "git tags",
				Type: "git",
				Key:  "git tag",
				Args: map[string]string{
					// TODO: switch to version-bump repo after it has enough tags
					"url":    "https://github.com/regclient/regclient.git",
					"type":   "tag",
					"tagExp": `^v0.4.[1-5]$`,
				},
				Sort: config.SourceSort{
					Method: "semver",
				},
				Template: `["{{ index .VerMap ( index .VerList 1 ) }}", "{{ index .VerMap ( index .VerList 0 ) }}"]`,
			},
			data: config.SourceTmplData{
				Filename:   "/dev/null",
				ScanArgs:   map[string]string{},
				ScanMatch:  map[string]string{},
				SourceArgs: map[string]string{},
			},
			expectGet: `["v0.4.4", "v0.4.5"]`,
			expectKey: "git tag",
		},
		{
			name: "git ref",
			confSrc: config.Source{
				Name: "git ref",
				Type: "git",
				Key:  "git ref",
				Args: map[string]string{
					// TODO: switch to version-bump repo after it has enough tags
					"url":  "https://github.com/regclient/regclient.git",
					"type": "ref",
					"ref":  `v0.4.3`,
				},
			},
			data: config.SourceTmplData{
				Filename:   "/dev/null",
				ScanArgs:   map[string]string{},
				ScanMatch:  map[string]string{},
				SourceArgs: map[string]string{},
			},
			expectGet: "6f5dc406130fdf939cc0f49fb0a5904b35a3c46f",
			expectKey: "git ref",
		},
		{
			name: "registry tags",
			confSrc: config.Source{
				Name: "registry tags",
				Type: "registry",
				Key:  "registry tag",
				Args: map[string]string{
					// TODO: switch to version-bump repo after it has enough tags
					"repo":   "ghcr.io/regclient/regctl",
					"type":   "tag",
					"tagExp": `^v0.4.[1-5]$`,
				},
				Sort: config.SourceSort{
					Method: "semver",
					Asc:    true, // reverse the sort
				},
				Template: `["{{ index .VerMap ( index .VerList 1 ) }}", "{{ index .VerMap ( index .VerList 0 ) }}"]`,
			},
			data: config.SourceTmplData{
				Filename:   "/dev/null",
				ScanArgs:   map[string]string{},
				ScanMatch:  map[string]string{},
				SourceArgs: map[string]string{},
			},
			expectGet: `["v0.4.2", "v0.4.1"]`,
			expectKey: "registry tag",
		},
		{
			name: "registry digest",
			confSrc: config.Source{
				Name: "registry digest",
				Type: "registry",
				Key:  "registry digest",
				Args: map[string]string{
					// TODO: switch to version-bump repo after it has enough tags
					"image": "ghcr.io/regclient/regctl:v0.4.3",
					"type":  "digest",
				},
			},
			data: config.SourceTmplData{
				Filename:   "/dev/null",
				ScanArgs:   map[string]string{},
				ScanMatch:  map[string]string{},
				SourceArgs: map[string]string{},
			},
			expectGet: "sha256:b76626b3eb7e2380183b29f550bea56dea67685907d4ec61b56ff770ae2d7138",
			expectKey: "registry digest",
		},
		// TODO: numeric sort
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := Get(tt.confSrc)
			if tt.errGetSource != nil {
				if err == nil {
					t.Errorf("get source did not fail")
				} else if !errors.Is(err, tt.errGetSource) && err.Error() != tt.errGetSource.Error() {
					t.Errorf("unexpected error, expected %v, received %v", tt.errGetSource, err)
				}
				return
			} else if err != nil {
				t.Errorf("get source failed: %v", err)
				return
			}
			getStr, err := src.Get(tt.data)
			if tt.errGet != nil {
				if err == nil {
					t.Errorf("get did not fail")
				} else if !errors.Is(err, tt.errGet) && err.Error() != tt.errGet.Error() {
					t.Errorf("unexpected error, expected %v, received %v", tt.errGet, err)
				}
				return
			} else if err != nil {
				t.Errorf("get failed: %v", err)
				return
			} else if tt.expectGet != getStr {
				t.Errorf("get unexpected response, expected %s, received %s", tt.expectGet, getStr)
			}
			keyStr, err := src.Key(tt.data)
			if tt.errKey != nil {
				if err == nil {
					t.Errorf("key did not fail")
				} else if !errors.Is(err, tt.errKey) && err.Error() != tt.errKey.Error() {
					t.Errorf("unexpected error, expected %v, received %v", tt.errKey, err)
				}
				return
			} else if err != nil {
				t.Errorf("key failed: %v", err)
				return
			} else if tt.expectKey != keyStr {
				t.Errorf("key unexpected response, expected %s, received %s", tt.expectKey, keyStr)
			}
		})
	}

}
