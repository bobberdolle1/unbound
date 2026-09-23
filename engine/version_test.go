package engine

import (
	"encoding/json"
	"runtime"
	"testing"
)

func TestCurrentBuildIdentitySchema(t *testing.T) {
	identity := CurrentBuildIdentity()
	if identity.Version == "" {
		t.Fatal("version is empty")
	}
	if identity.OS == "" || identity.OS != runtime.GOOS {
		t.Fatalf("os = %q, want %q", identity.OS, runtime.GOOS)
	}
	if identity.Arch == "" || identity.Arch != runtime.GOARCH {
		t.Fatalf("arch = %q, want %q", identity.Arch, runtime.GOARCH)
	}

	payload, err := json.Marshal(identity)
	if err != nil {
		t.Fatalf("marshal build identity: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal build identity: %v", err)
	}
	for _, name := range []string{"version", "commit", "dirty", "channel", "os", "arch"} {
		if _, ok := fields[name]; !ok {
			t.Errorf("build identity omitted %q: %s", name, payload)
		}
	}
}

func TestCurrentBuildIdentityDistinguishesReleaseAndDevelopment(t *testing.T) {
	originalVersion, originalCommit := Version, BuildCommit
	originalDirty, originalChannel := BuildDirty, BuildChannel
	t.Cleanup(func() {
		Version, BuildCommit = originalVersion, originalCommit
		BuildDirty, BuildChannel = originalDirty, originalChannel
	})

	Version = "0.6.9"
	BuildCommit = "bb4900fcc8e96564c4d21607dcd755a6350a185c"
	BuildDirty = "false"
	BuildChannel = "release"
	release := CurrentBuildIdentity()
	if release.Channel != "release" || release.Commit != BuildCommit || release.Dirty == nil || *release.Dirty {
		t.Fatalf("release identity = %+v", release)
	}

	Version = "0.7.0-dev"
	BuildCommit = "d437963"
	BuildDirty = "true"
	BuildChannel = "development"
	development := CurrentBuildIdentity()
	if development.Channel != "development" || development.Commit != BuildCommit || development.Dirty == nil || !*development.Dirty {
		t.Fatalf("development identity = %+v", development)
	}
}

func TestCurrentBuildIdentityRejectsInvalidInjectedMetadata(t *testing.T) {
	originalCommit, originalDirty, originalChannel := BuildCommit, BuildDirty, BuildChannel
	t.Cleanup(func() {
		BuildCommit, BuildDirty, BuildChannel = originalCommit, originalDirty, originalChannel
	})

	BuildCommit = "not-a-commit"
	BuildDirty = "not-a-bool"
	BuildChannel = "preview"
	identity := CurrentBuildIdentity()
	if identity.Commit != "unknown" || identity.Dirty != nil || identity.Channel != "development" {
		t.Fatalf("normalized identity = %+v", identity)
	}
}
