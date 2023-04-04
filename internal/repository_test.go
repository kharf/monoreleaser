package monoreleaser

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
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
	commits    []*Commit
	tags       []Tag

	lenCommits int
)

func TestMain(m *testing.M) {
	repository, commits, tags, lenCommits = newRepo(false)
	os.Exit(m.Run())
}

func newRepo(empty bool) (GoGitRepository, []*Commit, []Tag, int) {
	var commits []*Commit
	var tags []Tag
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
			"build: another change",
			"docs: newest",
		}

		lenCommitMessages := len(commitMessages)

		var latestMajor int
		var latestMinor int
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

			var version string
			switch before, _, _ := strings.Cut(message, ":"); before {
			case "feat!":
				version = dir + "v" + strconv.Itoa(i) + ".0.0"
				latestMajor = i
			case "fix":
				version = dir + "v" + strconv.Itoa(latestMajor) + "." + strconv.Itoa(latestMinor) + "." + strconv.Itoa(i)
			default:
				version = dir + "v" + strconv.Itoa(latestMajor) + "." + strconv.Itoa(i) + ".0"
				latestMinor = i
			}

			_, err = gitRepository.CreateTag(version, lastCommitHash, nil)
			if err != nil {
				log.Panic(err)
			}
			tags = append(tags, Tag{Hash: lastCommitHash.String(), Name: version})

			commits = append(commits, &Commit{Hash: lastCommitHash.String(), Message: message})
		}
	}

	lenCommits := len(commits)

	repository := NewGoGitRepository("myrepo", gitRepository)
	return repository, commits, tags, lenCommits
}

func TestHistory(t *testing.T) {
	commitIter, err := repository.History(HistoryOptions{})
	assert.NoError(t, err)

	for i := 0; i < lenCommits; i++ {
		commit, err := commitIter.Next()
		assert.NoError(t, err)
		assert.Equal(t, commits[lenCommits-i-1], commit)
	}

	commit, err := commitIter.Next()
	assert.ErrorIs(t, err, ErrEndOfHistory)
	assert.Nil(t, commit)
}

func TestHistory_Module(t *testing.T) {
	commitIter, err := repository.History(HistoryOptions{Module: "subdir"})
	assert.NoError(t, err)

	commit, err := commitIter.Next()
	assert.NoError(t, err)
	assert.Equal(t, commits[lenCommits-1], commit)

	commit, err = commitIter.Next()
	assert.NoError(t, err)
	assert.Equal(t, commits[0], commit)

	commit, err = commitIter.Next()
	assert.ErrorIs(t, err, ErrEndOfHistory)
	assert.Nil(t, commit)
}

func TestHistory_Hash(t *testing.T) {
	commitIter, err := repository.History(HistoryOptions{Hash: commits[1].Hash})
	assert.NoError(t, err)

	commit, err := commitIter.Next()
	assert.NoError(t, err)
	assert.Equal(t, commits[1], commit)

	commit, err = commitIter.Next()
	assert.NoError(t, err)
	assert.Equal(t, commits[0], commit)

	commit, err = commitIter.Next()
	assert.ErrorIs(t, err, ErrEndOfHistory)
	assert.Nil(t, commit)
}

func TestHistory_Module_NotFound(t *testing.T) {
	commitIter, err := repository.History(HistoryOptions{Hash: "bla"})
	assert.ErrorIs(t, err, ErrUnrecognizedHash)
	assert.Nil(t, commitIter)
}

func TestHistory_Hash_NotFound(t *testing.T) {
	commitIter, err := repository.History(HistoryOptions{Module: "bla"})
	assert.NoError(t, err)

	commit, err := commitIter.Next()
	assert.ErrorIs(t, err, ErrEndOfHistory)
	assert.Nil(t, commit)
}

var diffCommits []*Commit

func BenchmarkDiff(b *testing.B) {
	b.ReportAllocs()
	var c []*Commit
	newTag := Tag{Hash: commits[lenCommits-1].Hash}
	oldTag := &Tag{Hash: commits[0].Hash}
	opts := DiffOptions{}
	for i := 0; i < b.N; i++ {
		c, _ = repository.Diff(newTag, oldTag, opts)
	}

	// avoid compiler elimination
	diffCommits = c
}

func TestDiff(t *testing.T) {
	newTag := Tag{Hash: commits[lenCommits-1].Hash}
	oldTag := &Tag{Hash: commits[0].Hash}
	diffCommits, err := repository.Diff(newTag, oldTag, DiffOptions{})
	assert.NoError(t, err)

	assert.Len(t, diffCommits, lenCommits-1)

	for i := len(diffCommits) - 1; i >= 0; i-- {
		assert.Equal(t, commits[lenCommits-i-1], diffCommits[i])
		assert.NotEqual(t, diffCommits[i].Message, "feat: oldest")
	}
}

func TestDiff_NoOlderTagProvided(t *testing.T) {
	newTag := Tag{Hash: commits[lenCommits-1].Hash}
	diffCommits, err := repository.Diff(newTag, nil, DiffOptions{})
	assert.NoError(t, err)

	assert.Len(t, diffCommits, lenCommits)

	for i := lenCommits - 1; i >= 0; i-- {
		assert.Equal(t, commits[lenCommits-i-1], diffCommits[i])
	}
}

