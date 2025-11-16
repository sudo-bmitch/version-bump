package processor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/sudo-bmitch/version-bump/internal/config"
	"github.com/sudo-bmitch/version-bump/internal/lockfile"
	"github.com/sudo-bmitch/version-bump/internal/source"
)

func TestProcessor(t *testing.T) {
	// func Process(ctx context.Context, conf config.Config, procName, filename string, r io.Reader, w io.Writer, locks *lockfile.Locks) ([]*Change, error) {
	ctx := context.TODO()
	confGHAUses := config.Config{
		Files: map[string]*config.File{
			".github/workflows/*.yml": {
				Name: ".github/workflows/*.yml",
				Processors: []string{
					"gha-uses-semver-3.5",
					"gha-uses-commit",
				},
			},
		},
		Processors: map[string]*config.Processor{
			"gha-uses-semver-3.5": {
				Name:       "gha-uses-semver-3.5",
				Scan:       "gha-uses-semver",
				ScanArgs:   map[string]string{},
				Source:     "gha-uses-semver",
				SourceArgs: map[string]string{},
				Key:        `{{ .ScanMatch.Repo }}`,
				Filter: config.Filter{
					Expr: `^v3\.5\.\d+$`,
				},
				Sort: config.Sort{
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
				Filter: config.Filter{
					Expr: `^{{ .ScanMatch.Ref }}$`,
				},
			},
		},
		Scans: map[string]*config.Scan{
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
		Sources: map[string]*config.Source{
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
	}
	tt := []struct {
		name         string
		conf         config.Config
		procName     string
		filename     string
		in           []byte
		expectOut    []byte
		expectChange []*Change
		expectErr    error
		expectLocks  *lockfile.Locks
	}{
		{
			name:      "empty",
			filename:  "test",
			procName:  "missing",
			expectErr: fmt.Errorf("processor not defined: missing"),
		},
		{
			name:     "manual",
			filename: "test",
			procName: "manual",
			conf: config.Config{
				Processors: map[string]*config.Processor{
					"manual": {
						Name: "manual",
						Scan: "regexp",
						ScanArgs: map[string]string{
							"regexp": `^testVer=(?P<Version>[0-9.]+)`,
						},
						Source: "manual",
						SourceArgs: map[string]string{
							"Version": "4.3.2.1",
						},
						Key: "manual",
					},
				},
				Scans: map[string]*config.Scan{
					"regexp": {
						Type: "regexp",
					},
				},
				Sources: map[string]*config.Source{
					"manual": {
						Type: "manual",
					},
				},
			},
			in:        []byte(`testVer=1.2.3.4`),
			expectOut: []byte(`testVer=4.3.2.1`),
			expectChange: []*Change{
				{
					Filename:  "test",
					Processor: "manual",
					Source:    "manual",
					Scan:      "regexp",
					Key:       "manual",
					Orig:      `1.2.3.4`,
					New:       `4.3.2.1`,
				},
			},
			expectLocks: &lockfile.Locks{
				Lock: map[string]map[string]*lockfile.Lock{
					"manual": {
						"manual": {
							Name:    "manual",
							Key:     "manual",
							Version: `4.3.2.1`,
						},
					},
				},
			},
		},
		{
			name:     "filter-git-tag",
			filename: "test",
			procName: "git-tag-v0.3",
			conf: config.Config{
				Processors: map[string]*config.Processor{
					"git-tag-v0.3": {
						Name: "git-tag-v0.3",
						Scan: "regexp",
						ScanArgs: map[string]string{
							"regexp": `^testVer\[(?P<repo>[a-z/]+)\]=(?P<Version>v[0-9.]+)`,
						},
						Source: "git-tag",
						SourceArgs: map[string]string{
							"url": "https://github.com/{{ .ScanMatch.repo }}.git",
						},
						Key: "{{ .ScanMatch.repo }}-v0.3",
						Filter: config.Filter{
							Expr: `^v0\.3\.\d+$`,
						},
						Sort: config.Sort{
							Method: "semver",
						},
					},
				},
				Scans: map[string]*config.Scan{
					"regexp": {
						Type: "regexp",
					},
				},
				Sources: map[string]*config.Source{
					"git-tag": {
						Type: "git",
						Args: map[string]string{
							"type": "tag",
						},
					},
				},
			},
			in:        []byte(`testVer[regclient/regclient]=v0.3.8`),
			expectOut: []byte(`testVer[regclient/regclient]=v0.3.10`),
			expectChange: []*Change{
				{
					Filename:  "test",
					Processor: "git-tag-v0.3",
					Source:    "git-tag",
					Scan:      "regexp",
					Key:       "regclient/regclient-v0.3",
					Orig:      `v0.3.8`,
					New:       `v0.3.10`,
				},
			},
			expectLocks: &lockfile.Locks{
				Lock: map[string]map[string]*lockfile.Lock{
					"git-tag-v0.3": {
						"regclient/regclient-v0.3": {
							Name:    "git-tag-v0.3",
							Key:     "regclient/regclient-v0.3",
							Version: `v0.3.10`,
						},
					},
				},
			},
		},
		{
			name:     "filter-git-commit",
			filename: "test",
			procName: "filter-git-commit",
			conf: config.Config{
				Processors: map[string]*config.Processor{
					"filter-git-commit": {
						Name: "filter-git-commit",
						Scan: "regexp",
						ScanArgs: map[string]string{
							"regexp": `^testVer\[(?P<repo>[a-z/]+)@(?P<tag>v[0-9.]+)\]=(?P<Version>[0-9a-f]+)`,
						},
						Source: "git-commit",
						SourceArgs: map[string]string{
							"url": "https://github.com/{{ .ScanMatch.repo }}.git",
						},
						Key: "{{ .ScanMatch.repo }}-{{ .ScanMatch.tag }}",
						Filter: config.Filter{
							Expr: `^{{ .ScanMatch.tag }}$`,
						},
						Sort: config.Sort{},
					},
				},
				Scans: map[string]*config.Scan{
					"regexp": {
						Type: "regexp",
					},
				},
				Sources: map[string]*config.Source{
					"git-commit": {
						Type: "git",
					},
				},
			},
			in:        []byte(`testVer[regclient/regclient@v0.3.10]=c0d4e8078e3e40d9854010a52e8353f98c8ae1ed`),
			expectOut: []byte(`testVer[regclient/regclient@v0.3.10]=6a1a13c410f734f5e18a6032936bc6764814eae7`),
			expectChange: []*Change{
				{
					Filename:  "test",
					Processor: "filter-git-commit",
					Source:    "git-commit",
					Scan:      "regexp",
					Key:       "regclient/regclient-v0.3.10",
					Orig:      `c0d4e8078e3e40d9854010a52e8353f98c8ae1ed`,
					New:       `6a1a13c410f734f5e18a6032936bc6764814eae7`,
				},
			},
			expectLocks: &lockfile.Locks{
				Lock: map[string]map[string]*lockfile.Lock{
					"filter-git-commit": {
						"regclient/regclient-v0.3.10": {
							Name:    "filter-git-commit",
							Key:     "regclient/regclient-v0.3.10",
							Version: `6a1a13c410f734f5e18a6032936bc6764814eae7`,
						},
					},
				},
			},
		},
		{
			name:     "gha-uses-semver-3.5",
			filename: ".github/workflows/file.yml",
			procName: "gha-uses-semver-3.5",
			conf:     confGHAUses,
			in: []byte(`    steps:
    - name: Check out code
      uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3.5.2`),
			expectOut: []byte(`    steps:
    - name: Check out code
      uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3.5.3`),
			expectChange: []*Change{
				{
					Filename:  ".github/workflows/file.yml",
					Processor: "gha-uses-semver-3.5",
					Source:    "gha-uses-semver",
					Scan:      "gha-uses-semver",
					Key:       "actions/checkout",
					Orig:      "v3.5.2",
					New:       "v3.5.3",
				},
			},
			expectLocks: &lockfile.Locks{
				Lock: map[string]map[string]*lockfile.Lock{
					"gha-uses-semver-3.5": {
						"actions/checkout": {
							Name:    "gha-uses-semver-3.5",
							Key:     "actions/checkout",
							Version: "v3.5.3",
						},
					},
				},
			},
		},
		{
			name:     "gha-uses-commit",
			filename: ".github/workflows/file.yml",
			procName: "gha-uses-commit",
			conf:     confGHAUses,
			in: []byte(`    steps:
    - name: Check out code
      uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3.5.3`),
			expectOut: []byte(`    steps:
    - name: Check out code
      uses: actions/checkout@c85c95e3d7251135ab7dc9ce3241c5835cc595a9 # v3.5.3`),
			expectChange: []*Change{
				{
					Filename:  ".github/workflows/file.yml",
					Processor: "gha-uses-commit",
					Source:    "github-commit-match",
					Scan:      "gha-uses-commit",
					Key:       "actions/checkout:v3.5.3",
					Orig:      "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
					New:       "c85c95e3d7251135ab7dc9ce3241c5835cc595a9",
				},
			},
			expectLocks: &lockfile.Locks{
				Lock: map[string]map[string]*lockfile.Lock{
					"gha-uses-commit": {
						"actions/checkout": {
							Name:    "gha-uses-commit",
							Key:     "actions/checkout:v3.5.3",
							Version: "c85c95e3d7251135ab7dc9ce3241c5835cc595a9",
						},
					},
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			bufIn := bytes.NewBuffer(tc.in)
			bufOut := new(bytes.Buffer)
			locks := lockfile.New()
			resultChange, resultErr := Process(ctx, tc.conf, tc.procName, tc.filename, bufIn, bufOut, locks)
			if tc.expectErr != nil {
				if resultErr == nil || (!errors.Is(resultErr, tc.expectErr) && resultErr.Error() != tc.expectErr.Error()) {
					t.Fatalf("expected error %v, received %v", tc.expectErr, resultErr)
				}
				return
			}
			if resultErr != nil {
				t.Fatalf("unexpected error %v", resultErr)
			}
			out := bufOut.Bytes()
			if !bytes.Equal(tc.expectOut, out) {
				t.Errorf("unexpected output, expected %s, received %s", tc.expectOut, out)
			}
			if len(resultChange) != len(tc.expectChange) {
				t.Errorf("unexpected changes, expected %v, received %v", tc.expectChange, resultChange)
			} else {
				for i, expect := range tc.expectChange {
					if *expect != *resultChange[i] {
						t.Errorf("unexpected change entry %d: expected %v, received %v", i, *expect, *resultChange[i])
					}
				}
			}
			lockOutBuf := new(bytes.Buffer)
			lockExpectBuf := new(bytes.Buffer)
			err := locks.SaveWriter(lockOutBuf, false)
			if err != nil {
				t.Errorf("failed to write locks: %v", err)
			}
			err = tc.expectLocks.SaveWriter(lockExpectBuf, false)
			if err != nil {
				t.Errorf("failed to write locks: %v", err)
			}
			lockOut := lockOutBuf.Bytes()
			lockExpect := lockExpectBuf.Bytes()
			if !bytes.Equal(lockOut, lockExpect) {
				t.Errorf("unexpected locks: expected %s, received %s", lockExpect, lockOut)
			}
		})
	}
}

func TestResultsToVer(t *testing.T) {
	tt := []struct {
		name    string
		p       processor
		results source.Results
		tdp     tmplDataProcess
		expect  string
		err     error
	}{
		{
			name: "semver",
			p: processor{
				Filename: "test-semver",
				Processor: config.Processor{
					Name: "semver",
					Filter: config.Filter{
						Expr: "",
					},
					Sort: config.Sort{
						Method: "semver",
					},
				},
			},
			results: source.Results{
				VerMap: map[string]string{
					"1.2.3": "1.2.3",
					"1.2.4": "1.2.4",
					"1.3.3": "1.3.3",
					"2.2.3": "2.2.3",
				},
			},
			tdp: tmplDataProcess{
				ScanMatch: map[string]string{},
			},
			expect: "2.2.3",
		},
		{
			name: "go-subpackage",
			p: processor{
				Filename: "test-go-subpackage",
				Processor: config.Processor{
					Name: "semver",
					Filter: config.Filter{
						Expr: "subpackage/.*",
					},
					Sort: config.Sort{
						Method:   "semver",
						Template: "{{ index (split . \"/\") 1 }}",
					},
					Template: "{{ index (split .Version \"/\") 1 }}",
				},
			},
			results: source.Results{
				VerMap: map[string]string{
					"subpackage/1.2.3": "subpackage/1.2.3",
					"1.2.4":            "1.2.4",
					"subpackage/1.3.3": "subpackage/1.3.3",
					"2.2.3":            "2.2.3",
				},
			},
			tdp: tmplDataProcess{
				ScanMatch: map[string]string{},
			},
			expect: "1.3.3",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tc.tdp.processor = tc.p
			out, err := tc.p.resultsToVer(tc.results, tc.tdp)
			if tc.err != nil {
				if tc.err.Error() != err.Error() && !errors.Is(err, tc.err) {
					t.Errorf("expected error %v, received %v", tc.err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.expect != out {
				t.Errorf("expected version %q, received %q", tc.expect, out)
			}
		})
	}
}
