package monoreleaser

import (
	"errors"
	"io"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Iterator is an object that enables to traverse lists lazily.
type Iter[T any] interface {
	Next() (T, error)
}

// GenericIter traverses lists by the provided NextFunc.
type GenericIter[T any] struct {
	NextFunc func() (T, error)
}

var _ Iter[any] = GenericIter[any]{}

func (iter GenericIter[T]) Next() (T, error) {
	return iter.NextFunc()
}

type Commit struct {
	Hash    string
	Message string
}

type Tag struct {
	Name string
	Hash string
}

// A vcs Repository.
type Repository interface {
	// Short name of the repository, e.g. in github http://github.com/owner/name it is the path parameter after the owner.
	Name() string
	// Head always refers to the most recent commit on the current branch.
	Head() (string, error)
	// History retrieves a repository's Commit log from a specific Commit hash as an Iter.
	// History order is from newest(first) to lowest(last).
	History(opts HistoryOptions) (*GenericIter[*Commit], error)
	// Tag creates a specific important point(Tag) in a repository's history.
	Tag(version string, opts TagOptions) (*Tag, error)
	// GetTag retrieves a specific important point(Tag) from a repository's history.
	GetTag(version string, opts GetTagOptions) (*Tag, error)
	// GetTags retrieves important points(Tags) from a repository's history, sorted by name (highest first).
	GetTags(opts GetTagOptions) ([]Tag, error)
	// Diff compares histories of two Tags and returns the Commits in between.
	// If no olderTag provided, the commit history reachable from newerTag will be returned.
	Diff(newerTag Tag, olderTag *Tag, opts DiffOptions) ([]*Commit, error)
}

type GoGitRepository struct {
	name       string
	repository *git.Repository
}

var _ Repository = GoGitRepository{}

func NewGoGitRepository(name string, repository *git.Repository) GoGitRepository {
	return GoGitRepository{
		name:       name,
		repository: repository,
	}
}

// Optional parameters for getting the history.
type HistoryOptions struct {
	// When the Hash option is set the log will only contain commits reachable from it.
	// If this option is not set, HEAD will be used as the Hash.
	Hash string
	// A Module is just an application (directory) inside a mono repository.
	Module string
}

func (repo GoGitRepository) Name() string {
	return repo.name
}

func (repo GoGitRepository) Head() (string, error) {
	head, err := repo.repository.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

var (
	ErrEndOfHistory     = errors.New("no more commits available")
	ErrUnrecognizedHash = errors.New("unrecognized hash provided")
)

func (repo GoGitRepository) History(opts HistoryOptions) (*GenericIter[*Commit], error) {
	var filter func(path string) bool
	if opts.Module != "" {
		filter = func(path string) bool {
			return strings.HasPrefix(path, modulePrefix(opts.Module))
		}
	}
	var from plumbing.Hash
	if opts.Hash != "" {
		from = plumbing.NewHash(opts.Hash)
		if from == plumbing.ZeroHash {
			return nil, ErrUnrecognizedHash
		}
	}

	newTagCommitIter, err := repo.repository.Log(&git.LogOptions{From: from, PathFilter: filter})

	if err != nil {
		return nil, err
	}

	return &GenericIter[*Commit]{
		NextFunc: func() (*Commit, error) {
			commit, err := newTagCommitIter.Next()
			if errors.Is(err, io.EOF) {
				return nil, ErrEndOfHistory
			}

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

func modulePrefix(module string) string {
	return module + "/"
}

// Optional parameters for tagging.
type TagOptions struct {
	// The hash of the Commit to tag.
	// If this option is not set, HEAD will be used as the Hash.
	Hash string
	// A Module is just an application (directory) inside a mono repository.
	Module string
}

func (repo GoGitRepository) Tag(version string, opts TagOptions) (*Tag, error) {
	history, err := repo.History(HistoryOptions{Hash: opts.Hash, Module: opts.Module})
	if err != nil {
		return nil, err
	}

	latestCommit, err := history.Next()
	if err != nil {
		return nil, err
	}

	hash := plumbing.NewHash(latestCommit.Hash)
	tagName := tagName(version, opts.Module)
	tag, err := repo.repository.CreateTag(tagName, hash, nil)
	if err != nil {
		return nil, err
	}

	return &Tag{
		Name: tag.Name().Short(),
		Hash: tag.Hash().String(),
	}, nil
}

// Optional parameters for getting a tag.
type GetTagOptions struct {
	// A Module is just an application (directory) inside a mono repository.
	Module string
}

var ErrTagNotFound = errors.New("tag not found")

func (repo GoGitRepository) GetTag(version string, opts GetTagOptions) (*Tag, error) {
	tag, err := repo.repository.Tag(tagName(version, opts.Module))
	if errors.Is(err, git.ErrTagNotFound) {
		return nil, ErrTagNotFound
	}

	if err != nil {
		return nil, err
	}

	return &Tag{
		Name: tag.Name().Short(),
		Hash: tag.Hash().String(),
	}, nil
}

func (repo GoGitRepository) GetTags(opts GetTagOptions) ([]Tag, error) {
	tags, err := repo.repository.Tags()
	if err != nil {
		return nil, err
	}

	var prefix string
	if opts.Module != "" {
		prefix = modulePrefix(opts.Module)
	}

	var moduleTags []Tag
	if err := tags.ForEach(func(ref *plumbing.Reference) error {
		if strings.HasPrefix(ref.Name().Short(), prefix) {
			moduleTags = append(moduleTags, Tag{
				Name: ref.Name().Short(),
				Hash: ref.Hash().String(),
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}

	sort.Slice(moduleTags, func(i, j int) bool {
		namesWithoutPrefix1 := strings.SplitAfter(moduleTags[i].Name, "/")
		nameWithoutPrefix1 := namesWithoutPrefix1[len(namesWithoutPrefix1)-1]

		namesWithoutPrefix2 := strings.SplitAfter(moduleTags[j].Name, "/")
		nameWithoutPrefix2 := namesWithoutPrefix2[len(namesWithoutPrefix2)-1]

		return nameWithoutPrefix1 > nameWithoutPrefix2
	})

	return moduleTags, nil
}

func tagName(name string, module string) string {
	var tagName string

	if module == "" {
		tagName = name
	} else {
		tagName = modulePrefix(module) + name
	}

	return tagName
}

// Optional options for getting the commit history diff.
type DiffOptions struct {
	// A Module is just an application (directory) inside a mono repository.
	Module string
}

func (repo GoGitRepository) Diff(newerTag Tag, olderTag *Tag, opts DiffOptions) ([]*Commit, error) {
	historyIter, err := repo.History(HistoryOptions{Hash: newerTag.Hash, Module: opts.Module})

	if err != nil {
		return []*Commit{}, err
	}

	commitDiffs := make([]*Commit, 0, 20)
	for {
		newCommit, err := historyIter.Next()
		if err != nil && !errors.Is(err, ErrEndOfHistory) {
			return []*Commit{}, err
		}

		if newCommit == nil || errors.Is(err, ErrEndOfHistory) {
			break
		}

		if olderTag != nil && newCommit.Hash == olderTag.Hash {
			break
		}
		commitDiffs = append(commitDiffs, newCommit)
	}

	return commitDiffs, nil
}
