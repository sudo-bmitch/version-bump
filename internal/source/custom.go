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

func newCustom(src config.Source) (Results, error) {
	// TODO: add support for exec, bypassing the shell, which means arg values need to also support arrays
	if _, ok := src.Args[customCmd]; !ok {
		return Results{}, fmt.Errorf("custom source requires a cmd arg")
	}
	//#nosec G204 command to run is controlled by user running the command
	out, err := exec.Command("/bin/sh", "-c", src.Args[customCmd]).Output()
	if err != nil {
		return Results{}, fmt.Errorf("failed running %s: %w", src.Args[customCmd], err)
	}
	outVer := strings.TrimSpace(string(out))
	return Results{
		VerMap: map[string]string{
			outVer: outVer,
		},
	}, nil
}
