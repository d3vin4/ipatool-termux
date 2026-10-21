//go:build !android

package fairplay

import (
	"os"
	"testing"
)

// linkAlias creates a second name for an existing file. It reports whether the
// alias was created.
func linkAlias(t *testing.T, source, destination string) bool {
	t.Helper()

	if err := os.Link(source, destination); err != nil {
		t.Fatal(err)
	}

	return true
}
