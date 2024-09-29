package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sudo-bmitch/version-bump/internal/config"
	"github.com/sudo-bmitch/version-bump/internal/filesearch"
	"github.com/sudo-bmitch/version-bump/internal/lockfile"
	"github.com/sudo-bmitch/version-bump/internal/processor"
	"github.com/sudo-bmitch/version-bump/internal/template"
	"github.com/sudo-bmitch/version-bump/internal/version"
)

const (
	defaultConf = ".version-bump.yml"
	defaultLock = ".version-bump.lock"
	envConf     = "VERSION_BUMP_CONF"
	envLock     = "VERSION_BUMP_LOCK"
)

type cliOpts struct {
	chdir      string
	confFile   string
	lockFile   string
	dryrun     bool
	prune      bool
	format     string
	processors []string
	scans      []string
	// TODO: setup logging
	// verbosity string
	// logopts   []string
}

func NewRootCmd() *cobra.Command {
	var rootOpts cliOpts
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
		RunE: rootOpts.runAction,
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
		RunE: rootOpts.runAction,
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
		RunE: rootOpts.runAction,
	}

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show the version",
		Long:  `Show the version`,
		Args:  cobra.ExactArgs(0),
		RunE:  rootOpts.runVersion,
	}

	for _, cmd := range []*cobra.Command{checkCmd, scanCmd, updateCmd} {
		cmd.Flags().StringVar(&rootOpts.chdir, "chdir", "", "Changes to requested directory, defaults to config file location")
		cmd.Flags().StringVarP(&rootOpts.confFile, "conf", "c", "", "Config file to load")
		cmd.Flags().BoolVar(&rootOpts.dryrun, "dry-run", false, "Dry run")
		cmd.Flags().BoolVar(&rootOpts.prune, "prune", false, "Prune unused entries (default to true when no files are listed)")
		cmd.Flags().StringArrayVar(&rootOpts.processors, "processor", []string{}, "Only run specific processors")
		cmd.Flags().StringArrayVar(&rootOpts.scans, "scan", []string{}, "Deprecated: Only run specific scans")
		_ = cmd.Flags().MarkHidden("scan")
		rootCmd.AddCommand(cmd)
	}

	versionCmd.Flags().StringVar(&rootOpts.format, "format", "{{printPretty .}}", "Format output with go template syntax")
	rootCmd.AddCommand(versionCmd)

	return rootCmd
}

func (cli *cliOpts) runAction(cmd *cobra.Command, args []string) error {
	origDir := "."
	ctx := cmd.Context()
	// validate inputs
	if len(cli.scans) > 0 {
		// TODO: use logging library
		_, _ = cmd.ErrOrStderr().Write([]byte("warning: scan flag is deprecated, switch to processor"))
		cli.processors = append(cli.processors, cli.scans...)
	}
	// parse config
	conf, err := cli.getConf()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	locks, err := cli.locksLoad()
	if err != nil {
		return fmt.Errorf("failed to load lockfile: %w", err)
	}
	if len(args) == 0 && !flagChanged(cmd, "prune") {
		cli.prune = true
	}

	// cd to appropriate location
	if !flagChanged(cmd, "chdir") {
		cli.chdir = filepath.Dir(cli.confFile)
	}
	if cli.chdir != "." {
		origDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("unable to get current directory: %w", err)
		}
		err = os.Chdir(cli.chdir)
		if err != nil {
			return fmt.Errorf("unable to change directory to %s: %w", cli.chdir, err)
		}
	}
	action := cmd.Name()
	switch cmd.Name() {
	case "check", "scan", "update":
	default:
		return fmt.Errorf("unhandled command %s", cmd.Name())
	}

	// loop over files
	walk, err := filesearch.New(args, conf.Files)
	if err != nil {
		return err
	}
	changes := []*processor.Change{}
	for {
		filename, fileKey, err := walk.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		fmt.Printf("processing file: %s for config %s\n", filename, fileKey)
		curChanges, err := cli.procFile(ctx, filename, fileKey, conf, action, locks)
		if err != nil {
			return err
		}
		if len(curChanges) > 0 {
			changes = append(changes, curChanges...)
		}
	}
	// display changes
	for _, change := range changes {
		fmt.Printf("Version changed: filename=%s, processor=%s, key=%s, old=%s, new=%s\n",
			change.Filename, change.Processor, change.Key, change.Orig, change.New)
	}

	if origDir != "." {
		err = os.Chdir(origDir)
		if err != nil {
			return fmt.Errorf("unable to change directory to %s: %w", origDir, err)
		}
	}
	if !cli.dryrun {
		switch action {
		case "scan", "update":
			err = cli.locksSave(locks, cli.prune)
			if err != nil {
				return err
			}
		case "check":
			if len(changes) > 0 {
				return fmt.Errorf("changes detected")
			}
		}
	}
	return nil
}

func (cli *cliOpts) runVersion(cmd *cobra.Command, args []string) error {
	info := version.GetInfo()
	return template.Writer(cmd.OutOrStdout(), cli.format, info)
}

