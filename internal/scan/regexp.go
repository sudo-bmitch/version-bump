package scan

import (
	"context"
	"fmt"
	"io"
	"regexp"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

const (
	regexpArgRE   = "regexp"
	regexpVersion = "Version"
)

// runREScan executes a scanner based on a regexp.
func runREScan(ctx context.Context, conf config.Scan, filename string, r io.Reader, w io.Writer, getVer func(curVer string, args map[string]string) (string, error)) error {
	// validate config, extract and compile regexp
	if _, ok := conf.Args[regexpArgRE]; !ok {
		return fmt.Errorf("scan regexp arg is missing for %s", conf.Name)
	}
	re, err := regexp.Compile("(?m)" + conf.Args[regexpArgRE])
	if err != nil {
		return fmt.Errorf("scan regexp does not compile for %s: %s: %w", conf.Name, conf.Args[regexpArgRE], err)
	}
	// extract index of each subexp
	subNames := re.SubexpNames()
	nameInd := map[string]int{}
	for i, name := range subNames {
		nameInd[name] = i
	}
	// verify regexp contains a "Version" match
	if _, ok := nameInd[regexpVersion]; !ok {
		return fmt.Errorf("scan regexp is missing Version submatch (i.e. \"(?P<Version>\\d+)\") for %s: %s", conf.Name, conf.Args[regexpArgRE])
	}

	lastIndex := 0
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	// scan buf for all regexp matches
	matchIndexList := re.FindAllSubmatchIndex(b, -1)

	// for each result, build arg map, call action, handle response
	for _, matchIndexes := range matchIndexList {
		regexpMatches := map[string]string{}
		for name, i := range nameInd {
			i1, i2 := i*2, (i*2)+1
			if i2 >= len(matchIndexes) {
				return fmt.Errorf("regexp matches did not match compiled named field list (%d >= %d): %s", i2, len(matchIndexes), conf.Args[regexpArgRE])
			}
			regexpMatches[name] = string(b[matchIndexes[i1]:matchIndexes[i2]])
		}
		curVer := regexpMatches[regexpVersion]
		newVer, err := getVer(curVer, regexpMatches)
		if err != nil {
			return err
		}
		// write up to version field
		verI1 := nameInd[regexpVersion] * 2
		verI2 := verI1 + 1
		if lastIndex < matchIndexes[verI1] {
			_, err = w.Write(b[lastIndex:matchIndexes[verI1]])
			if err != nil {
				return err
			}
			lastIndex = matchIndexes[verI1]
		}
		if lastIndex > matchIndexes[verI1] {
			return fmt.Errorf("regexp match went backwards in the stream (%d > %d): %s", lastIndex, matchIndexes[verI1], conf.Args[regexpArgRE])
		}
		// write changed version
		if curVer != newVer {
			_, err = w.Write([]byte(newVer))
			if err != nil {
				return err
			}
			lastIndex = matchIndexes[verI2]
		}
	}
	// copy from last write index to end of buf
	if lastIndex < len(b) {
		_, err = w.Write(b[lastIndex:])
		if err != nil {
			return err
		}
	}
	return nil
}
