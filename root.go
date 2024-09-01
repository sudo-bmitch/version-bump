package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sudo-bmitch/version-bump/internal/action"
	"github.com/sudo-bmitch/version-bump/internal/config"
	"github.com/sudo-bmitch/version-bump/internal/filesearch"
	"github.com/sudo-bmitch/version-bump/internal/lockfile"
	"github.com/sudo-bmitch/version-bump/internal/scan"
	"github.com/sudo-bmitch/version-bump/internal/template"
	"github.com/sudo-bmitch/version-bump/internal/version"
)

const (
	defaultConf = ".version-bump.yml"
	defaultLock = ".version-bump.lock"
	envConf     = "VERSION_BUMP_CONF"
	envLock     = "VERSION_BUMP_LOCK"
)

var rootOpts struct {
	chdir     string
	confFile  string
	lockFile  string
	dryrun    bool
	verbosity string
	logopts   []string
	format    string
	scans     []string
}

var rootCmd = &cobra.Command{
	Use:           "version-bump <cmd>",
	Short:         "Version and pinning management tool",
	Long:          `version-bump updates versions embedded in various files of your project`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// check
var checkCmd = &cobra.Command{
	Use:   "check <file list>",
	Short: "Check versions in files compared to sources",
	Long: `Check each file identified in the configuration for versions.
Compare the version to the upstream source. Report any version mismatches.
Files or directories to scan should be passed as arguments, with the current dir as the default.
By default, the current directory is changed to the location of the config file.`,
	RunE: runAction,
}

// update
var updateCmd = &cobra.Command{
	Use:   "update <file list>",
	Short: "Update versions in files using upstream sources",
	Long: `Scan each file identified in the configuration for versions.
Compare the version to the upstream source.
Update old versions, update the lock file, and report changes.
Files or directories to scan should be passed as arguments, with the current dir as the default.
By default, the current directory is changed to the location of the config file.`,
	RunE: runAction,
}

// TODO:
// set
// reset

// scan
var scanCmd = &cobra.Command{
	Use:   "scan <file list>",
	Short: "Scan versions from files into lock file",
	Long: `Scan each file identified in the configuration for versions.
Store those versions in lock file.
Files or directories to scan should be passed as arguments, with the current dir as the default.
By default, the current directory is changed to the location of the config file.`,
	RunE: runAction,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the version",
	Long:  `Show the version`,
	Args:  cobra.ExactArgs(0),
	RunE:  runVersion,
}

func init() {
	for _, cmd := range []*cobra.Command{checkCmd, scanCmd, updateCmd} {
		cmd.Flags().StringVar(&rootOpts.chdir, "chdir", "", "Changes to requested directory, defaults to config file location")
		cmd.Flags().StringVarP(&rootOpts.confFile, "conf", "c", "", "Config file to load")
		cmd.Flags().BoolVar(&rootOpts.dryrun, "dry-run", false, "Dry run")
		cmd.Flags().StringArrayVar(&rootOpts.scans, "scan", []string{}, "Only run specific scans")
		rootCmd.AddCommand(cmd)
	}

	versionCmd.Flags().StringVar(&rootOpts.format, "format", "{{printPretty .}}", "Format output with go template syntax")
	rootCmd.AddCommand(versionCmd)
}

func runAction(cmd *cobra.Command, args []string) error {
	origDir := "."
	// parse config
	conf, err := getConf()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	locks, err := getLocks()
	if err != nil {
		return fmt.Errorf("failed to load lockfile: %w", err)
	}

	// cd to appropriate location
	if !flagChanged(cmd, "chdir") {
		rootOpts.chdir = filepath.Dir(rootOpts.confFile)
	}
	if rootOpts.chdir != "." {
		origDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("unable to get current directory: %w", err)
		}
		err = os.Chdir(rootOpts.chdir)
		if err != nil {
			return fmt.Errorf("unable to change directory to %s: %w", rootOpts.chdir, err)
		}
	}

	confRun := &action.Opts{
		DryRun: rootOpts.dryrun,
		Locks:  locks,
	}
	switch cmd.Name() {
	case "check":
		confRun.Action = action.ActionCheck
	case "scan":
		confRun.Action = action.ActionScan
	case "update":
		confRun.Action = action.ActionUpdate
	default:
		return fmt.Errorf("unhandled command %s", cmd.Name())
	}
	act := action.New(confRun, *conf)

	// loop over files
	walk, err := filesearch.New(args, conf.Files)
	if err != nil {
		return err
	}
	for {
		filename, key, err := walk.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		fmt.Printf("processing file: %s for config %s\n", filename, key)
		err = procFile(filename, key, conf, act)
		if err != nil {
			return err
		}
	}
	err = act.Done()
	if err != nil {
		return err
	}
	// display changes
	for _, change := range confRun.Changes {
		fmt.Printf("Version changed: filename=%s, scan=%s, source=%s, key=%s, old=%s, new=%s\n",
			change.Filename, change.Scan, change.Source, change.Key, change.Orig, change.New)
	}

	if origDir != "." {
		err = os.Chdir(origDir)
		if err != nil {
			return fmt.Errorf("unable to change directory to %s: %w", origDir, err)
		}
	}
	if !rootOpts.dryrun {
		switch confRun.Action {
		case action.ActionScan, action.ActionUpdate:
			err = saveLocks(locks)
			if err != nil {
				return err
			}
		case action.ActionCheck:
			if len(confRun.Changes) > 0 {
				return fmt.Errorf("changes detected")
			}
		}
	}
	return nil
}

