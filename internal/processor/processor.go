// Package processor wraps the call to the scanner and requests to the source for a single type of update to a single file.
// It includes logic for filtering, sorting, and templating of the source output.
package processor

import (
	"context"
	"fmt"
	"io"
	"maps"
	"regexp"
	"sort"
	"strconv"

	"github.com/Masterminds/semver/v3"

	"github.com/sudo-bmitch/version-bump/internal/config"
	"github.com/sudo-bmitch/version-bump/internal/lockfile"
	"github.com/sudo-bmitch/version-bump/internal/scan"
	"github.com/sudo-bmitch/version-bump/internal/source"
	"github.com/sudo-bmitch/version-bump/internal/template"
)

type processor struct {
	Filename  string
	Processor config.Processor
	Scan      config.Scan   // clone of scan with args overridden from Processor
	Source    config.Source // clone of source with args overridden from Processor
	locks     *lockfile.Locks
	changes   []*Change
}

// Change lists changes found or made to scanned files.
type Change struct {
	Filename  string // filename modified
	Processor string // name of the processor
	Source    string // name of the source
	Scan      string // name of the scan
	Key       string // key from processor
	Orig      string // previous version
	New       string // new version
}

// Process is used to process a single file with a single scanner and source.
func Process(ctx context.Context, conf config.Config, procName, filename string, r io.Reader, w io.Writer, locks *lockfile.Locks) ([]*Change, error) {
	var err error
	cProcOrig, ok := conf.Processors[procName]
	if !ok || cProcOrig == nil {
		return nil, fmt.Errorf("processor not defined: %s", procName)
	}
	cProc := cProcOrig.Clone()
	cScanOrig, ok := conf.Scans[cProcOrig.Scan]
	if !ok || cScanOrig == nil {
		return nil, fmt.Errorf("scanner not defined: %s", cProcOrig.Scan)
	}
	cScan := cScanOrig.Clone()
	cScan.Args = argsMerge(cScan.Args, cProc.ScanArgs)
	cSourceOrig, ok := conf.Sources[cProcOrig.Source]
	if !ok || cSourceOrig == nil {
		return nil, fmt.Errorf("source not defined: %s", cProcOrig.Source)
	}
	cSource := cSourceOrig.Clone()
	cSource.Args = argsMerge(cSource.Args, cProc.SourceArgs)
	p := processor{
		Filename:  filename,
		Processor: cProc,
		Scan:      cScan,
		Source:    cSource,
		locks:     locks,
		changes:   []*Change{},
	}
	// template various fields
	cScan.Args, err = templateArgs(cScan.Args, p)
	if err != nil {
		return nil, fmt.Errorf("failed to template scan args: %w", err)
	}
	err = scan.Run(ctx, cScan, filename, r, w, p.getVer)
	if err != nil {
		return nil, fmt.Errorf("scanner %s failed for %s: %w", cScan.Name, filename, err)
	}
	return p.changes, nil
}

func (p *processor) getVer(curVer string, matchArgs map[string]string) (string, error) {
	var err error
	src := p.Source.Clone()
	tdp := tmplDataProcess{
		processor: *p,
		ScanMatch: matchArgs,
	}
	// apply templates
	src.Args, err = templateArgs(src.Args, tdp)
	if err != nil {
		return curVer, fmt.Errorf("failed to template args: processor=%v, %v", *p, err)
	}
	tdp.processor.Source = src
	key, err := template.String(p.Processor.Key, tdp)
	if err != nil {
		return curVer, fmt.Errorf("failed to template key: processor=%v, %v", *p, err)
	}
	tdp.Processor.Key = key
	// TODO: handle different options for source (read from current lock or real source)
	results, err := source.Get(src)
	if err != nil {
		return curVer, fmt.Errorf("failed to query source %s: %v", src.Name, err)
	}
	// filter, sort, and template results
	newVer, err := p.resultsToVer(results, tdp)
	if err != nil {
		return curVer, err
	}
	// manage version locks
	err = p.locks.Set(p.Processor.Name, key, newVer)
	if err != nil {
		return curVer, err
	}
	// track changes
	if newVer != curVer {
		p.changes = append(p.changes, &Change{
			Filename:  p.Filename,
			Processor: p.Processor.Name,
			Scan:      p.Processor.Scan,
			Source:    p.Processor.Source,
			Key:       key,
			Orig:      curVer,
			New:       newVer,
		})
	}
	return newVer, nil
}

