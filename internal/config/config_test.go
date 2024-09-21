package config

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestConfig(t *testing.T) {
	cNew := New()
	// caution, byte contents must be space indented
	oldBytesManual := []byte(`
files:
  "root-*.txt":
    scans:
      - root-manual

scans:
  "root-manual":
    type: "regexp"
    source: "root-manual"
    args:
      regexp: '^manual-ver=(?P<Version>[^\s]+)\s*$'
      key: "root-manual-ver"

sources:
  "root-manual":
    type: "manual"
    key: "{{ .ScanArgs.key }}"
    args:
      Version: "good"
`)
	oldBytesComplex := []byte(`
files:
  ".github/workflows/*.yml":
    scans:
      - gha-uses-vx
      - gha-uses-semver
      - gha-uses-commit

scans:
  gha-uses-vx:
    type: "regexp"
    source: "gha-uses-vx"
    args:
      regexp: '^\s+-?\s+uses: (?P<Repo>[^@/]+/[^@/]+)[^@]*@(?P<Commit>[0-9a-f]+)\s+#\s+(?P<Version>v\d+)\s*$'
  gha-uses-semver:
    type: "regexp"
    source: "gha-uses-semver"
    args:
      regexp: '^\s+-?\s+uses: (?P<Repo>[^@/]+/[^@/]+)[^@]*@(?P<Commit>[0-9a-f]+)\s+#\s+(?P<Version>v\d+\.\d+\.\d+)\s*$'
  gha-uses-commit:
    type: "regexp"
    source: "github-commit-match"
    args:
      regexp: '^\s+-?\s+uses: (?P<Repo>[^@/]+/[^@/]+)[^@]*@(?P<Version>[0-9a-f]+)\s+#\s+(?P<Ref>[\w\d\.]+)\s*$'

sources:
  gha-uses-vx:
    type: "git"
    key: "{{ .ScanMatch.Repo }}"
    args:
      type: "tag"
      url: "https://github.com/{{ .ScanMatch.Repo }}.git"
    filter:
      expr: '^v\d+$'
    sort:
      method: "semver"
  gha-uses-semver:
    type: "git"
    key: "{{ .ScanMatch.Repo }}"
    args:
      type: "tag"
      url: "https://github.com/{{ .ScanMatch.Repo }}.git"
    filter:
      expr: '^v\d+\.\d+\.\d+$'
    sort:
      method: "semver"
  github-commit-match:
    type: "git"
    key: "{{ .ScanMatch.Repo }}:{{ .ScanMatch.Ref }}"
    args:
      type: "commit"
      url: "https://github.com/{{ .ScanMatch.Repo }}.git"
      ref: "{{ .ScanMatch.Ref }}"
    filter:
      expr: "^{{ .ScanMatch.Ref }}$"
`)
	curBytesManual := []byte(`
files:
  "root-*.txt":
    processors:
      - root-manual

processors:
  "root-manual":
    scan: "regexp"
    scanArgs:
      regexp: '^manual-ver=(?P<Version>[^\s]+)\s*$'
    source: "manual"
    sourceArgs:
      Version: "good"
    key: "root-manual-ver"

scans:
  "regexp":
    type: "regexp"

sources:
  "manual":
    type: "manual"
`)

	cOldManual, err := LoadReader(bytes.NewReader(oldBytesManual))
	if err != nil {
		t.Fatalf("failed to LoadReader: %v", err)
	}
	cOldComplex, err := LoadReader(bytes.NewReader(oldBytesComplex))
	if err != nil {
		t.Fatalf("failed to LoadReader: %v", err)
	}
	cCurManual, err := LoadReader(bytes.NewReader(curBytesManual))
	if err != nil {
		t.Fatalf("failed to LoadReader: %v", err)
	}
	tt := []struct {
		name   string
		test   *Config
		expect *Config
	}{
		{
			name: "new",
			test: cNew,
			expect: &Config{
				Files:      map[string]*File{},
				Processors: map[string]*Processor{},
				Scans:      map[string]*Scan{},
				Sources:    map[string]*Source{},
			},
		},
		{
			name: "old-manual",
			test: cOldManual,
			expect: &Config{
				Files: map[string]*File{
					"root-*.txt": {
						Name:       "root-*.txt",
						Processors: []string{"root-manual"},
					},
				},
				Processors: map[string]*Processor{
					"root-manual": {
						Name:       "root-manual",
						Scan:       "root-manual",
						ScanArgs:   map[string]string{},
						Source:     "root-manual",
						SourceArgs: map[string]string{},
						Key:        `{{ .ScanArgs.key }}`,
						Filter:     Filter{},
						Sort:       Sort{},
						Template:   "",
					},
				},
				Scans: map[string]*Scan{
					"root-manual": {
						Name: "root-manual",
						Type: "regexp",
						Args: map[string]string{
							"regexp": `^manual-ver=(?P<Version>[^\s]+)\s*$`,
							"key":    "root-manual-ver",
						},
					},
				},
				Sources: map[string]*Source{
					"root-manual": {
						Name: "root-manual",
						Type: "manual",
						Args: map[string]string{
							"Version": "good",
						},
					},
				},
			},
		},
		{
			name: "old-complex",
			test: cOldComplex,
			expect: &Config{
				Files: map[string]*File{
					".github/workflows/*.yml": {
						Name: ".github/workflows/*.yml",
						Processors: []string{
							"gha-uses-vx",
							"gha-uses-semver",
							"gha-uses-commit",
						},
					},
				},
				Processors: map[string]*Processor{
					"gha-uses-vx": {
						Name:       "gha-uses-vx",
						Scan:       "gha-uses-vx",
						ScanArgs:   map[string]string{},
						Source:     "gha-uses-vx",
						SourceArgs: map[string]string{},
						Key:        `{{ .ScanMatch.Repo }}`,
						Filter: Filter{
							Expr: `^v\d+$`,
						},
						Sort: Sort{
							Method: "semver",
						},
					},
					"gha-uses-semver": {
						Name:       "gha-uses-semver",
						Scan:       "gha-uses-semver",
						ScanArgs:   map[string]string{},
						Source:     "gha-uses-semver",
						SourceArgs: map[string]string{},
						Key:        `{{ .ScanMatch.Repo }}`,
						Filter: Filter{
							Expr: `^v\d+\.\d+\.\d+$`,
						},
						Sort: Sort{
							Method: "semver",
						},
					},
					"gha-uses-commit": {
						Name:       "gha-uses-commit",
						Scan:       "gha-uses-commit",
						ScanArgs:   map[string]string{},
						Source:     "github-commit-match",
						SourceArgs: map[string]string{},
						Key:        `{{ .ScanMatch.Repo }}:{{ .ScanMatch.Ref }}`,
						Filter: Filter{
							Expr: `^{{ .ScanMatch.Ref }}$`,
						},
					},
				},
				Scans: map[string]*Scan{
					"gha-uses-vx": {
						Name: "gha-uses-vx",
						Type: "regexp",
						Args: map[string]string{
							"regexp": `^\s+-?\s+uses: (?P<Repo>[^@/]+/[^@/]+)[^@]*@(?P<Commit>[0-9a-f]+)\s+#\s+(?P<Version>v\d+)\s*$`,
						},
					},
					"gha-uses-semver": {
						Name: "gha-uses-semver",
						Type: "regexp",
						Args: map[string]string{
							"regexp": `^\s+-?\s+uses: (?P<Repo>[^@/]+/[^@/]+)[^@]*@(?P<Commit>[0-9a-f]+)\s+#\s+(?P<Version>v\d+\.\d+\.\d+)\s*$`,
						},
					},
					"gha-uses-commit": {
						Name: "gha-uses-commit",
						Type: "regexp",
						Args: map[string]string{
							"regexp": `^\s+-?\s+uses: (?P<Repo>[^@/]+/[^@/]+)[^@]*@(?P<Version>[0-9a-f]+)\s+#\s+(?P<Ref>[\w\d\.]+)\s*$`,
						},
					},
				},
				Sources: map[string]*Source{
					"gha-uses-vx": {
						Name: "gha-uses-vx",
						Type: "git",
						Args: map[string]string{
							"type": "tag",
							"url":  "https://github.com/{{ .ScanMatch.Repo }}.git",
						},
					},
					"gha-uses-semver": {
						Name: "gha-uses-semver",
						Type: "git",
						Args: map[string]string{
							"type": "tag",
							"url":  "https://github.com/{{ .ScanMatch.Repo }}.git",
						},
					},
					"github-commit-match": {
						Name: "github-commit-match",
						Type: "git",
						Args: map[string]string{
							"type": "commit",
							"url":  "https://github.com/{{ .ScanMatch.Repo }}.git",
							"ref":  "{{ .ScanMatch.Ref }}",
						},
					},
				},
			},
		},
		{
			name: "cur-manual",
			test: cCurManual,
			expect: &Config{
				Files: map[string]*File{
					"root-*.txt": {
						Name:       "root-*.txt",
						Processors: []string{"root-manual"},
					},
				},
				Processors: map[string]*Processor{
					"root-manual": {
						Name: "root-manual",
						Scan: "regexp",
						ScanArgs: map[string]string{
							"regexp": `^manual-ver=(?P<Version>[^\s]+)\s*$`,
						},
						Source: "manual",
						SourceArgs: map[string]string{
							"Version": "good",
						},
						Key:      `root-manual-ver`,
						Filter:   Filter{},
						Sort:     Sort{},
						Template: "",
					},
				},
				Scans: map[string]*Scan{
					"regexp": {
						Name: "regexp",
						Type: "regexp",
					},
				},
				Sources: map[string]*Source{
					"manual": {
						Name: "manual",
						Type: "manual",
					},
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			if tc.test == nil {
				t.Fatalf("test value is nil")
			}
			if tc.expect == nil {
				t.Fatalf("expect value is nil")
			}
			if tc.test.Files == nil || tc.test.Processors == nil || tc.test.Scans == nil || tc.test.Sources == nil {
				t.Fatalf("test fields contain nil values: %v", *tc.test)
			}
			if tc.expect.Files == nil || tc.expect.Processors == nil || tc.expect.Scans == nil || tc.expect.Sources == nil {
				t.Fatalf("expect fields contain nil values: %v", *tc.expect)
			}
			for k := range tc.expect.Files {
				if v, ok := tc.test.Files[k]; !ok || v == nil {
					t.Errorf("file is not defined: %s", k)
					continue
				}
				if tc.expect.Files[k].Name != tc.test.Files[k].Name ||
					!eqStrSlices(tc.expect.Files[k].Processors, tc.test.Files[k].Processors) {
					ej, _ := json.Marshal(*tc.expect.Files[k])
					tj, _ := json.Marshal(*tc.test.Files[k])
					t.Errorf("file entries do not match: %s != %s", string(ej), string(tj))
				}
			}
			for k := range tc.expect.Processors {
				if v, ok := tc.test.Processors[k]; !ok || v == nil {
					t.Errorf("processor is not defined: %s", k)
					continue
				}
				if !tc.expect.Processors[k].Equal(*tc.test.Processors[k]) {
					ej, _ := json.Marshal(*tc.expect.Processors[k])
					tj, _ := json.Marshal(*tc.test.Processors[k])
					t.Errorf("processor entries do not match: %s != %s", string(ej), string(tj))
				}
			}
			for k := range tc.expect.Scans {
				if v, ok := tc.test.Scans[k]; !ok || v == nil {
					t.Errorf("scan is not defined: %s", k)
					continue
				}
				if !tc.expect.Scans[k].Equal(*tc.test.Scans[k]) {
					ej, _ := json.Marshal(*tc.expect.Scans[k])
					tj, _ := json.Marshal(*tc.test.Scans[k])
					t.Errorf("scan entries do not match: %s != %s", string(ej), string(tj))
				}
			}
			for k := range tc.expect.Sources {
				if v, ok := tc.test.Sources[k]; !ok || v == nil {
					t.Errorf("source is not defined: %s", k)
					continue
				}
				if !tc.expect.Sources[k].Equal(*tc.test.Sources[k]) {
					ej, _ := json.Marshal(*tc.expect.Sources[k])
					tj, _ := json.Marshal(*tc.test.Sources[k])
					t.Errorf("source entries do not match: %s != %s", string(ej), string(tj))
				}
			}
		})
	}
}

// TODO: test clone
