package illsample_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestChangelog checks that the changelog has the shape the releases
// need: an [Unreleased] section at the top for what has not been
// released yet, and a compare link for every version.
//
// The release preparation turns the [Unreleased] section into the
// version being released. When the section is missing the step does
// nothing, and the release ships without its own section -- which
// happened once.
func TestChangelog(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatalf("read CHANGELOG.md: %v", err)
	}
	changelog := string(data)

	if !strings.Contains(changelog, "\n## [Unreleased]\n") {
		t.Error("the changelog has no [Unreleased] section")
	}

	heading := regexp.MustCompile(`(?m)^## \[([^\]]+)\]`)
	links := regexp.MustCompile(`(?m)^\[([^\]]+)\]: `)
	linkNames := map[string]bool{}
	for _, m := range links.FindAllStringSubmatch(changelog, -1) {
		linkNames[m[1]] = true
	}
	for _, m := range heading.FindAllStringSubmatch(changelog, -1) {
		name := m[1]
		if !linkNames[name] {
			t.Errorf("the changelog has no compare link for %q", name)
		}
	}
}
