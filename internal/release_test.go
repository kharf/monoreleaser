package monoreleaser

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

func createRepoAndGithubReleaser(t *testing.T, userSettings UserSettings) ([]*Commit, *GithubReleaser) {
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

	return commits, releaser
}

func createServer(t *testing.T, changelog Changelog) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualHeader := r.Header

		authHeader := actualHeader.Get("Authorization")
		token, _ := strings.CutPrefix(authHeader, "Bearer ")
		if token == "" || token == "Bearer" {
			w.WriteHeader(500)
			return
		}

		assert.Equal(t, "application/vnd.github+json", actualHeader.Get("Accept"))

		actualBody, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		expectedBody, _ := json.Marshal(map[string]string{
			"tag_name": "v1",
			"body":     string(changelog),
		})

		assert.Equal(t, expectedBody, actualBody)

		w.WriteHeader(201)
	}))
	return ts
}

func TestGithubReleaser_Release(t *testing.T) {
	commits, releaser := createRepoAndGithubReleaser(t, UserSettings{Token: "abcd"})
	diffs := []*Commit{commits[len(commits)-1]}
	changes := Extract(diffs)
	changelog, _ := GenerateChangelog(changes)

	ts := createServer(t, changelog)
	defer ts.Close()

	url, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	releaser.url = url

	err = releaser.Release("v1", ReleaseOptions{})
	assert.NoError(t, err)
}

func TestGithubReleaser_Release_NoToken(t *testing.T) {
	commits, releaser := createRepoAndGithubReleaser(t, UserSettings{})
	diffs := []*Commit{commits[len(commits)-1]}
	changes := Extract(diffs)
	changelog, _ := GenerateChangelog(changes)

	ts := createServer(t, changelog)
	defer ts.Close()

	url, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	releaser.url = url

	err = releaser.Release("v1", ReleaseOptions{})
	assert.ErrorIs(t, err, ErrRequestUnsuccessful)
}

func TestGithubReleaser_Release_NoCommitHistory(t *testing.T) {
	_, releaser := createRepoAndGithubReleaser(t, UserSettings{})
	err := releaser.Release("v1", ReleaseOptions{Module: "notexisting"})
	assert.ErrorIs(t, err, ErrEndOfHistory)
}
