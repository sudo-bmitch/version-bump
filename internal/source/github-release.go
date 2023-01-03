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

const ()

type ghRelease struct {
	conf       config.Source
	httpClient *http.Client
}

func newGHRelease(conf config.Source) Source {
	return ghRelease{conf: conf, httpClient: http.DefaultClient}
}

func (g ghRelease) Get(data config.SourceTmplData) (string, error) {
	confExp, err := g.conf.ExpandTemplate(data)
	if err != nil {
		return "", fmt.Errorf("failed to expand template: %w", err)
	}
	releases, err := g.getReleaseList(confExp)
	if err != nil {
		return "", err
	}
	if confExp.Args["type"] == "artifact" {
		return g.getArtifact(confExp, releases)
	}
	return g.getReleaseName(confExp, releases)
}

var (
	ghCache     map[string][]*GHRelease = map[string][]*GHRelease{}
	ghCacheLock sync.Mutex
)

func (g ghRelease) getReleaseList(confExp config.Source) ([]*GHRelease, error) {
	if _, ok := confExp.Args["repo"]; !ok {
		return nil, fmt.Errorf("repo argument is required")
	}
	if releases, ok := ghCache[confExp.Args["repo"]]; ok {
		return releases, nil
	}
	ghCacheLock.Lock()
	defer ghCacheLock.Unlock()
	u, err := url.Parse("https://api.github.com/repos/" + confExp.Args["repo"] + "/releases")
	if err != nil {
		return nil, fmt.Errorf("failed to parse api url, check repo syntax (%s should be org/proj): %w", confExp.Args["repo"], err)
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
	resp, err := g.httpClient.Do(req)
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
	ghCache[confExp.Args["repo"]] = releases
	return releases, nil
}

func (g ghRelease) getReleaseName(confExp config.Source, releases []*GHRelease) (string, error) {
	var err error
	verData := VersionTmplData{
		VerMap:  map[string]string{},
		VerMeta: map[string]interface{}{},
	}
	allowDraft := false
	if val, ok := confExp.Args["allowDraft"]; ok {
		allowDraft, err = strconv.ParseBool(val)
		if err != nil {
			return "", fmt.Errorf("allowDraft must be a bool value: \"%s\": %w", val, err)
		}
	}
	allowPrerelease := false
	if val, ok := confExp.Args["allowPrerelease"]; ok {
		allowPrerelease, err = strconv.ParseBool(val)
		if err != nil {
			return "", fmt.Errorf("allowPrerelease must be a bool value: \"%s\": %w", val, err)
		}
	}
	for _, r := range releases {
		r := r
		if r.Draft && !allowDraft {
			continue
		}
		if r.Prerelease && !allowPrerelease {
			continue
		}
		verData.VerMap[r.TagName] = r.TagName
		verData.VerMeta[r.TagName] = r
	}
	return procResult(confExp, verData)
}

func (g ghRelease) getArtifact(confExp config.Source, releases []*GHRelease) (string, error) {
	var err error
	verData := VersionTmplData{
		VerMap:  map[string]string{},
		VerMeta: map[string]interface{}{},
	}
	artifactName, ok := confExp.Args["artifact"]
	if !ok {
		return "", fmt.Errorf("missing arg \"artifact\"")
	}
	allowDraft := false
	if val, ok := confExp.Args["allowDraft"]; ok {
		allowDraft, err = strconv.ParseBool(val)
		if err != nil {
			return "", fmt.Errorf("allowDraft must be a bool value: \"%s\": %w", val, err)
		}
	}
	allowPrerelease := false
	if val, ok := confExp.Args["allowPrerelease"]; ok {
		allowPrerelease, err = strconv.ParseBool(val)
		if err != nil {
			return "", fmt.Errorf("allowPrerelease must be a bool value: \"%s\": %w", val, err)
		}
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
				verData.VerMap[r.TagName] = asset.DownloadURL
				verData.VerMeta[r.TagName] = asset
				break
			}
		}
	}
	return procResult(confExp, verData)
}

func (g ghRelease) Key(data config.SourceTmplData) (string, error) {
	confExp, err := g.conf.ExpandTemplate(data)
	if err != nil {
		return "", fmt.Errorf("failed to expand template: %w", err)
	}
	return confExp.Key, nil
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
