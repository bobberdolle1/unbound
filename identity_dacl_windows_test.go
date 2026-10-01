//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// assertKeyRestrictedToOwner proves the local HMAC key is not reachable by other
// local principals. It reads the real DACL and resolves every ACE to a SID, so
// it fails if any trustee other than the current user or SYSTEM is granted
// access. Asserting the file mode instead would be meaningless on Windows: Go
// maps the perm to the read-only attribute, not to a DACL.
func assertKeyRestrictedToOwner(path string) error {
	allowed := map[string]bool{}

	tokenUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	allowed[tokenUser.User.Sid.String()] = true

	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	allowed[systemSID.String()] = true

	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	acl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	if acl == nil {
		return fmt.Errorf("security descriptor for %s has a NULL DACL", path)
	}
	if acl.AceCount == 0 {
		return fmt.Errorf("context key has an empty DACL, so the restriction cannot be verified")
	}
	for index := range int(acl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, uint32(index), &ace); err != nil {
			return err
		}
		if ace == nil {
			return fmt.Errorf("ACE %d could not be read", index)
		}
		// The SID begins immediately after the fixed ACE header, at SidStart.
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !allowed[sid.String()] {
			return fmt.Errorf("unexpected ACE %d grants access to %s", index, sid.String())
		}
	}
	return nil
}
