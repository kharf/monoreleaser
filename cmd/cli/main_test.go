package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	. "github.com/kharf/monoreleaser/internal"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRepo(empty bool) (*git.Repository, []*Commit) {
	var commits []*Commit
	gitRepository, _ := git.Init(memory.NewStorage(), memfs.New())
	workTree, err := gitRepository.Worktree()
	if err != nil {
		log.Panic(err)
	}

	if !empty {
		fs := workTree.Filesystem

		commitMessages := []string{
			"feat: oldest",
			"feat!: major change",
			"fix: patch change",
			"style: change",
			"chore: change",
			"test: change",
			"refactor: change",
			"ci: change",
			"build: change",
			"docs: newest",
		}

		lenCommitMessages := len(commitMessages)

		for i, message := range commitMessages {
			var dir string
			// make two changes in a folder
			if i == 0 || i == lenCommitMessages-1 {
				dir = "subdir/"
			} else {
				dir = ""
			}

			file, err := fs.Create(fmt.Sprintf("%s%v", dir, i))
			if err != nil {
				log.Panic(err)
			}
			workTree.Add(file.Name())
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
		}
	}

	sort.Slice(commits, func(i, j int) bool {
		return i > j
	})

	return gitRepository, commits
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
	releaser.ReleaseClient().URL().Host = serverUrl.Host
	releaser.ReleaseClient().URL().Scheme = serverUrl.Scheme
	require.NotContains(t, releaser.ReleaseClient().URL().Host, "github.com")
	releaser.AssetClient().URL().Host = serverUrl.Host
	releaser.AssetClient().URL().Scheme = serverUrl.Scheme
	require.NotContains(t, releaser.AssetClient().URL().Host, "github.com")

	return ts
}

func TestRootCommand(t *testing.T) {
	repo, _ := newRepo(false)
	config := viper.New()
	configBuffer := bytes.NewBufferString("mycfg")
	config.ReadConfig(configBuffer)
	rootCmdBuilder, err := initCli(repo, config, afero.NewMemMapFs())
	assert.NoError(t, err)

	rootCmd := rootCmdBuilder.Build()
	buffer := &bytes.Buffer{}
	rootCmd.SetOut(buffer)
	rootCmd.SetErr(buffer)
	rootCmd.SetArgs([]string{})

	_, err = rootCmd.ExecuteC()
	assert.NoError(t, err)
	expectedOutput :=
		`Monoreleaser is a CLI to create and view Releases for any Git Repository.
It aims to support a variety of Languages, Repository structures and Git hosting services.

Usage:
  monoreleaser [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  release     Release a piece of Software (Module)

Flags:
  -h, --help   help for monoreleaser

Use "monoreleaser [command] --help" for more information about a command.
`
	assert.Equal(t, expectedOutput, buffer.String())
}

func TestReleaseCommand_Help(t *testing.T) {
	config := viper.New()
	config.SetConfigType("yaml")
	configYaml := `owner: "kharf"
name: "monoreleaser"
provider: "github"`
	configBuffer := bytes.NewBufferString(configYaml)
	err := config.ReadConfig(configBuffer)
	assert.NoError(t, err)

	repo, _ := newRepo(false)
	rootCmdBuilder, err := initCli(repo, config, afero.NewMemMapFs())
	assert.NoError(t, err)

	rootCmd := rootCmdBuilder.Build()
	buffer := &bytes.Buffer{}
	rootCmd.SetOut(buffer)
	rootCmd.SetErr(buffer)
	rootCmd.SetArgs([]string{"release", "-h"})

	_, err = rootCmd.ExecuteC()
	assert.NoError(t, err)
	expectedOutput := `Release a piece of Software (Module)

Usage:
  monoreleaser release [MODULE] [VERSION] [flags]

Flags:
      --artifacts strings   artifacts to upload alongside the changelog (if supported by the provider)
  -h, --help                help for release
`
	assert.Equal(t, expectedOutput, buffer.String())
}

