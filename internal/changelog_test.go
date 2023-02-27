package monoreleaser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var cl Changelog

func BenchmarkGenerateChangelog(b *testing.B) {
	b.ReportAllocs()
	changes := Extract(commits)
	var c Changelog
	for i := 0; i < b.N; i++ {
		c, _ = GenerateChangelog(changes)
	}
	cl = c
}

func TestGenerateChangelog(t *testing.T) {
	changes := Extract(commits)
	changelog, _ := GenerateChangelog(changes)
	expected := Changelog(`# What's Changed
## 💔 Breaking
- feat!: major change

## 🚀 Minor
- feat: oldest
- style: change
- chore: change
- test: change
- refactor: change
- ci: change
- build: change
- docs: newest

## 🐛 Patch
- fix: patch change

`)

	assert.Equal(t, expected, changelog)
}