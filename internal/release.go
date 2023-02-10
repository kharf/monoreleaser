package monoreleaser

import (
	"errors"
	"net/http"
	"net/url"
)

var (
	ErrRequestUnsuccessful = errors.New("request was unsuccessful")
)

// A Releaser is capable of posting changelogs to external sources
type Releaser interface {
	Release(changelog Changelog) error
}

// A GithubReleaser makes use of the Github Rest API to create releases
type GithubReleaser func(changelog Changelog) error

type UserSettings struct {
	Token string
}

// A GithubReleaser makes use of the Github Rest API to create releases
func NewGithubReleaser(owner string, repository string, userSettings UserSettings) (*GithubReleaser, error) {
	client := http.Client{}
	header := http.Header{}
	header.Add("Accept", "application/vnd.github+json")
	header.Add("Authorization", "Bearer "+userSettings.Token)

	url, err := url.Parse("https://api.github.com/repos/" + owner + "/" + repository + "/releases")
	if err != nil {
		return nil, err
	}

	request := &http.Request{
		URL:    url,
		Header: header,
	}

	releaser := GithubReleaser(func(changelog Changelog) error {
		response, err := client.Do(request)
		if err != nil {
			return err
		}

		if response.StatusCode >= 200 && response.StatusCode <= 299 {
			return nil
		} else {
			return ErrRequestUnsuccessful
		}
	})

	return &releaser, nil
}

func (rel GithubReleaser) Release(changelog Changelog) error {
	return rel(changelog)
}
