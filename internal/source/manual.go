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
