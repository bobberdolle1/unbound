package engine

// Version is the single source of truth for the application version.
//
// It was previously hardcoded in three places that had already drifted apart:
// app.go's GetAppVersion() and wails.json both said "2.0.0" while the README,
// CHANGELOG and the shipped release archive said "2.5.0", so the About dialog
// and the tray tooltip reported a version that had not existed for two
// releases.
//
// Release builds override this at link time:
//
//	go build -ldflags="-X unbound/engine.Version=0.1.0-refresh"
//
// Keep the default in step with the newest CHANGELOG entry so plain
// `go build` and `wails build` still report something truthful.
var Version = "0.2.0"

// UserAgent is the HTTP User-Agent used for list updates and connectivity
// probes.
func UserAgent() string {
	return "Unbound/" + Version
}
