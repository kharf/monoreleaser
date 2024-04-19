package monoreleaser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var (
	ErrRequestUnsuccessful = errors.New("request was unsuccessful")
)

// A file released alongside the changelog.
type Artifact struct {
	Reader io.Reader
	Name   string
	Size   int64
}

// Optional parameters for releasing a version.
type ReleaseOptions struct {
	// A Module is just an application (directory) inside a mono repository.
	Module string
	// Artifacts to upload alongside the changelog.
	Artifacts []Artifact
}

// A Releaser is capable of drafting and tagging of release versions and posting changelogs to external sources like scms.
type Releaser interface {
	// Release creates a given version with specified optional options like module name.
	// It is also able to create Changelogs and depending on the implementation it posts/uploads it on external sources like scms (github, gitlab ...) and/or writes it to a Changelog file.
	Release(version string, opts ReleaseOptions) error
}

type GithubClient struct {
	client http.Client
	url    *url.URL
	header http.Header
}

func (client GithubClient) URL() *url.URL {
	return client.url
}

// A GithubReleaser makes use of Git to tag/release versions and the Github Rest API to post releases/changelogs.
// Use the constructor for a preconfigured git repository and http client.
type GithubReleaser struct {
	repository    Repository
	releaseClient GithubClient
	assetClient   GithubClient
}

func (rel GithubReleaser) ReleaseClient() GithubClient {
	return rel.releaseClient
}

func (rel GithubReleaser) AssetClient() GithubClient {
	return rel.assetClient
}

var _ Releaser = GithubReleaser{}

// User specific static releaser settings.
type UserSettings struct {
	Token string
}

func NewGithubReleaser(
	owner string,
	repository Repository,
	userSettings UserSettings,
) (*GithubReleaser, error) {
	releaseClient := http.Client{Timeout: time.Second * 10}
	releaseHeader := http.Header{}
	releaseHeader.Add("Accept", "application/vnd.github+json")
	releaseHeader.Add("Authorization", "Bearer "+userSettings.Token)

	releaseURL, err := url.Parse(
		"https://api.github.com/repos/" + owner + "/" + repository.Name() + "/releases",
	)
	if err != nil {
		return nil, err
	}

	assetClient := http.Client{Timeout: time.Second * 10}
	assetHeader := http.Header{}
	assetHeader.Add("Accept", "application/vnd.github+json")
	assetHeader.Add("Content-Type", "application/octet-stream")
	assetHeader.Add("Authorization", "Bearer "+userSettings.Token)

	assetURL, err := url.Parse(
		"https://uploads.github.com/repos/" + owner + "/" + repository.Name() + "/releases",
	)
	if err != nil {
		return nil, err
	}

	return &GithubReleaser{
		repository: repository,
		releaseClient: GithubClient{
			client: releaseClient,
			url:    releaseURL,
			header: releaseHeader,
		},
		assetClient: GithubClient{
			client: assetClient,
			url:    assetURL,
			header: assetHeader,
		},
	}, nil
}

func (rel GithubReleaser) Release(version string, opts ReleaseOptions) error {
	monoRepo := rel.repository
	tags, err := monoRepo.GetTags(GetTagOptions{Module: opts.Module})
	if err != nil {
		return err
	}

	tag, err := monoRepo.Tag(version, TagOptions{Module: opts.Module})
	if err != nil {
		return err
	}

	var latestTag *Tag
	if len(tags) > 0 {
		latestTag = &tags[0]
	}

	diffs, err := monoRepo.Diff(*tag, latestTag, DiffOptions{Module: opts.Module})
	if err != nil {
		return err
	}

	cl, err := GenerateChangelog(Extract(diffs))
	if err != nil {
		return err
	}

	err = rel.post(*tag, cl, opts)
	if err != nil {
		return err
	}

	return nil
}

type githubResponse struct {
	ID int `json:"id"`
}

func (rel GithubReleaser) post(tag Tag, changelog Changelog, opts ReleaseOptions) error {
	body, err := json.Marshal(map[string]string{
		"tag_name": tag.Name,
		"body":     string(changelog),
		"name":     tag.Name,
	})
	if err != nil {
		return err
	}

	request, err := http.NewRequest(
		http.MethodPost,
		rel.releaseClient.url.String(),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}

	request.Header = rel.releaseClient.header

	response, err := rel.releaseClient.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("%w: %s", ErrRequestUnsuccessful, responseBody)
	}

	var ghResponse githubResponse
	if err := json.Unmarshal(responseBody, &ghResponse); err != nil {
		return err
	}

	if len(opts.Artifacts) != 0 {
		if err := rel.upload(ghResponse.ID, opts); err != nil {
			return err
		}
	}

	return nil
}

func (rel GithubReleaser) upload(releaseID int, opts ReleaseOptions) error {
	for _, artifact := range opts.Artifacts {
		request, err := http.NewRequest(
			http.MethodPost,
			rel.assetClient.url.String()+"/"+strconv.Itoa(releaseID)+"/assets?name="+artifact.Name,
			artifact.Reader,
		)
		if err != nil {
			return err
		}

		request.Header = rel.assetClient.header

		request.ContentLength = artifact.Size

		response, err := rel.assetClient.client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
	}

	return nil
}
