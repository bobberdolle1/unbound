//go:build windows

package historyid

import "os"

// contextKeyFileMode on Windows relies on the user-owned config directory
// (AppData/<user>/Unbound), which is already access-controlled per user, plus
// the inherited DACL. No new privileged service and no ACL manipulation is
// introduced merely to protect this file; that would be a far larger privilege
// change than the asset itself warrants.
const contextKeyFileMode = os.FileMode(0o600)
