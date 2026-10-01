//go:build !windows

package historyid

import "os"

// contextKeyFileMode restricts the local HMAC key to its owner on Unix. The key
// is what makes the context identity non-guessable, so it must never be world
// readable.
const contextKeyFileMode = os.FileMode(0o600)

// hardenContextKeyFile re-asserts owner-only permissions. Creation already
// applies 0600, but a pre-existing key file could have been created with looser
// bits, so the restriction is enforced on every read path too.
func hardenContextKeyFile(path string) error {
	return os.Chmod(path, contextKeyFileMode)
}