func (cli *cliOpts) getConf() (*config.Config, error) {
	// if conf not provided, attempt to use env
	if cli.confFile == "" {
		if file, ok := os.LookupEnv(envConf); ok {
			cli.confFile = file
		}
	}
	// fall back to fixed name
	if cli.confFile == "" {
		cli.confFile = defaultConf
	}
	return config.LoadFile(cli.confFile)
}

func (cli *cliOpts) locksLoad() (*lockfile.Locks, error) {
	if cli.lockFile == "" {
		if file, ok := os.LookupEnv(envLock); ok {
			cli.lockFile = file
		}
	}
	// fall back to changing conf filename
	if cli.lockFile == "" && cli.confFile != "" {
		cli.lockFile = strings.TrimSuffix(cli.confFile, filepath.Ext(cli.confFile)) + ".lock"
	}
	// fall back to fixed name
	if cli.lockFile == "" {
		cli.lockFile = defaultLock
	}
	l, err := lockfile.LoadFile(cli.lockFile)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		l = lockfile.New()
	}
	return l, nil
}

func (cli *cliOpts) locksSave(l *lockfile.Locks, used bool) error {
	if cli.lockFile == "" {
		return fmt.Errorf("lockfile not defined")
	}
	return l.SaveFile(cli.lockFile, used)
}

type procFileChan struct {
	changes []*processor.Change
	err     error
}

func (cli *cliOpts) procFile(ctx context.Context, filename string, fileKey string, conf *config.Config, action string, locks *lockfile.Locks) ([]*processor.Change, error) {
	// TODO: for large files, write to a tmp file instead of using an in-memory buffer
	origBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	bRdr := bytes.NewReader(origBytes)
	var rdr io.ReadCloser
	rdr = io.NopCloser(bRdr)
	procCount := 0
	procResult := make(chan procFileChan)
	for _, p := range conf.Files[fileKey].Processors {
		// skip scans when CLI arg requests specific scans
		if len(cli.processors) > 0 && !containsStr(cli.processors, p) {
			continue
		}
		if _, ok := conf.Processors[p]; !ok {
			return nil, fmt.Errorf("missing processor config: %s, file config: %s, reading file: %s", p, fileKey, filename)
		}
		procCount++
		pr, pw := io.Pipe()
		// run processor in goroutine to read and write detected changes
		go func(procName string, r io.ReadCloser, w io.WriteCloser) {
			changes, pErr := processor.Process(ctx, *conf, procName, filename, r, w, locks)
			rErr := r.Close()
			wErr := w.Close()
			var err error
			if pErr != nil {
				if rErr != nil || wErr != nil {
					err = errors.Join(pErr, rErr, wErr)
				} else {
					err = pErr
				}
			}
			procResult <- procFileChan{
				changes: changes,
				err:     err,
			}
		}(p, rdr, pw)
		// increment reader
		rdr = pr
	}
	if procCount == 0 {
		return nil, nil
	}
	// TODO: discard when not updating file, and consider outputting directly to the temp file for large files
	finalBytes, err := io.ReadAll(rdr)
	if err != nil {
		return nil, fmt.Errorf("failed scanning file \"%s\": %w", filename, err)
	}
	err = rdr.Close()
	if err != nil {
		return nil, fmt.Errorf("failed closing reader while scanning \"%s\": %w", filename, err)
	}
	// check results from goroutines
	errs := []error{}
	changes := []*processor.Change{}
	for i := 0; i < procCount; i++ {
		curResult := <-procResult
		if curResult.err != nil {
			errs = append(errs, curResult.err)
		}
		if len(curResult.changes) > 0 {
			changes = append(changes, curResult.changes...)
		}
	}
	if len(errs) > 0 {
		return changes, errors.Join(errs...)
	}
	// if the file was changed and updates are being performed, output to a tmpfile and then copy/replace orig file
	if !cli.dryrun && action == "update" && !bytes.Equal(origBytes, finalBytes) {
		dir := filepath.Dir(filename)
		tmp, err := os.CreateTemp(dir, filepath.Base(filename))
		if err != nil {
			return nil, fmt.Errorf("unable to create temp file in %s: %w", dir, err)
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
			return nil, fmt.Errorf("failed to write temp file %s: %w", tmpName, err)
		}
		// update permissions to match existing file or 0644
		mode := os.FileMode(0644)
		stat, err := os.Stat(filename)
		if err == nil && stat.Mode().IsRegular() {
			mode = stat.Mode()
		}
		if err := os.Chmod(tmpName, mode); err != nil {
			return nil, fmt.Errorf("failed to adjust permissions on file %s: %w", filename, err)
		}
		// move temp file to target filename
		if err := os.Rename(tmpName, filename); err != nil {
			return nil, fmt.Errorf("failed to rename file %s to %s: %w", tmpName, filename, err)
		}
	}
	return changes, nil
}

func containsStr(strList []string, str string) bool {
	for _, cur := range strList {
		if cur == str {
			return true
		}
	}
	return false
}

func flagChanged(cmd *cobra.Command, name string) bool {
	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		return false
	}
	return flag.Changed
}
