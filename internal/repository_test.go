package monoreleaser_test

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	. "github.com/kharf/monoreleaser/internal"
	"github.com/stretchr/testify/assert"
)

var (
	repository Repository
	commits    = []*Commit{}

	ErrNotFound = errors.New("not found")
	lenCommits  int
)

func TestMain(m *testing.M) {
	gitRepository, _ := git.Init(memory.NewStorage(), memfs.New())
	workTree, err := gitRepository.Worktree()
	if err != nil {
		log.Panic(err)
	}
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
		_, err = gitRepository.CreateTag(fmt.Sprintf("v%v", i), lastCommitHash, nil)
		if err != nil {
			log.Panic(err)
		}

		commits = append(commits, &Commit{Hash: lastCommitHash.String(), Message: message})
	}

	lenCommits = len(commits)

	repository = GoGitRepository{gitRepository}

	os.Exit(m.Run())
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
	diffCommits, _ := repository.Diff(newTag, oldTag, DiffOptions{PathFilter: func(path string) bool {
		return strings.HasPrefix(path, "subdir/")
	}})

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
