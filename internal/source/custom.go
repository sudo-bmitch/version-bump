package source

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

const (
	customCmd = "cmd"
)

type custom struct {
	conf config.Source
}

func newCustom(conf config.Source) Source {
	return custom{conf: conf}
}

func (c custom) Get(data interface{}) (string, error) {
	// TODO: add templating support
	// TODO: add support for exec, bypassing the shell, which means arg values need to also support arrays
	if _, ok := c.conf.Args[customCmd]; !ok {
		return "", fmt.Errorf("custom source requires a cmd arg")
	}
	out, err := exec.Command("/bin/sh", "-c", c.conf.Args[customCmd]).Output()
	if err != nil {
		return "", fmt.Errorf("failed running %s: %w", c.conf.Args[customCmd], err)
	}
	outS := strings.TrimSpace(string(out))
	return outS, nil
}

func (c custom) Key(data interface{}) (string, error) {
	// TODO: add templating support
	return c.conf.Key, nil
}
