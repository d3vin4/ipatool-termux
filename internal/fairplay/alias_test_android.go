//go:build android

package fairplay

import "testing"

// linkAlias cannot create a hard link because Android denies the link syscall
// to unprivileged applications. The symlink case of the alias test covers the
// same alias detection on this platform.
func linkAlias(_ *testing.T, _, _ string) bool {
	return false
}
