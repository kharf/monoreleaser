package monoreleaser_test

import (
	"strings"
	"testing"

	. "github.com/kharf/monoreleaser/internal"
	"github.com/stretchr/testify/assert"
)

func TestExtract(t *testing.T) {
	changes := Extract(commits)

	assert.Len(t, changes, lenCommits)
	for i, change := range changes {
		assert.Equal(t, commits[i].Hash, change.Hash)
		assert.Equal(t, commits[i].Message, change.Message)

		typeAndScope, _, found := strings.Cut(change.Message, CommitSeperator)
		if !found {
			assert.Equal(t, Unknown, change.Semantic)
			continue
		}

		if strings.Contains(typeAndScope, BreakingIndicator) {
			assert.Equal(t, Major, change.Semantic)
			continue
		}

		typeStr, _, _ := strings.Cut(typeAndScope, ScopeStart)

		realType := Type(typeStr)
		switch realType {
		case Fix:
			assert.Equal(t, Patch, change.Semantic)
			continue
		case Feature, Build, Chore, Ci, Docs, Style, Refactor, Perf, Test:
			assert.Equal(t, Minor, change.Semantic)
			continue
		default:
			assert.Equal(t, Unknown, change.Semantic)
			continue
		}
	}
}