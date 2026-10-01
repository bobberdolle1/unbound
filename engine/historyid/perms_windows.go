//go:build windows

package historyid

import (
	"os"

	"golang.org/x/sys/windows"
)

// contextKeyFileMode on Windows is largely advisory: Go passes the perm through
// to CreateFile, which maps it only to the read-only attribute. It does NOT
// produce a restrictive DACL, so a key created with this mode alone inherits
// whatever ACL the containing directory happens to have.
//
// The product config directory is normally user-owned, but the key must not
// depend on where the caller put it. hardenContextKeyFile therefore applies an
// explicit owner-only DACL. This is ordinary per-user file security, not a new
// privileged service: it grants full control to the current user and to SYSTEM,
// and to nobody else.
const contextKeyFileMode = os.FileMode(0o600)

// fileAllAccessMask is the Win32 FILE_ALL_ACCESS standard-rights mask. The
// security package does not export a name for it, so it is spelled out here.
const fileAllAccessMask = 0x001F01FF

// hardenContextKeyFile restricts the local HMAC key to its owner.
//
// A key we cannot harden is reported as an error rather than used. Silently
// accepting a world-readable key would degrade exactly the privacy property the
// context identity depends on: the key is what stops an offline attacker with
// the ledger from brute-forcing the context fingerprint.
func hardenContextKeyFile(path string) error {
	// GetCurrentProcessToken returns a pseudo-handle that must not be closed.
	tokenUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	userSID := tokenUser.User.Sid

	// FA grants full control. SY is SYSTEM, granted so backup and service-level
	// access still work, which is the conventional expectation for a per-user
	// secret on Windows. No other trustee receives an ACE, so every other local
	// principal is denied. "P" makes the DACL protected, so an inherited
	// permissive ACE from the containing directory cannot be merged back in -
	// without it, restricting the DACL would achieve nothing.
	//
	// The SDDL form is used deliberately: hand-building an ACL through
	// ACLFromEntries is easy to get subtly wrong, and a silently malformed DACL
	// would be worse than no DACL at all.
	sddl := "D:P(A;;FA;;;" + userSID.String() + ")(A;;FA;;;SY)"
	descriptor, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		userSID, nil, dacl, nil,
	)
}
