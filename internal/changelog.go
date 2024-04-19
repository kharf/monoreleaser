package monoreleaser

import (
	"bufio"
	"fmt"
	"strings"
)

type Emoji string

const (
	BreakingHeart Emoji = "\U0001F494"
	Rocket        Emoji = "\U0001F680"
	Bug           Emoji = "\U0001F41B"
	Package       Emoji = "\U0001f4E6"
)

// A markdown formatted Changelog.
type Changelog string

// Generates a markdown formatted Changelog based on Commits(Changes).
func GenerateChangelog(changes []Change) (Changelog, error) {
	var sb strings.Builder
	sb.WriteString("# What's Changed")

	var major strings.Builder
	var minor strings.Builder
	var patches strings.Builder
	var unknowns strings.Builder

	for _, change := range changes {
		change := change
		switch change.Semantic {
		case Major:
			if err := write(string(BreakingHeart)+" Breaking", &major, change); err != nil {
				return "", err
			}
		case Minor:
			if err := write(string(Rocket)+" Minor", &minor, change); err != nil {
				return "", err
			}
		case Patch:
			if err := write(string(Bug)+" Patch", &patches, change); err != nil {
				return "", err
			}
		case Unknown:
			if err := write(string(Package)+" Uncategorized", &unknowns, change); err != nil {
				return "", err
			}
		default:
			if err := write(string(Package)+" Uncategorized", &unknowns, change); err != nil {
				return "", err
			}
		}
	}

	if _, err := sb.WriteString(fmt.Sprintf("\n%s", major.String())); err != nil {
		return "", err
	}
	if _, err := sb.WriteString(fmt.Sprintf("\n%s", minor.String())); err != nil {
		return "", err
	}
	if _, err := sb.WriteString(fmt.Sprintf("\n%s", patches.String())); err != nil {
		return "", err
	}
	if _, err := sb.WriteString(fmt.Sprintf("\n%s", unknowns.String())); err != nil {
		return "", err
	}

	return Changelog(sb.String()), nil
}

func write(header string, sb *strings.Builder, change Change) error {
	if sb.Len() == 0 {
		if _, err := sb.WriteString("## " + header + "\n"); err != nil {
			return err
		}
	}
	bufferedReader := bufio.NewScanner(strings.NewReader(change.Message))
	bufferedReader.Scan()
	if _, err := sb.WriteString("- " + bufferedReader.Text() + "\n"); err != nil {
		return err
	}
	for bufferedReader.Scan() {
		line := bufferedReader.Text()
		if _, err := sb.WriteString("\t" + line + "\n"); err != nil {
			return err
		}
	}
	if err := bufferedReader.Err(); err != nil {
		return err
	}
	return nil
}
