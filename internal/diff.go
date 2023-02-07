package monoreleaser

import (
	"log"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func Diff(repository *git.Repository, newestTag, latestTag *plumbing.Reference) ([]object.Commit, error) {
	newTagCommitIter, err := repository.Log(&git.LogOptions{From: newestTag.Hash(), PathFilter: func(path string) bool {
		return strings.HasPrefix(path, "commons/")
	}})

	if err != nil {
		log.Fatal(err)
	}

	latestTagCommitIter, err := repository.Log(&git.LogOptions{From: latestTag.Hash(), PathFilter: func(path string) bool {
		return strings.HasPrefix(path, "commons/")
	}})

	if err != nil {
		log.Fatal(err)
	}

	latestCommit, err := latestTagCommitIter.Next()
	if err != nil {
		return []object.Commit{}, err
	}

	commitDiffs := make([]object.Commit, 0)

	for {
		newCommit, err := newTagCommitIter.Next()
		if err != nil {
			return []object.Commit{}, err
		}

		if newCommit.Hash == latestCommit.Hash {
			break
		}

		commitDiffs = append(commitDiffs, *newCommit)
	}

	return commitDiffs, nil
}