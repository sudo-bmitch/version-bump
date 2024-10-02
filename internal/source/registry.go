package source

import (
	"context"
	"fmt"
	"sync"

	"github.com/regclient/regclient"
	"github.com/regclient/regclient/types/ref"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

var registry struct {
	once        sync.Once
	rc          *regclient.RegClient
	mu          sync.Mutex // mutex for cache access
	cacheTags   map[string]*Results
	cacheDigest map[string]*Results
}

func newRegistry(conf config.Source) (Results, error) {
	registry.once.Do(func() {
		registry.rc = regclient.New(
			regclient.WithDockerCreds(),
			regclient.WithUserAgent("sudo-bmitch/version-bump"),
		)
		registry.cacheDigest = map[string]*Results{}
		registry.cacheTags = map[string]*Results{}
	})
	if conf.Args["type"] == "tag" {
		return regGetTag(conf)
	}
	// default request is for a digest
	return regGetDigest(conf)
}

func regGetTag(conf config.Source) (Results, error) {
	repo, ok := conf.Args["repo"]
	if !ok {
		return Results{}, fmt.Errorf("repo not defined")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if res, ok := registry.cacheTags[repo]; ok {
		return *res, nil
	}
	repoRef, err := ref.New(repo)
	if err != nil {
		return Results{}, fmt.Errorf("failed to parse repo: %w", err)
	}
	tags, err := registry.rc.TagList(context.Background(), repoRef)
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
	registry.cacheTags[repo] = &res
	return res, nil
}

func regGetDigest(conf config.Source) (Results, error) {
	image, ok := conf.Args["image"]
	if !ok {
		return Results{}, fmt.Errorf("image not defined")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if res, ok := registry.cacheDigest[image]; ok {
		return *res, nil
	}
	imageRef, err := ref.New(image)
	if err != nil {
		return Results{}, fmt.Errorf("failed to parse image: %w", err)
	}
	m, err := registry.rc.ManifestHead(context.Background(), imageRef, regclient.WithManifestRequireDigest())
	if err != nil {
		return Results{}, fmt.Errorf("failed to query image: %w", err)
	}
	dig := m.GetDescriptor().Digest.String()
	res := Results{
		VerMap: map[string]string{
			dig: dig,
		},
	}
	registry.cacheDigest[image] = &res
	return res, nil
}
