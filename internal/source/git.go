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

type gitSource struct {
	conf config.Source
}

func newGit(conf config.Source) Source {
	return gitSource{conf: conf}
}

func (g gitSource) Get(data config.SourceTmplData) (string, error) {
	confExp, err := g.conf.ExpandTemplate(data)
	if err != nil {
		return "", fmt.Errorf("failed to expand template: %w", err)
	}
	if _, ok := confExp.Args["url"]; !ok {
		return "", fmt.Errorf("url argument is required")
	}

	if confExp.Args["type"] == "tag" {
		return g.getTag(confExp)
	}
	return g.getCommit(confExp)
}

func (g gitSource) getRefs(confExp config.Source) ([]*plumbing.Reference, error) {
	rem := git.NewRemote(memory.NewStorage(), &gitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{confExp.Args["url"]},
	})
	return rem.List(&git.ListOptions{})
}

func (g gitSource) getCommit(confExp config.Source) (string, error) {
	refs, err := g.getRefs(confExp)
	if err != nil {
		return "", err
	}
	verData := VersionTmplData{
		VerMap: map[string]string{},
	}
	for _, ref := range refs {
		verData.VerMap[ref.Name().Short()] = ref.Hash().String()
	}
	if len(verData.VerMap) == 0 {
		return "", fmt.Errorf("ref %s not found on %s", confExp.Args["ref"], confExp.Args["url"])
	}
	return procResult(confExp, verData)
}

func (g gitSource) getTag(confExp config.Source) (string, error) {
	refs, err := g.getRefs(confExp)
	if err != nil {
		return "", err
	}
	verData := VersionTmplData{
		VerMap: map[string]string{},
	}
	// find matching tags
	for _, ref := range refs {
		verData.VerMap[ref.Name().Short()] = ref.Name().Short()
	}
	if len(verData.VerMap) == 0 {
		return "", fmt.Errorf("no matching tags found")
	}
	return procResult(confExp, verData)
}

func (g gitSource) Key(data config.SourceTmplData) (string, error) {
	confExp, err := g.conf.ExpandTemplate(data)
	if err != nil {
		return "", fmt.Errorf("failed to expand template: %w", err)
	}
	return confExp.Key, nil
}
