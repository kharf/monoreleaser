package monoreleaser_test

import (
	"testing"

	. "github.com/kharf/monoreleaser/internal"
)

func TestGithubReleaser_Release(t *testing.T) {
	NewGithubReleaser("kharf", "myrepo", UserSettings{})
}
