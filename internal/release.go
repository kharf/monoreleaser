package monoreleaser

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

var (
	ErrRequestUnsuccessful = errors.New("request was unsuccessful")
)

// Optional parameters for releasing a version.
type ReleaseOptions struct {
	// A Module is just an application (directory) inside a mono repository.
	Module string
}

// A Releaser is capable of drafting and tagging of release versions and posting changelogs to external sources like scms.
type Releaser interface {
	// Release creates a given version with specified optional options like module name.
	// It is also able to create Changelogs and depending on the implementation it posts/uploads it on external sources like scms (github, gitlab ...) and/or writes it to a Changelog file.
	Release(version string, opts ReleaseOptions) error
}

// A VCSReleaser is capable of drafting and tagging of release versions. It doesn't post Changelogs.
type VCSReleaser struct {
	Repository Repository
}

var _ Releaser = VCSReleaser{}

func (rel VCSReleaser) Release(version string, opts ReleaseOptions) error {
	monoRepo := rel.Repository
	tags, err := monoRepo.GetTags(GetTagOptions{Module: opts.Module})
	if err != nil {
		return err
	}
	_, _ = monoRepo.Tag(version, TagOptions{Module: opts.Module})

	// diffs, err := monoRepo.Diff(*newTag, *oldTag, monoreleaser.DiffOptions{PathFilter: filter})

	// if err != nil {
	// 	log.Fatal(err)
	// }

	// changes := monoreleaser.Extract(diffs)
	// for _, change := range changes {
	// 	log.Println(change.Hash)
	// 	log.Println(change.Semantic)
	// 	log.Println(change.Message)
	// }

	// cl, _ := monoreleaser.GenerateChangelog(changes)
	return nil
}

// A GithubReleaser makes use of Git to tag/release versions and the Github Rest API to post releases/changelogs.
// Use the constructor for a preconfigured git repository and http client.
type GithubReleaser struct {
	vcsReleaser VCSReleaser
	client      http.Client
	url         *url.URL
	header      http.Header
}

var _ Releaser = GithubReleaser{}

// User specific releaser settings.
type UserSettings struct {
	Token string
}

func NewGithubReleaser(owner string, repository Repository, userSettings UserSettings) (*GithubReleaser, error) {
	vcsReleaser := VCSReleaser{Repository: repository}
	client := http.Client{}
	header := http.Header{}
	header.Add("Accept", "application/vnd.github+json")
	header.Add("Authorization", "Bearer "+userSettings.Token)

	url, err := url.Parse("https://api.github.com/repos/" + owner + "/" + repository.Name() + "/releases")
	if err != nil {
		return nil, err
	}

	return &GithubReleaser{
		vcsReleaser: vcsReleaser,
		client:      client,
		url:         url,
		header:      header,
	}, nil
}

func (rel GithubReleaser) Release(version string, opts ReleaseOptions) error {
	// diffs, err := monoRepo.Diff(*newTag, *oldTag, monoreleaser.DiffOptions{PathFilter: filter})

	// if err != nil {
	// 	log.Fatal(err)
	// }

	// changes := monoreleaser.Extract(diffs)
	// for _, change := range changes {
	// 	log.Println(change.Hash)
	// 	log.Println(change.Semantic)
	// 	log.Println(change.Message)
	// }

	// cl, _ := monoreleaser.GenerateChangelog(changes)
	return nil
}

func (rel GithubReleaser) post(tag Tag, changelog Changelog) error {
	body, err := json.Marshal(map[string]string{
		"tag_name": tag.Name,
		"body":     string(changelog),
	})

	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, rel.url.String(), bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	request.Header = rel.header

	response, err := rel.client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode <= 299 {
		return nil
	}

	return ErrRequestUnsuccessful
}
