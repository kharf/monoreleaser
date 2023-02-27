package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	. "github.com/kharf/monoreleaser/internal"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
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

func createServer(t *testing.T, changelog Changelog, releaser Releaser) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualBody, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		expectedBody, _ := json.Marshal(map[string]string{
			"tag_name": "1.0.0",
			"body":     string(changelog),
		})

		assert.Equal(t, expectedBody, actualBody)

		w.WriteHeader(201)
	}))

	ghReleaser, ok := releaser.(*GithubReleaser)
	assert.True(t, ok)
	url := ghReleaser.URL()
	serverUrl, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	url.Host = serverUrl.Host
	url.Scheme = serverUrl.Scheme
	assert.NotContains(t, url.Host, "github.com")
	return ts
}

func TestRootCommand(t *testing.T) {
	repo, _ := newRepo(false)
	config := viper.New()
	configBuffer := bytes.NewBufferString("mycfg")
	config.ReadConfig(configBuffer)
	rootCmdBuilder, err := initCli(repo, config)
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
	rootCmdBuilder, err := initCli(repo, config)
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
  -h, --help   help for release
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
	rootCmdBuilder, err := initCli(repo, config)
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
  -h, --help   help for release

`
	assert.Equal(t, expectedOutput, buffer.String())
}

func TestReleaseCommand(t *testing.T) {
	config := viper.New()
	config.SetConfigType("yaml")
	configYaml := `owner: "kharf"
name: "monoreleaser"
provider: "github"`
	configBuffer := bytes.NewBufferString(configYaml)
	err := config.ReadConfig(configBuffer)
	assert.NoError(t, err)

	repo, diffs := newRepo(false)
	rootCmdBuilder, err := initCli(repo, config)
	assert.NoError(t, err)

	changes := Extract(diffs)
	changelog, _ := GenerateChangelog(changes)

	releaser := rootCmdBuilder.releaseCmdBuilder.releaser
	ts := createServer(t, changelog, releaser)
	defer ts.Close()

	rootCmd := rootCmdBuilder.Build()
	buffer := &bytes.Buffer{}
	rootCmd.SetOut(buffer)
	rootCmd.SetErr(buffer)
	rootCmd.SetArgs([]string{"release", ".", "1.0.0"})

	_, err = rootCmd.ExecuteC()
	assert.NoError(t, err)
	expectedOutput := ""
	assert.Equal(t, expectedOutput, buffer.String())
}