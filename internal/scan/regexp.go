package scan

import (
	"fmt"
	"io"
	"regexp"

	"github.com/sudo-bmitch/version-bump/internal/action"
	"github.com/sudo-bmitch/version-bump/internal/config"
)

type reScan struct {
	action   *action.Action
	conf     config.Scan
	srcRdr   io.ReadCloser
	outRdr   *io.PipeReader
	re       *regexp.Regexp
	nameInd  map[string]int
	filename string
}

func newREScan(conf config.Scan, srcRdr io.ReadCloser, a *action.Action, filename string) (Scan, error) {
	// validate config, extract and compile regexp
	if _, ok := conf.Args["regexp"]; !ok {
		return nil, fmt.Errorf("scan regexp arg is missing for %s", conf.Name)
	}
	re, err := regexp.Compile("(?m)" + conf.Args["regexp"])
	if err != nil {
		return nil, fmt.Errorf("scan regexp does not compile for %s: %s: %w", conf.Name, conf.Args["regexp"], err)
	}
	// extract index of each subexp
	subNames := re.SubexpNames()
	nameInd := map[string]int{}
	for i, name := range subNames {
		nameInd[name] = i
	}
	// verify regexp contains a "Version" match
	if _, ok := nameInd["Version"]; !ok {
		return nil, fmt.Errorf("scan regexp is missing Version submatch (i.e. \"(?P<Version>\\d+)\") for %s: %s", conf.Name, conf.Args["regexp"])
	}

	// create pipe
	pipeRdr, pipeWrite := io.Pipe()

	// configure buf reader
	r := &reScan{
		action:   a,
		conf:     conf,
		srcRdr:   srcRdr,
		outRdr:   pipeRdr,
		re:       re,
		nameInd:  nameInd,
		filename: filename,
	}

	// TODO: have a separate implementation for reading the full file vs individual lines
	// run pipe handler in goroutine
	go r.handlePipe(pipeWrite)

	return r, nil
}

func (r *reScan) handlePipe(pw *io.PipeWriter) {
	lastIndex := 0
	b, err := io.ReadAll(r.srcRdr)
	if err != nil {
		pw.CloseWithError(err)
		return
	}
	// scan buf for all regexp matches
	matchIndexList := r.re.FindAllSubmatchIndex(b, -1)
	matchData := config.SourceTmplData{
		Filename: r.filename,
		ScanArgs: r.conf.Args,
	}

	// for each result, build arg map, call action, handle response
	for _, matchIndexes := range matchIndexList {
		regexpMatches := map[string]string{}
		for name, i := range r.nameInd {
			i1, i2 := i*2, (i*2)+1
			if i2 >= len(matchIndexes) {
				// skip if not enough entries in match
				// TODO: should this fail/error
				continue
			}
			regexpMatches[name] = string(b[matchIndexes[i1]:matchIndexes[i2]])
		}
		matchData.ScanMatch = regexpMatches
		change, newVer, err := r.action.HandleMatch(r.filename, r.conf.Name, r.conf.Source, regexpMatches["Version"], matchData)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		// write up to version field
		verI1 := r.nameInd["Version"] * 2
		verI2 := verI1 + 1
		if lastIndex < matchIndexes[verI1] {
			_, err = pw.Write(b[lastIndex:matchIndexes[verI1]])
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			lastIndex = matchIndexes[verI1]
		}
		// write changed version
		if change {
			_, err = pw.Write([]byte(newVer))
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			lastIndex = matchIndexes[verI2]
		}
	}
	// copy from last write index to end of buf
	if lastIndex < len(b) {
		_, err = pw.Write(b[lastIndex:])
		if err != nil {
			pw.CloseWithError(err)
			return
		}
	}
	pw.Close()
}

func (r *reScan) Read(b []byte) (int, error) {
	return r.outRdr.Read(b)
}

func (r *reScan) Close() error {
	_, err1 := io.Copy(io.Discard, r.outRdr)
	err2 := r.srcRdr.Close()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}
