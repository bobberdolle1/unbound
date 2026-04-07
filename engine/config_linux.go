//go:build linux
// +build linux

package engine

// applyAutoStartSetting is a no-op on Linux.
// Linux autostart can be implemented via .desktop files in ~/.config/autostart/
// but is not implemented in this version.
func applyAutoStartSetting(enable bool) error {
	return nil
}
