package source

import (
	"context"
	"fmt"

	"github.com/regclient/regclient"
	"github.com/regclient/regclient/types/ref"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

type registry struct {
	rc *regclient.RegClient
}

func newRegistry(conf config.Source) (Results, error) {
	// TODO: cache regclient instance to only create it once
	rc := regclient.New(
		regclient.WithDockerCreds(),
	)
	r := registry{
		rc: rc,
	}
	if conf.Args["type"] == "tag" {
		return r.getTag(conf)
	}
	// default request is for a digest
	return r.getDigest(conf)
}

func (r registry) getTag(conf config.Source) (Results, error) {
	repo, ok := conf.Args["repo"]
	if !ok {
		return Results{}, fmt.Errorf("repo not defined")
	}
	repoRef, err := ref.New(repo)
	if err != nil {
		return Results{}, fmt.Errorf("failed to parse repo: %w", err)
	}
	tags, err := r.rc.TagList(context.Background(), repoRef)
	if err != nil {
		return Results{}, fmt.Errorf("failed to list tags: %w", err)
	}
	res := Results{
		VerMap: map[string]string{},
	}
	for _, tag := range tags.Tags {
		res.VerMap[tag] = tag
	}
	if len(res.VerMap) == 0 {
		return Results{}, fmt.Errorf("no matching tags found")
	}
	return res, nil
}

func (r registry) getDigest(conf config.Source) (Results, error) {
	image, ok := conf.Args["image"]
	if !ok {
		return Results{}, fmt.Errorf("image not defined")
	}
	imageRef, err := ref.New(image)
	if err != nil {
		return Results{}, fmt.Errorf("failed to parse image: %w", err)
	}
	m, err := r.rc.ManifestHead(context.Background(), imageRef, regclient.WithManifestRequireDigest())
	if err != nil {
		return Results{}, fmt.Errorf("failed to query image: %w", err)
	}
	dig := m.GetDescriptor().Digest.String()
	res := Results{
		VerMap: map[string]string{
			dig: dig,
		},
	}
	return res, nil
}
