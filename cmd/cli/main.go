package main

import (
	"log"
	"strings"

	"github.com/go-git/go-git/v5"
	monoreleaser "github.com/kharf/monoreleaser/internal"
)

func main() {
	dir := "/home/kharf/code/campaign-orchestration"

	r, err := git.PlainOpen(dir)
	if err != nil {
		log.Fatal(err)
	}

	monoRepo := monoreleaser.GoGitRepository{Repository: r}
	newTag := monoRepo.Tag("commons/v0.17.0")
	oldTag := monoRepo.Tag("commons/v0.14.0")
	filter := func(path string) bool {
		return strings.HasPrefix(path, "commons/")
	}

	diffs, err := monoRepo.Diff(newTag, oldTag, monoreleaser.DiffOptions{PathFilter: filter})

	if err != nil {
		log.Fatal(err)
	}

	changes := monoreleaser.Extract(diffs)
	for _, change := range changes {
		log.Println(change.Hash)
		log.Println(change.Semantic)
		log.Println(change.Message)
	}
}
