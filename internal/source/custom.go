// Copyright the version-bump contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
