package engine

// Version is the single source of truth for the application version.
//
// It was previously hardcoded in three places that had already drifted apart:
// app.go's GetAppVersion() and wails.json both said "2.0.0" while the README,
// CHANGELOG and the shipped release archive said "2.5.0", so the About dialog
// and the tray tooltip reported a version that had not existed for two
// releases.
//
// Release builds override this at link time. `wails.json` is the canonical
// release-version source and build entrypoints pass it through `-ldflags`.
//
//	go build -ldflags="-X unbound/engine.Version=0.6.9"
//
// Keep this fallback in sync so direct Go and Wails builds remain truthful.
var Version = "0.6.9"

// StrategiesVersion is the single source of truth for the strategy catalog schema/version.
const StrategiesVersion = "2026.09.04"

// UserAgent is the HTTP User-Agent used for list updates and connectivity
// probes.
func UserAgent() string {
	return "Unbound/" + Version
}
