// Package config defines the config file and load methods
package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sudo-bmitch/version-bump/internal/template"
	"gopkg.in/yaml.v3"
)

// File defines a file to process for version bumps
type File struct {
	Name  string   `yaml:"-" json:"-"`         // Name is a filename or glob to match against
	Scans []string `yaml:"scans" json:"scans"` // Scans are Scan names to apply to the file
}

// Scan defines how to search a file for versions to bump
type Scan struct {
	Name   string            `yaml:"-" json:"-"`           // Name is the name of the scan entry
	Type   string            `yaml:"type" json:"type"`     // Type is the method for scanning the file
	Source string            `yaml:"source" json:"source"` // Source is the name of the source for updating the version
	Args   map[string]string `yaml:"args" json:"args"`     // Args provide additional options used by scanners, sources, and templating
}

// Source defines how to get the latest version
type Source struct {
	Name     string            `yaml:"-" json:"-"`               // Name is the name of the source entry
	Type     string            `yaml:"type" json:"type"`         // Type is the method used to query the source
	Key      string            `yaml:"key" json:"key"`           // Key is a unique value to store with the source and version in a lock file
	Filter   SourceFilter      `yaml:"filter" json:"filter"`     // Filter specifies which items to include from the source
	Sort     SourceSort        `yaml:"sort" json:"sort"`         // Sort is used to pick from multiple results
	Template string            `yaml:"template" json:"template"` // Template is used to output the version
	Args     map[string]string `yaml:"args" json:"args"`         // Args provide additional options used by sources
	// TODO: delete exec?
	Exec []string `yaml:"exec" json:"exec"` // Exec defines a command to run for custom sources
}

// SourceFilter defines how items are filtered in from the source.
// By default, all items are included.
type SourceFilter struct {
	Expr     string `yaml:"expr" json:"expr"`
	Template string `yaml:"template" json:"template"`
}

// SourceSort defines how multiple results should be filtered and sorted.
// By default, sort returns the 0 offset of a descending sort.
type SourceSort struct {
	Method string `yaml:"method" json:"method"`
	Asc    bool   `yaml:"asc" json:"asc"`
	Offset uint   `yaml:"offset" json:"offset"`
}

// Script defines an addition command to run
type Script struct {
	Name string   `yaml:"-" json:"-"`       // Name is the name of the script
	Step string   `yaml:"step" json:"step"` // Step is when to execute the script, pre-check, post-check, pre-update, post-update
	Exec []string `yaml:"exec" json:"exec"` // Exec defines the command to run for this script
}

// Config contains the configuration options for the project
type Config struct {
	Version int                `yaml:"version" json:"version"`
	Files   map[string]*File   `yaml:"files" json:"files"`
	Scans   map[string]*Scan   `yaml:"scans" json:"scans"`
	Sources map[string]*Source `yaml:"sources" json:"sources"`
	Scripts map[string]*Script `yaml:"scripts" json:"scripts"`
}

// SourceTmplData is passed from the scan to the source for performing a version lookup
type SourceTmplData struct {
	Filename   string            // name of the file being updated
	ScanArgs   map[string]string // args to the scan
	ScanMatch  map[string]string // result of a match
	SourceArgs map[string]string // args to the source
}

// New creates an empty config
func New() *Config {
	return &Config{
		Files:   map[string]*File{},
		Scans:   map[string]*Scan{},
		Sources: map[string]*Source{},
		Scripts: map[string]*Script{},
	}
}

// LoadReader imports a config from a reader
func LoadReader(r io.Reader) (*Config, error) {
	c := New()
	err := yaml.NewDecoder(r).Decode(c)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if c.Version > 1 {
		return nil, fmt.Errorf("unsupported config version: %d", c.Version)
	}
	for k := range c.Files {
		c.Files[k].Name = k
	}
	for k := range c.Scans {
		c.Scans[k].Name = k
	}
	for k := range c.Sources {
		c.Sources[k].Name = k
	}
	for k := range c.Scripts {
		c.Scripts[k].Name = k
	}
	return c, nil
}

// LoadFile imports a config from a filename
func LoadFile(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return LoadReader(file)
}

func (s Source) ExpandTemplate(data interface{}) (Source, error) {
	sExp := Source{
		Name:     s.Name,
		Type:     s.Type,
		Filter:   s.Filter,
		Sort:     s.Sort,
		Template: s.Template,
		Args:     map[string]string{},
	}
	var err error
	sExp.Key, err = template.String(s.Key, data)
	if err != nil {
		return sExp, fmt.Errorf("failed to parse template for source \"%s\": \"%s\": %w", s.Name, s.Key, err)
	}
	for k := range s.Args {
		sExp.Args[k], err = template.String(s.Args[k], data)
		if err != nil {
			return sExp, fmt.Errorf("failed to parse template for source \"%s\": \"%s\": %w", s.Name, s.Args[k], err)
		}
	}
	sExp.Filter.Expr, err = template.String(s.Filter.Expr, data)
	if err != nil {
		return sExp, fmt.Errorf("failed to parse template for source \"%s\": \"%s\": %w", s.Name, s.Filter.Expr, err)
	}
	// TODO: support exec field too?
	return sExp, nil
}