func TestReleaseCommand_RequiredArgs(t *testing.T) {
	config := viper.New()
	config.SetConfigType("yaml")
	configYaml := `owner: "kharf"
name: "monoreleaser"
provider: "github"`
	configBuffer := bytes.NewBufferString(configYaml)
	err := config.ReadConfig(configBuffer)
	assert.NoError(t, err)

	repo, _ := newRepo(false)
	rootCmdBuilder, err := initCli(repo, config, afero.NewMemMapFs())
	assert.NoError(t, err)

	rootCmd := rootCmdBuilder.Build()
	buffer := &bytes.Buffer{}
	rootCmd.SetOut(buffer)
	rootCmd.SetErr(buffer)
	rootCmd.SetArgs([]string{"release"})

	_, err = rootCmd.ExecuteC()
	assert.Error(t, err)
	expectedOutput := `Error: requires at least 2 arg(s), only received 0
Usage:
  monoreleaser release [MODULE] [VERSION] [flags]

Flags:
      --artifacts strings   artifacts to upload alongside the changelog (if supported by the provider)
  -h, --help                help for release

`
	assert.Equal(t, expectedOutput, buffer.String())
}

func TestReleaseCommand(t *testing.T) {
	config := viper.New()
	config.SetConfigType("yaml")
	configYaml := `owner: "kharf"
name: "monoreleaser"
provider: "github"
github:
  token: "abcd"`
	configBuffer := bytes.NewBufferString(configYaml)
	err := config.ReadConfig(configBuffer)
	assert.NoError(t, err)

	repo, diffs := newRepo(false)
	rootCmdBuilder, err := initCli(repo, config, afero.NewMemMapFs())
	assert.NoError(t, err)

	changes := Extract(diffs)
	changelog, _ := GenerateChangelog(changes)

	releaser := rootCmdBuilder.releaseCmdBuilder.releaser
	ghReleaser, ok := releaser.(*GithubReleaser)
	assert.True(t, ok)
	ts := createServer(t, changelog, ghReleaser)
	defer ts.Close()

	rootCmd := rootCmdBuilder.Build()
	buffer := &bytes.Buffer{}
	rootCmd.SetOut(buffer)
	rootCmd.SetErr(buffer)
	rootCmd.SetArgs([]string{"release", ".", "v1"})

	_, err = rootCmd.ExecuteC()
	assert.NoError(t, err)
	expectedOutput := ""
	assert.Equal(t, expectedOutput, buffer.String())
}

func TestReleaseCommand_Artifacts(t *testing.T) {
	config := viper.New()
	config.SetConfigType("yaml")
	configYaml := `owner: "kharf"
name: "monoreleaser"
provider: "github"
github:
  token: "abcd"`
	configBuffer := bytes.NewBufferString(configYaml)
	err := config.ReadConfig(configBuffer)
	assert.NoError(t, err)

	repo, diffs := newRepo(false)
	fs := afero.NewMemMapFs()
	rootCmdBuilder, err := initCli(repo, config, fs)
	assert.NoError(t, err)

	changes := Extract(diffs)
	changelog, _ := GenerateChangelog(changes)

	releaser := rootCmdBuilder.releaseCmdBuilder.releaser
	ghReleaser, ok := releaser.(*GithubReleaser)
	assert.True(t, ok)
	ts := createServer(t, changelog, ghReleaser)
	defer ts.Close()

	fs.MkdirAll("build/output", 755)
	afero.WriteFile(fs, "build/output/monoreleaser.exe", []byte("file content"), 755)
	afero.WriteFile(fs, "build/monoreleaser", []byte("file content"), 755)
	rootCmd := rootCmdBuilder.Build()
	buffer := &bytes.Buffer{}
	rootCmd.SetOut(buffer)
	rootCmd.SetErr(buffer)
	rootCmd.SetArgs([]string{"release", ".", "v1", "--artifacts=\"build/output/monoreleaser.exe\",\"build/monoreleaser\""})

	_, err = rootCmd.ExecuteC()
	assert.NoError(t, err)
	expectedOutput := ""
	assert.Equal(t, expectedOutput, buffer.String())
}