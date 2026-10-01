//go:build !windows

package historyid

import "os"

// contextKeyFileMode restricts the local HMAC key to its owner on Unix. The key
// is what makes the context identity non-guessable, so it must never be world
// readable.
const contextKeyFileMode = os.FileMode(0o600)
