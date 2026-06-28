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

	"github.com/sudo-bmitch/version-bump/internal/config"
)

func newManual(conf config.Source) (Results, error) {
	if conf.Args == nil {
		conf.Args = map[string]string{}
	}
	if _, ok := conf.Args["Version"]; !ok {
		return Results{}, fmt.Errorf("manual source is missing a Version arg")
	}
	res := Results{
		VerMap: map[string]string{
			conf.Args["Version"]: conf.Args["Version"],
		},
	}
	return res, nil
}
