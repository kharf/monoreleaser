package monoreleaser

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

var (
	repository Repository
	commits    = []*Commit{}

	ErrNotFound = errors.New("not found")
	lenCommits  int
)

func TestMain(m *testing.M) {
	repository, commits, lenCommits = newRepo(false)
	os.Exit(m.Run())
}

func newRepo(empty bool) (Repository, []*Commit, int) {
	var commits = []*Commit{}
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

			_, err = gitRepository.CreateTag(fmt.Sprintf("%vv%v", dir, i), lastCommitHash, nil)
			if err != nil {
				log.Panic(err)
			}

			commits = append(commits, &Commit{Hash: lastCommitHash.String(), Message: message})
		}
	}

	lenCommits := len(commits)

	repository := GoGitRepository{repository: gitRepository, name: "myrepo"}
	return repository, commits, lenCommits
}

var diffCommits []*Commit

func BenchmarkDiff(b *testing.B) {
	b.ReportAllocs()
	var c []*Commit
	newTag := Tag{Hash: commits[lenCommits-1].Hash}
	oldTag := Tag{Hash: commits[0].Hash}
	opts := DiffOptions{}
	for i := 0; i < b.N; i++ {
		c, _ = repository.Diff(newTag, oldTag, opts)
	}

	// avoid compiler elimination
	diffCommits = c
}

func TestDiff(t *testing.T) {
	newTag := Tag{Hash: commits[lenCommits-1].Hash}
	oldTag := Tag{Hash: commits[0].Hash}
	diffCommits, _ := repository.Diff(newTag, oldTag, DiffOptions{})

	assert.Len(t, diffCommits, lenCommits-1)

	for i := 8; i >= 0; i-- {
		assert.Equal(t, commits[lenCommits-i-1], diffCommits[i])
		assert.NotEqual(t, diffCommits[i].Message, "feat: oldest")
	}
}

func TestDiff_PathFilterSubDir(t *testing.T) {
	newTag := Tag{Hash: commits[lenCommits-1].Hash}
	oldTag := Tag{Hash: commits[0].Hash}
	diffCommits, _ := repository.Diff(newTag, oldTag, DiffOptions{Module: "subdir"})

	assert.Len(t, commits, 10)
	assert.Len(t, diffCommits, 1)

	assert.Equal(t, commits[lenCommits-1], diffCommits[0])
}

func TestDiff_NotExistingTags(t *testing.T) {
	newTag := Tag{Hash: "asdasd"}
	oldTag := Tag{Hash: "lkjhlkjh"}
	diffCommits, _ := repository.Diff(newTag, oldTag, DiffOptions{})

	assert.Len(t, diffCommits, 0)
}

func TestHead(t *testing.T) {
	head, err := repository.Head()
	assert.NoError(t, err)
	assert.Equal(t, commits[lenCommits-1].Hash, head)
}

func TestTag_LatestCommit(t *testing.T) {
	repository, commits, lenCommits := newRepo(false)
	tag, err := repository.Tag("mytag", TagOptions{})
	assert.NoError(t, err)
	assert.Equal(t, commits[lenCommits-1].Hash, tag.Hash)
	assert.Equal(t, "mytag", tag.Name)
}

func TestTag_SecondLatestCommit(t *testing.T) {
	repository, commits, lenCommits := newRepo(false)
	secondNewestCommit := commits[lenCommits-2]
	tag, err := repository.Tag("mytag", TagOptions{Hash: secondNewestCommit.Hash})
	assert.NoError(t, err)
	assert.Equal(t, secondNewestCommit.Hash, tag.Hash)
	assert.Equal(t, "mytag", tag.Name)
}

func TestTag_NoCommitHistory(t *testing.T) {
	repository, _, _ := newRepo(true)
	_, err := repository.Tag("mytag", TagOptions{})
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
}

func TestGetTag(t *testing.T) {
	tag, err := repository.GetTag("v1", GetTagOptions{})
	assert.NoError(t, err)
	assert.Equal(t, commits[1].Hash, tag.Hash)
	assert.Equal(t, "v1", tag.Name)
}

func TestGetTag_Module(t *testing.T) {
	tag, err := repository.GetTag("v0", GetTagOptions{Module: "subdir"})
	assert.NoError(t, err)
	assert.Equal(t, commits[0].Hash, tag.Hash)
	assert.Equal(t, "subdir/v0", tag.Name)
}

func TestGetTags(t *testing.T) {
	tags, err := repository.GetTags(GetTagOptions{})
	assert.NoError(t, err)
	lenTags := len(tags)
	assert.Equal(t, lenCommits, lenTags)
	for i := lenTags - 1; i >= 0; i-- {
		tagIndex := lenTags - i - 1
		newestTag := tags[tagIndex]
		newestCommit := commits[i]
		assert.Equal(t, newestCommit.Hash, newestTag.Hash)
		var prefix string
		if i == 0 || i == lenTags-1 {
			prefix = tagPrefix("subdir")
		}
		assert.Equal(t, prefix+"v"+strconv.Itoa(i), newestTag.Name)
	}
}

func TestGetTags_Module(t *testing.T) {
	tags, err := repository.GetTags(GetTagOptions{Module: "subdir"})
	assert.NoError(t, err)
	lenTags := len(tags)
	assert.Equal(t, 2, lenTags)
	assert.Equal(t, commits[lenCommits-1].Hash, tags[0].Hash)
	assert.Equal(t, "subdir/v9", tags[0].Name)
	assert.Equal(t, commits[0].Hash, tags[lenTags-1].Hash)
	assert.Equal(t, "subdir/v0", tags[lenTags-1].Name)
}
