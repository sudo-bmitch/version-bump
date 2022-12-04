package main

import (
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
}

var rootCmd = &cobra.Command{
	Use:           "version-bump <cmd>",
	Short:         "Version and pinning management tool",
	Long:          `version-bump updates versions embedded in various files of your project`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// TODO:
// check
// update
// set
// reset

// scan
var scanCmd = &cobra.Command{
	Use:   "scan <file list>",
	Short: "Scan for versions in files",
	Long: `Scan each file identified in the configuration for versions.
Store those versions in lock file.
Files or directories to scan should be passed as arguments, with the current dir as the default.
By default, the current directory is changed to the location of the config file.`,
	RunE: runScan,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the version",
	Long:  `Show the version`,
	Args:  cobra.ExactArgs(0),
	RunE:  runVersion,
}

func init() {
	scanCmd.Flags().StringVarP(&rootOpts.chdir, "chdir", "", "", "Changes to requested directory, defaults to config file location")
	scanCmd.Flags().StringVarP(&rootOpts.confFile, "conf", "c", "", "Config file to load")
	scanCmd.Flags().BoolVarP(&rootOpts.dryrun, "dry-run", "", false, "Dry run")

	versionCmd.Flags().StringVarP(&rootOpts.format, "format", "", "{{printPretty .}}", "Format output with go template syntax")

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(versionCmd)
}

func runScan(cmd *cobra.Command, args []string) error {
	origDir := "."
	// parse config
	conf, err := getConf()
	if err != nil {
		return err
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
			return err
		}
		err = os.Chdir(rootOpts.chdir)
		if err != nil {
			return err
		}
	}

	confRun := &action.Opts{
		Action: action.ActionScan,
		DryRun: rootOpts.dryrun,
		Locks:  locks,
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

	if origDir != "." {
		err = os.Chdir(origDir)
		if err != nil {
			return err
		}
	}
	err = saveLocks(locks)
	if err != nil {
		return err
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
	var lastCloser io.Closer // closing the last reader propagates through all readers
	defer func() {
		if lastCloser != nil {
			newErr := lastCloser.Close()
			if newErr != nil && err == nil {
				err = newErr
			}
		}
	}()
	fh, err := os.Open(filename)
	if err != nil {
		return err
	}
	lastCloser = fh
	var curFH io.ReadCloser
	curFH = fh
	for _, s := range conf.Files[fileConf].Scans {
		if _, ok := conf.Scans[s]; !ok {
			return fmt.Errorf("missing scan config: %s, file config: %s, reading file: %s", s, fileConf, filename)
		}
		curScan, err := scan.New(*conf.Scans[s], curFH, act, filename)
		if err != nil {
			return err
		}
		lastCloser = curScan
		curFH = curScan
	}
	_, err = io.Copy(io.Discard, curFH)
	return err
}
