package source

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

const ()

func newGit(conf config.Source) (Results, error) {
	if _, ok := conf.Args["url"]; !ok {
		return Results{}, fmt.Errorf("url argument is required")
	}
	if conf.Args["type"] == "tag" {
		return gitTag(conf)
	}
	return gitCommit(conf)
}

func gitRefs(conf config.Source) ([]*plumbing.Reference, error) {
	rem := git.NewRemote(memory.NewStorage(), &gitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{conf.Args["url"]},
	})
	return rem.List(&git.ListOptions{
		PeelingOption: git.AppendPeeled,
	})
}

func gitCommit(conf config.Source) (Results, error) {
	refs, err := gitRefs(conf)
	if err != nil {
		return Results{}, err
	}
	res := Results{
		VerMap: map[string]string{},
	}
	// make a map of tags to hashes
	for _, ref := range refs {
		res.VerMap[ref.Name().Short()] = ref.Hash().String()
	}
	// loop over the map entries to prefer the peeled hash (underlying commit vs signed/annotated tag hash)
	for k := range res.VerMap {
		if _, ok := res.VerMap[k+"^{}"]; ok {
			res.VerMap[k] = res.VerMap[k+"^{}"]
			delete(res.VerMap, k+"^{}")
		}
	}
	if len(res.VerMap) == 0 {
		return Results{}, fmt.Errorf("no tagged commits found on %s", conf.Args["url"])
	}
	return res, nil
}

func gitTag(conf config.Source) (Results, error) {
	refs, err := gitRefs(conf)
	if err != nil {
		return Results{}, err
	}
	res := Results{
		VerMap: map[string]string{},
	}
	// make a map of tags
	for _, ref := range refs {
		res.VerMap[ref.Name().Short()] = ref.Name().Short()
	}
	if len(res.VerMap) == 0 {
		return Results{}, fmt.Errorf("no tagged commits found on %s", conf.Args["url"])
	}
	return res, nil
}
