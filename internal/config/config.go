// Package config defines the config file and load methods
package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// File defines a file to process for version bumps.
type File struct {
	Name       string   `yaml:"-" json:"-"`                   // Name is a filename or glob to match against
	Processors []string `yaml:"processors" json:"processors"` // Processors to run on a given file
	Scans      []string `yaml:"scans" json:"scans"`           // Deprecated: Scans are Scan names to apply to the file
}

// Processor scans with a selected scanner and source.
type Processor struct {
	Name       string            `yaml:"-" json:"-"`                   // Name of the processor
	Scan       string            `yaml:"scan" json:"scan"`             // Scanner to use
	ScanArgs   map[string]string `yaml:"scanArgs" json:"scanArgs"`     // Args to scanner
	Source     string            `yaml:"source" json:"source"`         // Source to use
	SourceArgs map[string]string `yaml:"sourceArgs" json:"sourceArgs"` // Args to source
	Key        string            `yaml:"key" json:"key"`               // Key is a unique value to store in a lock file
	Filter     Filter            `yaml:"filter" json:"filter"`         // Filter specifies which items to include from the source
	Sort       Sort              `yaml:"sort" json:"sort"`             // Sort is used to pick from multiple results
	Template   string            `yaml:"template" json:"template"`     // Template is used to output the version
}

// Scan defines how to search a file for versions.
type Scan struct {
	Name   string            `yaml:"-" json:"-"`           // Name is the name of the scan entry
	Type   string            `yaml:"type" json:"type"`     // Type is the method for scanning the file
	Args   map[string]string `yaml:"args" json:"args"`     // Args provide additional options used by scanners, sources, and templating
	Source string            `yaml:"source" json:"source"` // Deprecated: Source is the name of the source for updating the version
}

// Source defines how to get the current version.
type Source struct {
	Name     string            `yaml:"-" json:"-"`               // Name is the name of the source entry
	Type     string            `yaml:"type" json:"type"`         // Type is the method used to query the source
	Args     map[string]string `yaml:"args" json:"args"`         // Args provide additional options used by sources
	Key      string            `yaml:"key" json:"key"`           // Deprecated: Key is a unique value to store in a lock file
	Filter   Filter            `yaml:"filter" json:"filter"`     // Deprecated: Filter specifies which items to include from the source
	Sort     Sort              `yaml:"sort" json:"sort"`         // Deprecated: Sort is used to pick from multiple results
	Template string            `yaml:"template" json:"template"` // Deprecated: Template is used to output the version
}

// Filter defines how items are filtered in from the source.
// By default, all items are included.
type Filter struct {
	Expr string `yaml:"expr" json:"expr"`
	// Template string `yaml:"template" json:"template"` // Deprecated: removed after no usage found
}

// Sort defines how multiple results should be filtered and sorted.
// By default, sort returns the 0 offset of a descending sort.
type Sort struct {
	Method string `yaml:"method" json:"method"`
	Asc    bool   `yaml:"asc" json:"asc"`
	Offset int    `yaml:"offset" json:"offset"`
}

// Config contains the configuration options for the project
type Config struct {
	Version    int                   `yaml:"version" json:"version"`
	Files      map[string]*File      `yaml:"files" json:"files"`
	Processors map[string]*Processor `yaml:"processors" json:"processors"`
	Scans      map[string]*Scan      `yaml:"scans" json:"scans"`
	Sources    map[string]*Source    `yaml:"sources" json:"sources"`
}

// New creates an empty config
func New() *Config {
	return &Config{
		Files:      map[string]*File{},
		Processors: map[string]*Processor{},
		Scans:      map[string]*Scan{},
		Sources:    map[string]*Source{},
	}
}

// LoadReader imports a config from a reader.
func LoadReader(r io.Reader) (*Config, error) {
	c := New()
	err := yaml.NewDecoder(r).Decode(c)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if c.Version > 1 {
		return nil, fmt.Errorf("unsupported config version: %d", c.Version)
	}
	// set name on each entry
	for k := range c.Files {
		c.Files[k].Name = k
	}
	for k := range c.Processors {
		c.Processors[k].Name = k
	}
	for k := range c.Scans {
		c.Scans[k].Name = k
	}
	for k := range c.Sources {
		c.Sources[k].Name = k
	}
	// automatically convert older format files without processors to use the scans and sources
	convertScans := map[string]bool{}
	for k := range c.Files {
		for _, s := range c.Files[k].Scans {
			convertScans[s] = true
			c.Files[k].Processors = append(c.Files[k].Processors, s)
		}
	}
	for scanName := range convertScans {
		scan, ok := c.Scans[scanName]
		if !ok || scan == nil {
			return c, fmt.Errorf("invalid config reference to missing scan: %s", scanName)
		}
		source, ok := c.Sources[scan.Source]
		if !ok || source == nil {
			return c, fmt.Errorf("invalid config reference to missing source: %s", scan.Source)
		}
		c.Processors[scanName] = &Processor{
			Name:     scanName,
			Scan:     scanName,
			Source:   source.Name,
			Key:      source.Key,
			Filter:   source.Filter,
			Sort:     source.Sort,
			Template: source.Template,
		}
	}
	return c, nil
}

// LoadFile imports a config from a filename
func LoadFile(filename string) (*Config, error) {
	//#nosec G304 file to read is controlled by user running the command
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return LoadReader(file)
}

func (p Processor) Clone() Processor {
	return Processor{
		Name:       p.Name,
		Scan:       p.Scan,
		ScanArgs:   mapStrStrClone(p.ScanArgs),
		Source:     p.Source,
		SourceArgs: mapStrStrClone(p.SourceArgs),
		Key:        p.Key,
		Filter:     p.Filter,
		Sort:       p.Sort,
		Template:   p.Template,
	}
}

func (p Processor) Equal(p2 Processor) bool {
	if p.Name != p2.Name ||
		p.Scan != p2.Scan ||
		p.Source != p2.Source ||
		p.Key != p2.Key ||
		p.Sort != p2.Sort ||
		p.Filter != p2.Filter ||
		p.Template != p2.Template ||
		!eqStrMaps(p.ScanArgs, p2.ScanArgs) ||
		!eqStrMaps(p.SourceArgs, p2.SourceArgs) {
		return false
	}
	return true
}

func (s Scan) Clone() Scan {
	return Scan{
		Name: s.Name,
		Type: s.Type,
		Args: mapStrStrClone(s.Args),
	}
}

func (s Scan) Equal(s2 Scan) bool {
	if s.Name != s2.Name ||
		s.Type != s2.Type ||
		!eqStrMaps(s.Args, s2.Args) {
		return false
	}
	return true
}

func (s Source) Clone() Source {
	return Source{
		Name: s.Name,
		Type: s.Type,
		Args: mapStrStrClone(s.Args),
	}
}

func (s Source) Equal(s2 Source) bool {
	if s.Name != s2.Name ||
		s.Type != s2.Type ||
		!eqStrMaps(s.Args, s2.Args) {
		return false
	}
	return true
}

func eqStrSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func eqStrMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		if bv, ok := b[k]; !ok || av != bv {
			return false
		}
	}
	return true
}

func mapStrStrClone(s map[string]string) map[string]string {
	c := map[string]string{}
	for k, v := range s {
		c[k] = v
	}
	return c
}
