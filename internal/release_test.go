package monoreleaser

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, expectedUrl.String(), releaser.releaseClient.url.String())

	assetUrl := "https://uploads.github.com/repos/kharf/myrepo/releases"
	assert.Contains(t, releaser.assetClient.url.String(), assetUrl)

	return commits, releaser
}

func createServer(t *testing.T, changelog Changelog, releaser *GithubReleaser) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualHeader := r.Header

		authHeader := actualHeader.Get("Authorization")
		token, _ := strings.CutPrefix(authHeader, "Bearer ")
		if token == "" || token == "Bearer" {
			w.WriteHeader(500)
			return
		}

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/vnd.github+json", actualHeader.Get("Accept"))

		var response string
		contentTypes := r.Header["Content-Type"]
		if len(contentTypes) != 0 && contentTypes[0] == "application/octet-stream" {
			artifacts := r.URL.Query()["name"]
			assert.Len(t, artifacts, 1)
			artifact := artifacts[0]
			if artifact != "monoreleaser.exe" && artifact != "monoreleaser" {
				t.Fail()
			}

			content, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.Equal(t, "file content", string(content))
			assert.Equal(t, strconv.Itoa(len(content)), actualHeader.Get("Content-Length"))

			response = `{
  "url": "https://api.github.com/repos/octocat/Hello-World/releases/assets/1",
  "browser_download_url": "https://github.com/octocat/Hello-World/releases/download/v1.0.0/example.zip",
  "id": 1,
  "node_id": "MDEyOlJlbGVhc2VBc3NldDE=",
  "name": "example.zip",
  "label": "short description",
  "state": "uploaded",
  "content_type": "application/zip",
  "size": 1024,
  "download_count": 42,
  "created_at": "2013-02-27T19:35:32Z",
  "updated_at": "2013-02-27T19:35:32Z",
  "uploader": {
    "login": "octocat",
    "id": 1,
    "node_id": "MDQ6VXNlcjE=",
    "avatar_url": "https://github.com/images/error/octocat_happy.gif",
    "gravatar_id": "",
    "url": "https://api.github.com/users/octocat",
    "html_url": "https://github.com/octocat",
    "followers_url": "https://api.github.com/users/octocat/followers",
    "following_url": "https://api.github.com/users/octocat/following{/other_user}",
    "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
    "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
    "organizations_url": "https://api.github.com/users/octocat/orgs",
    "repos_url": "https://api.github.com/users/octocat/repos",
    "events_url": "https://api.github.com/users/octocat/events{/privacy}",
    "received_events_url": "https://api.github.com/users/octocat/received_events",
    "type": "User",
    "site_admin": false
  }
}`
		} else {
			actualBody, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			expectedBody, _ := json.Marshal(map[string]string{
				"tag_name": "v1",
				"body":     string(changelog),
				"name":     "v1",
			})

			assert.Equal(t, expectedBody, actualBody)
			response = `{
  "url": "https://api.github.com/repos/octocat/Hello-World/releases/1",
  "html_url": "https://github.com/octocat/Hello-World/releases/v1.0.0",
  "assets_url": "https://api.github.com/repos/octocat/Hello-World/releases/1/assets",
  "upload_url": "https://uploads.github.com/repos/octocat/Hello-World/releases/1/assets{?name,label}",
  "tarball_url": "https://api.github.com/repos/octocat/Hello-World/tarball/v1.0.0",
  "zipball_url": "https://api.github.com/repos/octocat/Hello-World/zipball/v1.0.0",
  "discussion_url": "https://github.com/octocat/Hello-World/discussions/90",
  "id": 1,
  "node_id": "MDc6UmVsZWFzZTE=",
  "tag_name": "v1.0.0",
  "target_commitish": "master",
  "name": "v1.0.0",
  "body": "Description of the release",
  "draft": false,
  "prerelease": false,
  "created_at": "2013-02-27T19:35:32Z",
  "published_at": "2013-02-27T19:35:32Z",
  "author": {
    "login": "octocat",
    "id": 1,
    "node_id": "MDQ6VXNlcjE=",
    "avatar_url": "https://github.com/images/error/octocat_happy.gif",
    "gravatar_id": "",
    "url": "https://api.github.com/users/octocat",
    "html_url": "https://github.com/octocat",
    "followers_url": "https://api.github.com/users/octocat/followers",
    "following_url": "https://api.github.com/users/octocat/following{/other_user}",
    "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
    "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
    "organizations_url": "https://api.github.com/users/octocat/orgs",
    "repos_url": "https://api.github.com/users/octocat/repos",
    "events_url": "https://api.github.com/users/octocat/events{/privacy}",
    "received_events_url": "https://api.github.com/users/octocat/received_events",
    "type": "User",
    "site_admin": false
  },
  "assets": [
    {
      "url": "https://api.github.com/repos/octocat/Hello-World/releases/assets/1",
      "browser_download_url": "https://github.com/octocat/Hello-World/releases/download/v1.0.0/example.zip",
      "id": 1,
      "node_id": "MDEyOlJlbGVhc2VBc3NldDE=",
      "name": "example.zip",
      "label": "short description",
      "state": "uploaded",
      "content_type": "application/zip",
      "size": 1024,
      "download_count": 42,
      "created_at": "2013-02-27T19:35:32Z",
      "updated_at": "2013-02-27T19:35:32Z",
      "uploader": {
        "login": "octocat",
        "id": 1,
        "node_id": "MDQ6VXNlcjE=",
        "avatar_url": "https://github.com/images/error/octocat_happy.gif",
        "gravatar_id": "",
        "url": "https://api.github.com/users/octocat",
        "html_url": "https://github.com/octocat",
        "followers_url": "https://api.github.com/users/octocat/followers",
        "following_url": "https://api.github.com/users/octocat/following{/other_user}",
        "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
        "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
        "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
        "organizations_url": "https://api.github.com/users/octocat/orgs",
        "repos_url": "https://api.github.com/users/octocat/repos",
        "events_url": "https://api.github.com/users/octocat/events{/privacy}",
        "received_events_url": "https://api.github.com/users/octocat/received_events",
        "type": "User",
        "site_admin": false
      }
    }
  ]
}`
		}

		w.WriteHeader(201)
		w.Write([]byte(response))
	}))

	serverUrl, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	releaser.releaseClient.url.Host = serverUrl.Host
	releaser.releaseClient.url.Scheme = serverUrl.Scheme
	require.NotContains(t, releaser.releaseClient.url.Host, "github.com")
	releaser.assetClient.url.Host = serverUrl.Host
	releaser.assetClient.url.Scheme = serverUrl.Scheme
	require.NotContains(t, releaser.assetClient.url.Host, "github.com")

	return ts
}