func runVersion(cmd *cobra.Command, args []string) error {
	info := version.GetInfo()
	return template.Writer(os.Stdout, rootOpts.format, info)
}

func flagChanged(cmd *cobra.Command, name string) bool {
	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		return false
	}
	return flag.Changed
}

func getConf() (*config.Config, error) {
	// if conf not provided, attempt to use env
	if rootOpts.confFile == "" {
		if file, ok := os.LookupEnv(envConf); ok {
			rootOpts.confFile = file
		}
	}
	// fall back to fixed name
	if rootOpts.confFile == "" {
		rootOpts.confFile = defaultConf
	}
	return config.LoadFile(rootOpts.confFile)
}

func getLocks() (*lockfile.Locks, error) {
	if rootOpts.lockFile == "" {
		if file, ok := os.LookupEnv(envLock); ok {
			rootOpts.lockFile = file
		}
	}
	// fall back to changing conf filename
	if rootOpts.lockFile == "" && rootOpts.confFile != "" {
		rootOpts.lockFile = strings.TrimSuffix(rootOpts.confFile, filepath.Ext(rootOpts.confFile)) + ".lock"
	}
	// fall back to fixed name
	if rootOpts.lockFile == "" {
		rootOpts.lockFile = defaultLock
	}
	l, err := lockfile.LoadFile(rootOpts.lockFile)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		l = lockfile.New()
	}
	return l, nil
}

func saveLocks(l *lockfile.Locks) error {
	if rootOpts.lockFile == "" {
		return fmt.Errorf("lockfile not defined")
	}
	return lockfile.SaveFile(rootOpts.lockFile, l)
}

func procFile(filename string, fileConf string, conf *config.Config, act *action.Action) (err error) {
	// TODO: for large files, write to a tmp file instead of using an in-memory buffer
	origBytes, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	origRdr := bytes.NewReader(origBytes)
	var curFH io.ReadCloser
	curFH = io.NopCloser(origRdr)
	defer func() {
		if curFH != nil {
			newErr := curFH.Close()
			if newErr != nil && err == nil {
				err = newErr
			}
		}
	}()
	scanFound := false
	for _, s := range conf.Files[fileConf].Scans {
		// skip scans when CLI arg requests specific scans
		if len(rootOpts.scans) > 0 && !containsStr(rootOpts.scans, s) {
			continue
		}
		if _, ok := conf.Scans[s]; !ok {
			return fmt.Errorf("missing scan config: %s, file config: %s, reading file: %s", s, fileConf, filename)
		}
		curScan, err := scan.New(*conf.Scans[s], curFH, act, filename)
		if err != nil {
			return fmt.Errorf("failed scanning file \"%s\", scan \"%s\": %w", filename, s, err)
		}
		curFH = curScan
		scanFound = true
	}
	if !scanFound {
		return nil
	}
	finalBytes, err := io.ReadAll(curFH)
	if err != nil {
		return fmt.Errorf("failed scanning file \"%s\": %w", filename, err)
	}
	// if the file was changed, output to a tmpfile and then copy/replace orig file
	if !bytes.Equal(origBytes, finalBytes) {
		dir := filepath.Dir(filename)
		tmp, err := os.CreateTemp(dir, filepath.Base(filename))
		if err != nil {
			return fmt.Errorf("unable to create temp file in %s: %w", dir, err)
		}
		tmpName := tmp.Name()
		_, err = tmp.Write(finalBytes)
		tmp.Close()
		defer func() {
			if err != nil {
				os.Remove(tmpName)
			}
		}()
		if err != nil {
			return fmt.Errorf("failed to write temp file %s: %w", tmpName, err)
		}
		// update permissions to match existing file or 0644
		mode := os.FileMode(0644)
		stat, err := os.Stat(filename)
		if err == nil && stat.Mode().IsRegular() {
			mode = stat.Mode()
		}
		if err := os.Chmod(tmpName, mode); err != nil {
			return fmt.Errorf("failed to adjust permissions on file %s: %w", filename, err)
		}
		// move temp file to target filename
		if err := os.Rename(tmpName, filename); err != nil {
			return fmt.Errorf("failed to rename file %s to %s: %w", tmpName, filename, err)
		}
	}
	return nil
}

func containsStr(strList []string, str string) bool {
	for _, cur := range strList {
		if cur == str {
			return true
		}
	}
	return false
}