func (p *processor) resultsToVer(results source.Results, tdp tmplDataProcess) (string, error) {
	// build a list of keys/versions that match the filter
	var filterExp *regexp.Regexp
	if p.Processor.Filter.Expr != "" {
		expr, err := template.String(p.Processor.Filter.Expr, tdp)
		if err != nil {
			return "", fmt.Errorf("failed to process template \"%s\": %w", p.Processor.Filter.Expr, err)
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return "", fmt.Errorf("failed to compile filter expr \"%s\": %w", expr, err)
		}
		filterExp = re
	}
	// Keys are sorted.
	// They may be the result of templating, in which case k2v is needed to return to the version.
	keys := make([]string, 0, len(results.VerMap))
	k2v := map[string]string{}
	for v := range results.VerMap {
		if filterExp != nil && !filterExp.MatchString(v) {
			continue
		}
		k := v
		if p.Processor.Sort.Template != "" {
			kt, err := template.String(p.Processor.Sort.Template, v)
			if err != nil {
				continue
			}
			k = kt
		}
		k2v[k] = v
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return "", fmt.Errorf("no results found matching the filter %s", p.Processor.Filter.Expr)
	}
	// sort according to the specified method
	switch p.Processor.Sort.Method {
	case "semver":
		vers := make([]*semver.Version, 0, len(keys))
		for _, k := range keys {
			sv, err := semver.NewVersion(k)
			if err != nil {
				continue // ignore versions that do not compile
			}
			vers = append(vers, sv)
		}
		if len(vers) == 0 {
			return "", fmt.Errorf("no valid semver versions found in %v", keys)
		}
		if p.Processor.Sort.Asc {
			sort.Sort(semver.Collection(vers))
		} else {
			sort.Sort(sort.Reverse(semver.Collection(vers)))
		}
		// rebuild keys from parsed semver
		keys = make([]string, len(vers))
		for i, sv := range vers {
			keys[i] = sv.Original()
		}
	case "numeric":
		keyInts := make([]int, 0, len(keys))
		orig := map[int]string{} // map from int back to original value
		for _, k := range keys {
			// parse numbers from keys
			i, err := strconv.Atoi(k)
			if err != nil {
				continue // ignore versions that are not numeric
			}
			keyInts = append(keyInts, i)
			orig[i] = k
		}
		if len(keyInts) == 0 {
			return "", fmt.Errorf("no valid numeric versions found in %v", keys)
		}
		if p.Processor.Sort.Asc {
			sort.Sort(sort.IntSlice(keyInts))
		} else {
			sort.Sort(sort.Reverse(sort.IntSlice(keyInts)))
		}
		keys = make([]string, len(keyInts))
		for i, iv := range keyInts {
			keys[i] = orig[iv]
		}
	default:
		if p.Processor.Sort.Asc {
			sort.Sort(sort.StringSlice(keys))
		} else {
			sort.Sort(sort.Reverse(sort.StringSlice(keys)))
		}
	}
	// convert the keys back to a list of versions (reverse the templating)
	verList := make([]string, len(keys))
	for i, k := range keys {
		verList[i] = k2v[k]
	}
	// select the requested offset and template
	if p.Processor.Sort.Offset < 0 {
		return "", fmt.Errorf("offset cannot be negative")
	}
	if len(verList) <= p.Processor.Sort.Offset {
		return "", fmt.Errorf("requested offset is too large, %d matching versions found: %v", len(verList), verList)
	}
	tdr := tmplDataResults{
		Results: results,
		VerList: verList,
		Version: results.VerMap[verList[p.Processor.Sort.Offset]],
	}
	if p.Processor.Template != "" {
		return template.String(p.Processor.Template, tdr)
	}
	return tdr.Version, nil
}

// tmplDataProcess is template data wrapping the [processor] struct.
type tmplDataProcess struct {
	processor
	ScanMatch map[string]string // current matches from the running scan, used for templating
}

// ScanArgs provides backwards compatibility for templating.
func (tdp tmplDataProcess) ScanArgs() map[string]string {
	return tdp.Scan.Args
}

// SourceArgs provides backwards compatibility for templating.
func (tdp tmplDataProcess) SourceArgs() map[string]string {
	return tdp.Source.Args
}

// tmplDataResults is the template data wrapping the [source.Results] struct.
type tmplDataResults struct {
	source.Results
	VerList []string
	Version string
}

func argsMerge(mList ...map[string]string) map[string]string {
	r := map[string]string{}
	for _, m := range mList {
		maps.Copy(r, m)
	}
	return r
}

func templateArgs(args map[string]string, data any) (map[string]string, error) {
	out := map[string]string{}
	var err error
	for k, in := range args {
		out[k], err = template.String(in, data)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}
