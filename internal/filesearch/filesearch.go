// Package filesearch is used to retrieve files for scanning
package filesearch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

type walk struct {
	conf    map[string]*config.File // list of conf entries to search for
	confKey []string                // list of keys from the conf, list aligns with confPat
	confPat []*pattern              // patterns for each conf entry
	paths   []string                // list of files/dirs to process
	curPath [][]string              // current directory queue, curPath[i+1][] = subdir entries of curPath[i][0]
	curConf int                     // index of last returned conf, used when a path matches multiple scans
	// matched map[string]bool // TODO: list of entries that have already been matched and can be skipped, matches need to be for both filename and confName
}

// New returns a directory traversal struct, implementing the Next() method to walk all paths according to conf
func New(paths []string, conf map[string]*config.File) (*walk, error) {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	confKey := make([]string, 0, len(conf))
	for i := range conf {
		confKey = append(confKey, i)
	}
	sort.Strings(confKey)
	confPat := make([]*pattern, len(confKey))
	for i, name := range confKey {
		p, err := newPattern(name)
		if err != nil {
			return nil, fmt.Errorf("failed to parse \"%s\": %w", name, err)
		}
		confPat[i] = p
	}
	return &walk{
		conf:    conf,
		confKey: confKey,
		confPat: confPat,
		paths:   paths,
		curPath: [][]string{},
		curConf: -1,
	}, nil
}

// Next returns: filename, name of the matching File expression in the config, and any errors
func (w *walk) Next() (string, string, error) {
	// loop until EOF, fatal error, or match found
	for {
		// if all conf entries checked on the current path have been checked, pop last entry
		if w.curConf+1 >= len(w.confPat) {
			w.popCurPath()
		}

		// if the entire tree has been walked, add the next entry from paths to curPath
		if len(w.curPath) == 0 {
			if len(w.paths) == 0 {
				return "", "", fmt.Errorf("end of list%.0w", io.EOF)
			}
			pathSplit := strings.Split(filepath.Clean(w.paths[0]), string(filepath.Separator))
			w.paths = w.paths[1:]
			w.curPath = make([][]string, len(pathSplit))
			for i := range pathSplit {
				w.curPath[i] = []string{pathSplit[i]}
			}
			w.curConf = -1
		}

		// build the current path and stat it
		fileSplit := make([]string, len(w.curPath))
		for i := range w.curPath {
			fileSplit[i] = w.curPath[i][0]
		}
		filename := filepath.Join(fileSplit...)
		fi, err := os.Stat(filename)
		if err != nil {
			return "", "", fmt.Errorf("failed to read file %s: %w", filename, err)
		}

		// for directories
		if fi.IsDir() {
			// remove/skip if no matching w.conf prefix
			foundPrefix := false
			// always search the current dir, regexp will not otherwise match this
			if filename == "." || filename == "/" {
				foundPrefix = true
			}
			for i := 0; i < len(w.confPat) && !foundPrefix; i++ {
				if w.confPat[i].match(filename, true) {
					foundPrefix = true
				}
			}
			if !foundPrefix {
				w.popCurPath()
				continue
			}
			// else add subdir entries
			deList, err := os.ReadDir(filename)
			if err != nil {
				w.popCurPath()
				return "", "", fmt.Errorf("failed to read directory %s: %w", filename, err)
			}
			if len(deList) == 0 {
				w.popCurPath()
				continue
			}
			deNames := make([]string, len(deList))
			for i := range deList {
				deNames[i] = deList[i].Name()
			}
			w.curPath = append(w.curPath, deNames)
			continue
		}

		// for files, check each conf to see if it matches
		w.curConf++
		for w.curConf < len(w.confPat) {
			if w.confPat[w.curConf].match(filename, false) {
				return filename, w.confKey[w.curConf], nil
			}
			w.curConf++
		}
	}
}

// popCurPath is used to finish processing of the curPath, removing the top entry
func (w *walk) popCurPath() {
	// remove last path entry, recursive if entry is was the last entry in subDir
	for {
		i := len(w.curPath) - 1
		if i < 0 {
			// end of curPath
			return
		}
		// last subdir contains multiple entries, remove head
		if len(w.curPath[i]) > 1 {
			w.curPath[i] = w.curPath[i][1:]
			w.curConf = -1
			return
		}
		// last entry in subdir, remove and repeat in parent
		w.curPath = w.curPath[:i]
	}
}

// pattern is used to compare a file or directory to a regexp
type pattern struct {
	full, prefix *regexp.Regexp
}

// newPattern converts a string to a set of regexp's for matching the full file or directory
func newPattern(expr string) (*pattern, error) {
	expr = filepath.Clean(expr)
	reParts := []string{}
	reCurStr := ""
	state := "default"
	for _, ch := range expr {
		switch state {
		case "default":
			switch ch {
			case '\\':
				state = "escape"
			case '*':
				state = "star"
			case '/':
				reParts = append(reParts, reCurStr)
				// "**/" matches an empty path too, so separator is optional
				if reCurStr == ".*" || reCurStr == regexp.QuoteMeta(string(filepath.Separator))+".*" {
					reCurStr = regexp.QuoteMeta(string(filepath.Separator)) + "?"
				} else {
					reCurStr = regexp.QuoteMeta(string(filepath.Separator))
				}
			default:
				reCurStr += regexp.QuoteMeta(string(ch))
			}
		case "escape":
			reCurStr += "\\" + string(ch)
			state = "default"
		case "star":
			state = "default"
			if ch == '*' {
				// ** matches anything, even across path separators
				reCurStr += ".*"
			} else {
				// * matches only within the current path
				reCurStr += "[^" + regexp.QuoteMeta(string(filepath.Separator)) + "]*"
				if ch == '\\' {
					state = "escape"
				} else if ch == '/' {
					reParts = append(reParts, reCurStr)
					reCurStr = regexp.QuoteMeta(string(filepath.Separator))
				} else {
					reCurStr += regexp.QuoteMeta(string(ch))
				}
			}
		}
	}
	if state == "star" {
		reCurStr += "[^" + regexp.QuoteMeta(string(filepath.Separator)) + "]*"
	}
	reParts = append(reParts, reCurStr)

	// full match requires the entire path to match
	reFullStr := "^" + strings.Join(reParts, "") + "$"
	// partial match makes every successive path entry optional
	rePartStr := "^" + strings.Join(reParts, "(?:")
	for i := 0; i < len(reParts)-1; i++ {
		rePartStr += ")?"
	}
	rePartStr += "$"
	p := pattern{
		full:   regexp.MustCompile(reFullStr),
		prefix: regexp.MustCompile(rePartStr),
	}

	return &p, nil
}

// isMatch indicates if a pattern matches a specific file (or dir prefix)
func (p *pattern) match(filename string, prefix bool) bool {
	filename = filepath.Clean(filename)
	if prefix {
		return p.prefix.Match([]byte(filename))
	}
	return p.full.Match([]byte(filename))
}
