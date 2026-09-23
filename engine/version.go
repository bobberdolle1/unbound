package engine

import (
	"regexp"
	"runtime"
	"strconv"
)

// Version is the application version. Release builds override it at link time.
// Direct source builds intentionally identify as the next development version.
var Version = "0.7.0-dev"

// BuildCommit, BuildDirty, and BuildChannel are injected at build time. They
// deliberately never consult a packaged .git directory at runtime.
var (
	BuildCommit  = "unknown"
	BuildDirty   = "unknown"
	BuildChannel = "development"
)

var buildCommitPattern = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

// BuildIdentity is the machine-readable identity emitted by --version --json.
// Dirty is nil when an ad-hoc build did not supply source-state metadata.
type BuildIdentity struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Dirty   *bool  `json:"dirty"`
	Channel string `json:"channel"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// CurrentBuildIdentity returns build metadata embedded by the build entrypoint.
func CurrentBuildIdentity() BuildIdentity {
	return BuildIdentity{
		Version: Version,
		Commit:  normalizedBuildCommit(BuildCommit),
		Dirty:   parseBuildDirty(BuildDirty),
		Channel: normalizedBuildChannel(BuildChannel),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
}

func normalizedBuildCommit(commit string) string {
	if buildCommitPattern.MatchString(commit) {
		return commit
	}
	return "unknown"
}

func parseBuildDirty(value string) *bool {
	dirty, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}
	return &dirty
}

func normalizedBuildChannel(channel string) string {
	if channel == "release" {
		return channel
	}
	return "development"
}

// StrategiesVersion is the single source of truth for the strategy catalog schema/version.
const StrategiesVersion = "2026.09.04"

// UserAgent is the HTTP User-Agent used for list updates and connectivity
// probes.
func UserAgent() string {
	return "Unbound/" + Version
}
