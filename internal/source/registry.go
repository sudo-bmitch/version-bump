package source

import (
	"context"
	"fmt"
	"regexp"

	"github.com/regclient/regclient"
	"github.com/regclient/regclient/types/ref"
	"github.com/sudo-bmitch/version-bump/internal/config"
)

const ()

type registry struct {
	conf config.Source
	rc   *regclient.RegClient
}

func newRegistry(conf config.Source) Source {
	rc := regclient.New(
		regclient.WithDockerCreds(),
	)
	return registry{conf: conf, rc: rc}
}

func (r registry) Get(data config.SourceTmplData) (string, error) {
	confExp, err := r.conf.ExpandTemplate(data)
	if err != nil {
		return "", fmt.Errorf("failed to expand template: %w", err)
	}
	if confExp.Args["type"] == "tag" {
		return r.getTag(confExp)
	}
	// default request is for a digest
	return r.getDigest(confExp)
}

func (r registry) getTag(confExp config.Source) (string, error) {
	repo, ok := confExp.Args["repo"]
	if !ok {
		return "", fmt.Errorf("repo not defined")
	}
	repoRef, err := ref.New(repo)
	if err != nil {
		return "", fmt.Errorf("failed to parse repo: %w", err)
	}
	// TODO: move filtering into procResult
	tagExp, ok := confExp.Args["tagExp"]
	if !ok {
		return "", fmt.Errorf("tagExp not defined")
	}
	tagRE, err := regexp.Compile(tagExp)
	if err != nil {
		return "", fmt.Errorf("failed to parse tagExp: %w", err)
	}
	tags, err := r.rc.TagList(context.Background(), repoRef)
	if err != nil {
		return "", fmt.Errorf("failed to list tags: %w", err)
	}
	verData := config.VersionTmplData{
		VerMap: map[string]string{},
	}
	for _, tag := range tags.Tags {
		if tagRE.Match([]byte(tag)) {
			verData.VerMap[tag] = tag
		}
	}
	if len(verData.VerMap) == 0 {
		return "", fmt.Errorf("no matching tags found")
	}
	return procResult(confExp, verData)
}

func (r registry) getDigest(confExp config.Source) (string, error) {
	image, ok := confExp.Args["image"]
	if !ok {
		return "", fmt.Errorf("image not defined")
	}
	imageRef, err := ref.New(image)
	if err != nil {
		return "", fmt.Errorf("failed to parse image: %w", err)
	}
	m, err := r.rc.ManifestHead(context.Background(), imageRef)
	if err != nil {
		return "", fmt.Errorf("failed to query image: %w", err)
	}
	verData := config.VersionTmplData{
		Version: m.GetDescriptor().Digest.String(),
	}
	return procResult(confExp, verData)
}

func (r registry) Key(data config.SourceTmplData) (string, error) {
	confExp, err := r.conf.ExpandTemplate(data)
	if err != nil {
		return "", fmt.Errorf("failed to expand template: %w", err)
	}
	return confExp.Key, nil
}
