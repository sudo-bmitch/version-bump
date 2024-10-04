package source

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sudo-bmitch/version-bump/internal/config"
)

const (
	ghrArgType            = "type"
	ghrArgRepo            = "repo"
	ghrArgArtifact        = "artifact"
	ghrArgAllowDraft      = "allowDraft"
	ghrArgAllowPrerelease = "allowPrerelease"
)

var ghrState struct {
	once           sync.Once
	httpClient     *http.Client
	mu             sync.Mutex // mutex for cache access
	cacheReleases  map[string][]*GHRelease
	cacheArtifacts map[string]*Results
	cacheNames     map[string]*Results
}

func newGHRelease(conf config.Source) (Results, error) {
	if _, ok := conf.Args[ghrArgRepo]; !ok {
		return Results{}, fmt.Errorf("repo argument is required")
	}
	ghrState.once.Do(func() {
		ghrState.httpClient = http.DefaultClient
		ghrState.cacheReleases = map[string][]*GHRelease{}
		ghrState.cacheArtifacts = map[string]*Results{}
		ghrState.cacheNames = map[string]*Results{}
	})
	if conf.Args[ghrArgType] == "artifact" {
		return ghrArtifact(conf)
	}
	return ghrReleaseName(conf)
}

func ghrReleaseList(conf config.Source) ([]*GHRelease, error) {
	repo := conf.Args[ghrArgRepo]
	if releases, ok := ghrState.cacheReleases[repo]; ok {
		return releases, nil
	}
	u, err := url.Parse("https://api.github.com/repos/" + repo + "/releases")
	if err != nil {
		return nil, fmt.Errorf("failed to parse api url, check repo syntax (%s should be org/proj): %w", repo, err)
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Accept", "application/json")
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token != "" {
		req.SetBasicAuth("git", token)
	}
	resp, err := ghrState.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call releases API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status from API, status: %d, body: %s", resp.StatusCode, string(b))
	}
	releases := []*GHRelease{}
	err = json.NewDecoder(resp.Body).Decode(&releases)
	if err != nil {
		return nil, fmt.Errorf("failed to decode release API response: %w", err)
	}
	// cache result for future requests
	ghrState.cacheReleases[repo] = releases
	return releases, nil
}

func ghrReleaseName(conf config.Source) (Results, error) {
	var err error
	allowDraft := false
	if val, ok := conf.Args[ghrArgAllowDraft]; ok {
		allowDraft, err = strconv.ParseBool(val)
		if err != nil {
			return Results{}, fmt.Errorf("allowDraft must be a bool value: \"%s\": %w", val, err)
		}
	}
	allowPrerelease := false
	if val, ok := conf.Args[ghrArgAllowPrerelease]; ok {
		allowPrerelease, err = strconv.ParseBool(val)
		if err != nil {
			return Results{}, fmt.Errorf("allowPrerelease must be a bool value: \"%s\": %w", val, err)
		}
	}
	key := fmt.Sprintf("%s:%t:%t", conf.Args[ghrArgRepo], allowDraft, allowPrerelease)
	ghrState.mu.Lock()
	defer ghrState.mu.Unlock()
	if r, ok := ghrState.cacheNames[key]; ok {
		return *r, nil
	}
	releases, err := ghrReleaseList(conf)
	if err != nil {
		return Results{}, err
	}
	res := Results{
		VerMap:  map[string]string{},
		VerMeta: map[string]interface{}{},
	}
	for _, r := range releases {
		r := r
		if r.Draft && !allowDraft {
			continue
		}
		if r.Prerelease && !allowPrerelease {
			continue
		}
		res.VerMap[r.TagName] = r.TagName
		res.VerMeta[r.TagName] = r
	}
	ghrState.cacheNames[key] = &res
	return res, nil
}

func ghrArtifact(conf config.Source) (Results, error) {
	var err error
	allowDraft := false
	if val, ok := conf.Args[ghrArgAllowDraft]; ok {
		allowDraft, err = strconv.ParseBool(val)
		if err != nil {
			return Results{}, fmt.Errorf("allowDraft must be a bool value: \"%s\": %w", val, err)
		}
	}
	allowPrerelease := false
	if val, ok := conf.Args[ghrArgAllowPrerelease]; ok {
		allowPrerelease, err = strconv.ParseBool(val)
		if err != nil {
			return Results{}, fmt.Errorf("allowPrerelease must be a bool value: \"%s\": %w", val, err)
		}
	}
	artifactName, ok := conf.Args["artifact"]
	if !ok {
		return Results{}, fmt.Errorf("missing arg \"artifact\"")
	}
	key := fmt.Sprintf("%s:%s:%t:%t", conf.Args[ghrArgRepo], artifactName, allowDraft, allowPrerelease)
	ghrState.mu.Lock()
	defer ghrState.mu.Unlock()
	if r, ok := ghrState.cacheArtifacts[key]; ok {
		return *r, nil
	}
	releases, err := ghrReleaseList(conf)
	if err != nil {
		return Results{}, err
	}
	res := Results{
		VerMap:  map[string]string{},
		VerMeta: map[string]interface{}{},
	}
	for _, r := range releases {
		r := r
		if r.Draft && !allowDraft {
			continue
		}
		if r.Prerelease && !allowPrerelease {
			continue
		}
		for _, asset := range r.Assets {
			if asset.Name == artifactName {
				asset := asset
				res.VerMap[r.TagName] = asset.DownloadURL
				res.VerMeta[r.TagName] = asset
				break
			}
		}
	}
	if len(res.VerMap) <= 0 {
		return Results{}, fmt.Errorf("no releases found with artifact \"%s\"", artifactName)
	}
	ghrState.cacheArtifacts[key] = &res
	return res, nil
}

type GHRelease struct {
	URL             string     `json:"url"`
	HTMLURL         string     `json:"html_url"`
	ID              int64      `json:"id"`
	TagName         string     `json:"tag_name"`
	TargetCommitish string     `json:"target_commitish"`
	Name            string     `json:"name"`
	Draft           bool       `json:"draft"`
	Prerelease      bool       `json:"prerelease"`
	CreatedAt       GHTime     `json:"created_at"`
	PublishedAt     GHTime     `json:"published_at"`
	Assets          []*GHAsset `json:"assets"`
}

type GHAsset struct {
	URL           string `json:"url"`
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	ContentType   string `json:"content_type"`
	State         string `json:"state"`
	Size          uint64 `json:"size"`
	DownloadCount uint64 `json:"download_count"`
	DownloadURL   string `json:"browser_download_url"`
	CreatedAt     GHTime `json:"created_at"`
	UpdatedAt     GHTime `json:"updated_at"`
}

type GHTime time.Time

func (t *GHTime) UnmarshalJSON(data []byte) (err error) {
	str := string(data)
	i, err := strconv.ParseInt(str, 10, 64)
	if err == nil {
		*t = GHTime(time.Unix(i, 0))
		if time.Time(*t).Year() > 3000 {
			*t = GHTime(time.Unix(0, i*1e6))
		}
	} else {
		var tt time.Time
		tt, err = time.Parse(`"`+time.RFC3339+`"`, str)
		*t = GHTime(tt)
	}
	return err
}
