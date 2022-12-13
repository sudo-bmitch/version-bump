package source

import (
	"fmt"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

type manual struct {
	conf config.Source
}

func newManual(conf config.Source) Source {
	if conf.Args == nil {
		conf.Args = map[string]string{}
	}
	if _, ok := conf.Args["Version"]; !ok {
		conf.Args["Version"] = "{{ .ScanMatch.Version }}"
	}
	return manual{conf: conf}
}

func (m manual) Get(data config.TemplateData) (string, error) {
	confExp, err := m.conf.ExpandTemplate(data)
	if err != nil {
		return "", fmt.Errorf("failed to expand template: %w", err)
	}
	if _, ok := confExp.Args["Version"]; !ok {
		return "", fmt.Errorf("manual source is missing a version arg")
	}
	return confExp.Args["Version"], nil
}

func (m manual) Key(data config.TemplateData) (string, error) {
	confExp, err := m.conf.ExpandTemplate(data)
	if err != nil {
		return "", fmt.Errorf("failed to expand template: %w", err)
	}
	return confExp.Key, nil
}
