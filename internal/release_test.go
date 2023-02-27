package monoreleaser

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

func createRepoAndGithubReleaser(t *testing.T, userSettings UserSettings) ([]*Commit, *GithubReleaser, http.Header) {
	repository, commits, _, _ := newRepo(false)

	workTree, err := repository.repository.Worktree()
	if err != nil {
		log.Panic(err)
	}

	fs := workTree.Filesystem
	file, err := fs.Create("myNewReleaseChange")
	if err != nil {
		log.Panic(err)
	}
	workTree.Add(file.Name())
	message := "feat: my release change"
	lastCommitHash, err := workTree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "orca",
			Email: "orca-dev@mail.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		log.Panic(err)
	}
	commits = append(commits, &Commit{Hash: lastCommitHash.String(), Message: message})

	releaser, _ := NewGithubReleaser("kharf", repository, userSettings)

	expectedUrl, _ := url.Parse("https://api.github.com/repos/kharf/myrepo/releases")
	assert.Equal(t, expectedUrl.String(), releaser.url.String())

	actualHeader := releaser.header
	expectedHeader := http.Header{}
	expectedHeader.Add("Accept", "application/vnd.github+json")
	if userSettings.Token != "" {
		expectedHeader.Add("Authorization", "Bearer "+userSettings.Token)
	}

	assert.Equal(t, expectedHeader.Get("Accept"), actualHeader.Get("Accept"))
	assert.Equal(t, expectedHeader.Get("Authorization"), actualHeader.Get("Authorization"))
	return commits, releaser, expectedHeader
}

func createServer(t *testing.T, expectedHeader http.Header, changelog Changelog, expectedResponseStatusCode int) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualHeader := r.Header
		assert.Equal(t, expectedHeader.Get("Accept"), actualHeader.Get("Accept"))
		assert.Equal(t, expectedHeader.Get("Authorization"), actualHeader.Get("Authorization"))

		actualBody, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		expectedBody, _ := json.Marshal(map[string]string{
			"tag_name": "v1",
			"body":     string(changelog),
		})

		assert.Equal(t, expectedBody, actualBody)

		w.WriteHeader(expectedResponseStatusCode)
	}))
	return ts
}

func TestGithubReleaser_Release(t *testing.T) {
	commits, releaser, expectedHeader := createRepoAndGithubReleaser(t, UserSettings{Token: "abcd"})
	diffs := []*Commit{commits[len(commits)-1]}
	changes := Extract(diffs)
	changelog, _ := GenerateChangelog(changes)

	ts := createServer(t, expectedHeader, changelog, 201)
	defer ts.Close()

	url, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	releaser.url = url

	err = releaser.Release("v1", ReleaseOptions{})
	assert.NoError(t, err)
}

func TestGithubReleaser_Release_NoToken(t *testing.T) {
	commits, releaser, expectedHeader := createRepoAndGithubReleaser(t, UserSettings{})
	diffs := []*Commit{commits[len(commits)-1]}
	changes := Extract(diffs)
	changelog, _ := GenerateChangelog(changes)

	ts := createServer(t, expectedHeader, changelog, 201)
	defer ts.Close()

	url, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	releaser.url = url

	err = releaser.Release("v1", ReleaseOptions{})
	assert.NoError(t, err)
}

func TestGithubReleaser_Release_500(t *testing.T) {
	commits, releaser, expectedHeader := createRepoAndGithubReleaser(t, UserSettings{Token: "abcd"})
	diffs := []*Commit{commits[len(commits)-1]}
	changes := Extract(diffs)
	changelog, _ := GenerateChangelog(changes)

	ts := createServer(t, expectedHeader, changelog, 500)
	defer ts.Close()

	url, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	releaser.url = url

	err = releaser.Release("v1", ReleaseOptions{})
	assert.ErrorIs(t, err, ErrRequestUnsuccessful)
}

func TestGithubReleaser_Release_NoCommitHistory(t *testing.T) {
	_, releaser, _ := createRepoAndGithubReleaser(t, UserSettings{})
	err := releaser.Release("v1", ReleaseOptions{Module: "notexisting"})
	assert.ErrorIs(t, err, ErrEndOfHistory)
}