func TestGithubReleaser_Release(t *testing.T) {
	commits, releaser := createRepoAndGithubReleaser(t, UserSettings{Token: "abcd"})
	diffs := []*Commit{commits[len(commits)-1]}
	changes := Extract(diffs)
	changelog, _ := GenerateChangelog(changes)

	ts := createServer(t, changelog, releaser)
	defer ts.Close()

	err := releaser.Release("v1", ReleaseOptions{})
	assert.NoError(t, err)
}

func TestGithubReleaser_Release_NoToken(t *testing.T) {
	commits, releaser := createRepoAndGithubReleaser(t, UserSettings{})
	diffs := []*Commit{commits[len(commits)-1]}
	changes := Extract(diffs)
	changelog, _ := GenerateChangelog(changes)

	ts := createServer(t, changelog, releaser)
	defer ts.Close()

	err := releaser.Release("v1", ReleaseOptions{})
	assert.ErrorIs(t, err, ErrRequestUnsuccessful)
}

func TestGithubReleaser_Release_NoCommitHistory(t *testing.T) {
	_, releaser := createRepoAndGithubReleaser(t, UserSettings{})
	err := releaser.Release("v1", ReleaseOptions{Module: "notexisting"})
	assert.ErrorIs(t, err, ErrEndOfHistory)
}

func TestGithubReleaser_Release_Upload(t *testing.T) {
	commits, releaser := createRepoAndGithubReleaser(t, UserSettings{Token: "abcd"})
	diffs := []*Commit{commits[len(commits)-1]}
	changes := Extract(diffs)
	changelog, _ := GenerateChangelog(changes)

	ts := createServer(t, changelog, releaser)
	defer ts.Close()

	file := []byte("file content")
	artifacts := []Artifact{
		{
			Name:   "monoreleaser.exe",
			Reader: bytes.NewBuffer(file),
			Size:   int64(len(file)),
		},
		{
			Name:   "monoreleaser",
			Reader: bytes.NewBuffer(file),
			Size:   int64(len(file)),
		},
	}

	err := releaser.Release("v1", ReleaseOptions{Artifacts: artifacts})
	assert.NoError(t, err)
}
