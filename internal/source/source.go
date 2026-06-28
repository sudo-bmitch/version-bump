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

// Package source is used to fetch the latest version information from upstream
package source

import (
	"fmt"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

var sourceTypes map[string]func(config.Source) (Results, error) = map[string]func(config.Source) (Results, error){
	"custom":     newCustom,
	"git":        newGit,
	"manual":     newManual,
	"registry":   newRegistry,
	"gh-release": newGHRelease,
	// TODO: add url (headers, parse json/yaml, parse regex), github release
}

// Results are returned by a source for a given request.
type Results struct {
	VerMap  map[string]string // list of keys and values for a given source, e.g. tag=digest
	VerMeta map[string]any    // additional metadata specific to each source, e.g. GitHub release metadata
}

func Get(src config.Source) (Results, error) {
	if srcFn, ok := sourceTypes[src.Type]; ok {
		return srcFn(src)
	}
	return Results{}, fmt.Errorf("source type not found: %s", src.Type)
}
