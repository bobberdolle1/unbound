package providers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"unbound/engine/internal/configdir"
)

func TestMergeUniquePreservesBaseAndAppendsNew(t *testing.T) {
	base := []string{"discord.com", "discord.gg"}
	extra := []string{"discord.gg", "gateway.discord.gg", "discord.com"}
	got := mergeUnique(base, extra)
	want := []string{"discord.com", "discord.gg", "gateway.discord.gg"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestMergeUniqueHandlesEmptySlices(t *testing.T) {
	if got := mergeUnique(nil, []string{"a.com"}); len(got) != 1 || got[0] != "a.com" {
		t.Fatalf("got %v", got)
	}
	if got := mergeUnique([]string{"a.com"}, nil); len(got) != 1 || got[0] != "a.com" {
		t.Fatalf("got %v", got)
	}
}

func TestDoSyncHostlistsHonorsConfigDirOverride(t *testing.T) {
	tmpConfig := t.TempDir()
	restore := configdir.SetForTest(tmpConfig)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("custom-test.example\n"))
	}))
	defer srv.Close()

	origSources := defaultHostlistSources
	defaultHostlistSources = []HostlistSource{
		{
			Name:            "TestSource",
			RemoteURL:       srv.URL,
			Filename:        "test_source.txt",
			FallbackDomains: "fallback.example",
		},
	}
	defer func() { defaultHostlistSources = origSources }()

	if err := doSyncHostlists(); err != nil {
		t.Fatalf("doSyncHostlists failed: %v", err)
	}

	targetFile := filepath.Join(tmpConfig, "lists", "test_source.txt")
	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("expected synced file in overridden lists dir %s: %v", targetFile, err)
	}
	content := string(data)
	if !strings.Contains(content, "custom-test.example") || !strings.Contains(content, "fallback.example") {
		t.Fatalf("unexpected synced content: %q", content)
	}
}
