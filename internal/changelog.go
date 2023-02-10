package monoreleaser

import (
	"fmt"
	"strings"
)

// A markdown formatted Changelog
type Changelog string

// Generates a markdown formatted Changelog based on Commits(Changes)
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
			if err := write("Breaking", &major, change); err != nil {
				return "", err
			}
		case Minor:
			if err := write("Minor", &minor, change); err != nil {
				return "", err
			}
		case Patch:
			if err := write("Patch", &patches, change); err != nil {
				return "", err
			}
		case Unknown:
			if err := write("Uncategorized", &unknowns, change); err != nil {
				return "", err
			}
		default:
			if err := write("Uncategorized", &unknowns, change); err != nil {
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
		if _, err := sb.WriteString("### " + header + "\n"); err != nil {
			return err
		}
	}
	if _, err := sb.WriteString(change.Message + "\n"); err != nil {
		return err
	}
	return nil
}
