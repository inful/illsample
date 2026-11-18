package illsample_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestChangelog checks that the changelog and its compare links
// agree: every version heading has a compare link, and every compare
// link but [Unreleased] has a version heading.
//
// The release preparation turns the [Unreleased] section into the
// version being released and adds a compare link for it. When the
// [Unreleased] section is missing the step does nothing, and the
// release ships with a compare link and no section -- which happened
// once.
func TestChangelog(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatalf("read CHANGELOG.md: %v", err)
	}
	changelog := string(data)

	heading := regexp.MustCompile(`(?m)^## \[([^\]]+)\]`)
	links := regexp.MustCompile(`(?m)^\[([^\]]+)\]: `)

	headings := map[string]bool{}
	for _, m := range heading.FindAllStringSubmatch(changelog, -1) {
		headings[m[1]] = true
	}
	linkNames := map[string]bool{}
	for _, m := range links.FindAllStringSubmatch(changelog, -1) {
		linkNames[m[1]] = true
	}

	for name := range headings {
		if !linkNames[name] {
			t.Errorf("the changelog has no compare link for %q", name)
		}
	}
	for name := range linkNames {
		// The [Unreleased] link points at the next release and has
		// no section of its own until there is something to put in
		// it.
		if name == "Unreleased" {
			continue
		}
		if !headings[name] {
			t.Errorf("the changelog has a compare link for %q but no section", name)
		}
	}
	if !strings.Contains(changelog, "[Unreleased]:") {
		t.Error("the changelog has no [Unreleased] compare link")
	}
}
