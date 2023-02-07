package main

import (
	"log"

	"github.com/go-git/go-git/v5"
	monoreleaser "github.com/kharf/monoreleaser/internal"
)

func main() {
	dir := "/home/kharf/code/campaign-orchestration"

	r, err := git.PlainOpen(dir)
	if err != nil {
		log.Fatal(err)
	}

	newestTag, err := r.Tag("commons/v0.17.0")
	if err != nil {
		log.Fatal(err)
	}

	latestTag, err := r.Tag("commons/v0.16.1")
	if err != nil {
		log.Fatal(err)
	}

	diffs, err := monoreleaser.Diff(r, newestTag, latestTag)

	if err != nil {
		log.Fatal(err)
	}

	for _, diff := range diffs {
		log.Println(diff.Message)
	}
}
