//go:build !windows

package main

// assertKeyRestrictedToOwner is a no-op off Windows. Unix carries the
// restriction in the file mode bits, which the smoke test asserts directly;
// there is no DACL to inspect. The Windows build has a real implementation that
// decodes the ACL and rejects any unexpected trustee.
func assertKeyRestrictedToOwner(string) error { return nil }
