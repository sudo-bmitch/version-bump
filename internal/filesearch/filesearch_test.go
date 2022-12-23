package filesearch

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

func TestPattern(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		newErr   error
		filename string
		prefix   bool
		expect   bool
	}{
		{
			name:     "empty",
			expr:     "",
			filename: "./filename.txt",
			prefix:   false,
			expect:   false,
		},
		{
			name:     "match file",
			expr:     "filename",
			filename: "filename",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "match path",
			expr:     "path/to/file.txt",
			filename: "path/to/file.txt",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "mismatch",
			expr:     "path/to/file.txt",
			filename: "path/from/file.csv",
			prefix:   false,
			expect:   false,
		},
		{
			name:     "prefix",
			expr:     "path/to/file.txt",
			filename: "path/to",
			prefix:   true,
			expect:   true,
		},
		{
			name:     "not prefix",
			expr:     "path/to/file.txt",
			filename: "path/from",
			prefix:   true,
			expect:   false,
		},
		{
			name:     "wildcard name",
			expr:     "path/to/*.txt",
			filename: "path/to/file.txt",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "wildcard ext",
			expr:     "file*",
			filename: "file",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "wildcard ext path",
			expr:     "path/to/file.*",
			filename: "path/to/file.txt",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "wildcard file",
			expr:     "path/to/*",
			filename: "path/to/file.txt",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "wildcard path",
			expr:     "path/*/file.txt",
			filename: "path/to/file.txt",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "wildcard path prefix",
			expr:     "path/*/file.txt",
			filename: "path/to",
			prefix:   true,
			expect:   true,
		},
		{
			name:     "wildcard path not prefix",
			expr:     "path/*/file.txt",
			filename: "path/to/subdir/file.txt",
			prefix:   true,
			expect:   false,
		},
		{
			name:     "wildcard file not prefix",
			expr:     "path/to/*",
			filename: "path/to/subdir/file.txt",
			prefix:   true,
			expect:   false,
		},
		{
			name:     "double star path",
			expr:     "**/file.txt",
			filename: "path/to/file.txt",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "double star root",
			expr:     "**/file.txt",
			filename: "file.txt",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "double star prefix",
			expr:     "**/file.txt",
			filename: "path/to",
			prefix:   true,
			expect:   true,
		},
		{
			name:     "double star mid nil",
			expr:     "path/**/file.txt",
			filename: "path/file.txt",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "double star mid multiple",
			expr:     "path/**/file.txt",
			filename: "path/to/sub/file.txt",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "escape star",
			expr:     "path/to/star\\*file.txt",
			filename: "path/to/star*file.txt",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "escape star wild",
			expr:     "path/to/*\\*file.txt",
			filename: "path/to/star*file.txt",
			prefix:   false,
			expect:   true,
		},
		{
			name:     "escape star prefix",
			expr:     "path/to/star\\*file.txt",
			filename: "path/to",
			prefix:   true,
			expect:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := newPattern(tt.expr)
			if tt.newErr != nil {
				if err == nil {
					t.Errorf("newPattern expected %v, err was nil", tt.newErr)
				} else if !errors.Is(err, tt.newErr) && err.Error() != tt.newErr.Error() {
					t.Errorf("newPattern expected %v, received %v", tt.newErr, err)
				}
				return
			}
			if err != nil {
				t.Errorf("newPattern failed with %v", err)
				return
			}
			result := p.match(tt.filename, tt.prefix)
			if result != tt.expect {
				t.Errorf("p.match expected %v, received %v", tt.expect, result)
			}
		})
	}
}

func TestWalk(t *testing.T) {
	curdir, err := os.Getwd()
	if err != nil {
		t.Errorf("current directory cannot be determined: %v", err)
	}
	defer os.Chdir(curdir)
	err = os.Chdir("../../testdata")
	if err != nil {
		t.Errorf("failed to chdir to testdata: %v", err)
		return
	}
	conf, err := config.LoadFile("ex-conf.yaml")
	if err != nil {
		t.Errorf("failed to load config: %v", err)
		return
	}
	w, err := New([]string{}, conf.Files)
	if err != nil {
		t.Errorf("failed to create walk: %v", err)
	}
	list := []struct {
		filename string
		confName string
	}{}
	for {
		filename, confName, err := w.Next()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				t.Errorf("failed with error other than eof: %v", err)
			}
			break
		}
		list = append(list, struct {
			filename string
			confName string
		}{filename: filename, confName: confName})
	}
	if len(list) < 2 ||
		list[0].filename != "01-example.sh" || list[0].confName != "**/*.sh" ||
		list[1].filename != "01-example.sh" || list[1].confName != "01-example.sh" {
		t.Errorf("expected 2 entries for 01-example.sh, received: %v", list)
	}
}