func TestDiff_PathFilterSubDir(t *testing.T) {
	newTag := Tag{Hash: commits[lenCommits-1].Hash}
	oldTag := &Tag{Hash: commits[0].Hash}
	diffCommits, _ := repository.Diff(newTag, oldTag, DiffOptions{Module: "subdir"})

	assert.Len(t, commits, 11)
	assert.Len(t, diffCommits, 1)

	assert.Equal(t, commits[lenCommits-1], diffCommits[0])
}

func TestDiff_PathFilterSubDir_OldTagFromOtherDir(t *testing.T) {
	newTag := Tag{Hash: commits[lenCommits-1].Hash}
	oldTag := &Tag{Hash: commits[lenCommits-2].Hash}
	diffCommits, _ := repository.Diff(newTag, oldTag, DiffOptions{Module: "subdir"})

	assert.Len(t, commits, 11)
	assert.Len(t, diffCommits, 1)

	assert.Equal(t, commits[lenCommits-1], diffCommits[0])
}

func TestDiff_NoExistingTags(t *testing.T) {
	newTag := Tag{Hash: "asdasd"}
	oldTag := &Tag{Hash: "lkjhlkjh"}
	diffCommits, err := repository.Diff(newTag, oldTag, DiffOptions{})
	assert.Len(t, diffCommits, 0)
	assert.ErrorIs(t, err, ErrUnrecognizedHash)
}

func TestHead(t *testing.T) {
	head, err := repository.Head()
	assert.NoError(t, err)
	assert.Equal(t, commits[lenCommits-1].Hash, head)
}

func TestTag_LatestCommit(t *testing.T) {
	repository, commits, _, lenCommits := newRepo(false)
	tag, err := repository.Tag("mytag", TagOptions{})
	assert.NoError(t, err)
	assert.Equal(t, commits[lenCommits-1].Hash, tag.Hash)
	assert.Equal(t, "mytag", tag.Name)
}

func TestTag_SecondLatestCommit(t *testing.T) {
	repository, commits, _, lenCommits := newRepo(false)
	secondNewestCommit := commits[lenCommits-2]
	tag, err := repository.Tag("mytag", TagOptions{Hash: secondNewestCommit.Hash})
	assert.NoError(t, err)
	assert.Equal(t, secondNewestCommit.Hash, tag.Hash)
	assert.Equal(t, "mytag", tag.Name)
}

func TestTag_NoCommitHistory(t *testing.T) {
	repository, _, _, _ := newRepo(true)
	_, err := repository.Tag("mytag", TagOptions{})
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
}

func TestGetTag(t *testing.T) {
	tag, err := repository.GetTag("v1.0.0", GetTagOptions{})
	assert.NoError(t, err)
	assert.Equal(t, commits[1].Hash, tag.Hash)
	assert.Equal(t, "v1.0.0", tag.Name)
}

func TestGetTag_Module(t *testing.T) {
	tag, err := repository.GetTag("v0.0.0", GetTagOptions{Module: "subdir"})
	assert.NoError(t, err)
	assert.Equal(t, commits[0].Hash, tag.Hash)
	assert.Equal(t, "subdir/v0.0.0", tag.Name)
}

func TestGetTag_NotFound(t *testing.T) {
	tag, err := repository.GetTag("sdjask", GetTagOptions{})
	assert.ErrorIs(t, err, ErrTagNotFound)
	assert.Nil(t, tag)
}

func TestGetTags(t *testing.T) {
	mrTags, err := repository.GetTags(GetTagOptions{})
	assert.NoError(t, err)
	lenTags := len(mrTags)
	assert.Equal(t, lenCommits, lenTags)
	for i, tag := range mrTags {
		assert.Equal(t, tags[len(tags)-i-1].Hash, tag.Hash)
		assert.Equal(t, tags[len(tags)-i-1].Name, tag.Name)
	}
}

func TestGetTags_Module(t *testing.T) {
	mrTags, err := repository.GetTags(GetTagOptions{Module: "subdir"})
	assert.NoError(t, err)
	lenTags := len(mrTags)
	assert.Equal(t, 2, lenTags)

	assert.Equal(t, tags[len(tags)-1].Hash, mrTags[0].Hash)
	assert.Equal(t, tags[len(tags)-1].Name, mrTags[0].Name)

	assert.Equal(t, tags[0].Hash, mrTags[1].Hash)
	assert.Equal(t, tags[0].Name, mrTags[1].Name)
}

func TestGetTags_NoTags(t *testing.T) {
	repository, _, _, _ := newRepo(true)
	tags, err := repository.GetTags(GetTagOptions{})
	assert.NoError(t, err)
	lenTags := len(tags)
	assert.Equal(t, 0, lenTags)
}

func TestVersion_Gt(t *testing.T) {
	v1 := Version{version: "v1.0.0"}
	v2 := Version{version: "v1.1.0"}

	greater, err := v1.Gt(v2)
	assert.NoError(t, err)
	assert.False(t, greater)
}
