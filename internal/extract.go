package monoreleaser

import "strings"

type Semantic string
type Type string

const (
	Unknown Semantic = "unknown"
	Major   Semantic = "major"
	Minor   Semantic = "minor"
	Patch   Semantic = "patch"

	CommitSeperator        = ":"
	ScopeStart             = "("
	BreakingIndicator      = "!"
	UnknownType       Type = "unknown"
	Fix               Type = "fix"
	Feature           Type = "feat"
	Build             Type = "build"
	Chore             Type = "chore"
	Ci                Type = "ci"
	Docs              Type = "docs"
	Style             Type = "style"
	Refactor          Type = "refactor"
	Perf              Type = "perf"
	Test              Type = "test"
)

// Change is an interpreted Commit.
type Change struct {
	Message  string
	Semantic Semantic
	Hash     string
}

// Inspects a list of Commits and transform each of them into Changes by extracting the semantic of the commit message.
func Extract(commits []*Commit) []Change {
	changes := make([]Change, 0, len(commits))
	for _, commit := range commits {
		changes = append(changes, Change{Message: commit.Message, Hash: commit.Hash, Semantic: extractSemantic(commit.Message)})
	}
	return changes
}

func extractSemantic(message string) Semantic {
	typeAndScope, _, found := strings.Cut(message, CommitSeperator)
	if !found {
		return Unknown
	}

	if strings.Contains(typeAndScope, BreakingIndicator) {
		return Major
	}

	typeStr, _, _ := strings.Cut(typeAndScope, ScopeStart)

	realType := Type(typeStr)
	switch realType {
	case Fix:
		return Patch
	case Feature, Build, Chore, Ci, Docs, Style, Refactor, Perf, Test:
		return Minor
	case UnknownType:
		return Unknown
	default:
		return Unknown
	}
}
