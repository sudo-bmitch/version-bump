package source

import (
	"fmt"
	"os/exec"
)

const (
	customCmd = "cmd"
)

type custom struct{}

func (c custom) Get(args map[string]string) (string, error) {
	// TODO: add support for exec, bypassing the shell, which means arg values need to also support arrays
	if _, ok := args[customCmd]; !ok {
		return "", fmt.Errorf("custom source requires a cmd arg")
	}
	out, err := exec.Command("/bin/sh", "-c", args[customCmd]).Output()
	if err != nil {
		return "", fmt.Errorf("failed running %s: %w", args[customCmd], err)
	}
	return string(out), nil
}
