package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

type cobraTestOpts struct {
	stdin io.Reader
}

func cobraTest(t *testing.T, opts *cobraTestOpts, args ...string) (string, error) {
	t.Helper()

	buf := new(bytes.Buffer)
	rootCmd := NewRootCmd()
	if opts != nil && opts.stdin != nil {
		rootCmd.SetIn(opts.stdin)
	}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()
	return strings.TrimSpace(buf.String()), err
}

func TestRootCmd(t *testing.T) {
	// TODO: copy testdata to a temp dir and test scan and update commands
	tt := []struct {
		name        string
		args        []string
		expectErr   error
		expectOut   string
		outContains bool
	}{
		{
			name:        "Version",
			args:        []string{"version"},
			expectOut:   "VCSRef:",
			outContains: true,
		},
		{
			name: "Check-Good",
			args: []string{"check", "--conf", "./testdata/root-conf.yaml", "root-good.txt"},
		},
		{
			name:      "Check-Bad",
			args:      []string{"check", "--conf", "./testdata/root-conf.yaml", "root-bad.txt"},
			expectErr: fmt.Errorf("changes detected"),
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			out, err := cobraTest(t, nil, tc.args...)
			if tc.expectErr != nil {
				if err == nil {
					t.Errorf("did not receive expected error: %v", tc.expectErr)
				} else if !errors.Is(err, tc.expectErr) && err.Error() != tc.expectErr.Error() {
					t.Errorf("unexpected error, received %v, expected %v", err, tc.expectErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("returned unexpected error: %v", err)
			}
			if (!tc.outContains && out != tc.expectOut) || (tc.outContains && !strings.Contains(out, tc.expectOut)) {
				t.Errorf("unexpected output, expected %s, received %s", tc.expectOut, out)
			}
		})
	}
}
