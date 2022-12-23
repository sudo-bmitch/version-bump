package source

import (
	"fmt"
	"regexp"
	"sort"

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

func (g gitSource) Get(data config.TemplateData) (string, error) {
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
	if _, ok := confExp.Args["ref"]; !ok {
		return "", fmt.Errorf("ref argument is required")
	}
	refs, err := g.getRefs(confExp)
	if err != nil {
		return "", err
	}
	for _, ref := range refs {
		if ref.Name().String() == confExp.Args["ref"] || ref.Name().Short() == confExp.Args["ref"] {
			return ref.Hash().String(), nil
		}
	}
	return "", fmt.Errorf("ref %s not found on %s", confExp.Args["ref"], confExp.Args["url"])
}

func (g gitSource) getTag(confExp config.Source) (string, error) {
	if _, ok := confExp.Args["tagExp"]; !ok {
		return "", fmt.Errorf("tagExp argument is required")
	}
	tagRE, err := regexp.Compile(confExp.Args["tagExp"])
	if err != nil {
		return "", fmt.Errorf("failed to parse tagExp regexp: %w", err)
	}
	refs, err := g.getRefs(confExp)
	if err != nil {
		return "", err
	}
	// find matching tags
	tagMatches := []string{}
	for _, ref := range refs {
		if ref.Name().IsTag() && tagRE.MatchString(ref.Name().Short()) {
			tagMatches = append(tagMatches, ref.Name().Short())
		}
	}
	if len(tagMatches) == 0 {
		return "", fmt.Errorf("no matching tags found")
	}
	// TODO: support other types of sorts (semver?)
	sort.Strings(tagMatches)
	return tagMatches[len(tagMatches)-1], nil
}

func (g gitSource) Key(data config.TemplateData) (string, error) {
	confExp, err := g.conf.ExpandTemplate(data)
	if err != nil {
		return "", fmt.Errorf("failed to expand template: %w", err)
	}
	return confExp.Key, nil
}
