//go:build darwin

package engine

const (
	MacOSPlistFilename = "com.bobberdolle1.unbound.plist"
)

func applyAutoStartSetting(enable bool) error {
	// On macOS, autostart is managed via launchd (StartupMacOS provider).
	// We call the appropriate EnableAutoStart/DisableAutoStart functions.
	if enable {
		return EnableAutoStart()
	} else {
		return DisableAutoStart()
	}
}
