package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"
)

// Repository is where noct's releases are published.
const Repository = "AlexandreSJ/noct"

// maxDownload bounds what one request may read, so a broken server cannot fill the memory.
const maxDownload = 64 << 20

// releasesListed is how many of the latest releases a check reads, with their changelogs.
const releasesListed = 30

// Release is one published version of noct and its changelog, as written for the release.
type Release struct {
	Version   string
	Notes     string
	Published time.Time
}

// Source is where releases are listed and downloaded from; tests point it at a local server.
type Source struct {
	// API lists the releases (GitHub's REST API for the repository).
	API string
	// Downloads holds each release's files, under its version.
	Downloads string
	Client    *http.Client
}

// GitHub is noct's own releases.
func GitHub() Source {
	return Source{
		API:       "https://api.github.com/repos/" + Repository,
		Downloads: "https://github.com/" + Repository + "/releases/download",
		Client:    &http.Client{Timeout: 2 * time.Minute},
	}
}

// Releases lists the latest published releases, the newest first. Drafts and tags that are not
// versions are left out.
func (s Source) Releases(ctx context.Context) ([]Release, error) {
	body, err := s.get(ctx, fmt.Sprintf("%s/releases?per_page=%d", s.API, releasesListed))
	if err != nil {
		return nil, fmt.Errorf("listing the releases: %w", err)
	}
	var listed []struct {
		Tag       string    `json:"tag_name"`
		Body      string    `json:"body"`
		Draft     bool      `json:"draft"`
		Published time.Time `json:"published_at"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		return nil, fmt.Errorf("reading the releases: %w", err)
	}
	var releases []Release
	for _, r := range listed {
		if !r.Draft && IsRelease(r.Tag) {
			releases = append(releases, Release{Version: r.Tag, Notes: r.Body, Published: r.Published})
		}
	}
	slices.SortFunc(releases, func(a, b Release) int { return Compare(b.Version, a.Version) })
	return releases, nil
}

// Newer are the releases newer than current, the newest first; none for a dev build.
func Newer(releases []Release, current string) []Release {
	if !IsRelease(current) {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(releases), func(r Release) bool { return Compare(r.Version, current) <= 0 })
}

func (s Source) get(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "noct")
	response, err := s.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", url, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxDownload+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxDownload {
		return nil, fmt.Errorf("%s is larger than %d MB", url, maxDownload>>20)
	}
	return body, nil
}
