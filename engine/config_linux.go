//go:build linux

package engine

// applyAutoStartSetting persists the autostart preference.
//
// This used to be a no-op with a comment saying .desktop autostart "is not
// implemented in this version", so SaveSettings silently discarded the user's
// choice. See startup_linux.go for the implementation.
func applyAutoStartSetting(enable bool) error {
	if enable {
		return EnableAutoStart()
	}
	return DisableAutoStart()
}
