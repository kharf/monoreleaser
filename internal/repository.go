package monoreleaser

import (
	"log"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type Iter[T any] struct {
	NextFunc func() (T, error)
}

func (iter Iter[T]) Next() (T, error) {
	return iter.NextFunc()
}

type Commit struct {
	Hash    string
	Message string
}

type Tag struct {
	Hash string
}

// A vcs Repository.
type Repository interface {
	// History retrieves a repository's Commit log from a specific Commit hash as an Iter.
	// History order is from newest(first) to lowest(last)
	History(opts HistoryOptions) (*Iter[*Commit], error)
	// Tag retrieves a specific important point(Tag) from a repository's history:
	Tag(name string) Tag
	// Diff compares histories of two Tags and returns the Commits in between.
	Diff(newerTag, olderTag Tag, opts DiffOptions) ([]*Commit, error)
}

type GoGitRepository struct {
	*git.Repository
}

var _ Repository = (*GoGitRepository)(nil)

type HistoryOptions struct {
	Hash       string
	PathFilter func(path string) bool
}

func (repository GoGitRepository) History(opts HistoryOptions) (*Iter[*Commit], error) {
	newTagCommitIter, err := repository.Log(&git.LogOptions{From: plumbing.NewHash(opts.Hash), PathFilter: opts.PathFilter})

	if err != nil {
		return nil, err
	}

	return &Iter[*Commit]{
		NextFunc: func() (*Commit, error) {
			commit, err := newTagCommitIter.Next()
			if err != nil {
				return nil, err
			}

			return &Commit{
				Hash:    commit.Hash.String(),
				Message: commit.Message,
			}, nil
		},
	}, nil
}

func (repository GoGitRepository) Tag(name string) Tag {
	tag, err := repository.Repository.Tag(name)
	if err != nil {
		log.Fatal(err)
	}

	return Tag{
		Hash: tag.Hash().String(),
	}
}

type DiffOptions struct {
	PathFilter func(path string) bool
}

func (repository GoGitRepository) Diff(newerTag, olderTag Tag, opts DiffOptions) ([]*Commit, error) {
	historyIter, err := repository.History(HistoryOptions{newerTag.Hash, opts.PathFilter})

	if err != nil {
		return nil, err
	}

	commitDiffs := make([]*Commit, 0, 20)
	for {
		newCommit, err := historyIter.Next()
		if err != nil {
			return []*Commit{}, err
		}

		if newCommit.Hash == olderTag.Hash {
			break
		}

		commitDiffs = append(commitDiffs, newCommit)
	}

	return commitDiffs, nil
}
