package main

// Hermetic coverage for bounded service-graph managed intent persistence.
//
// The load-time contract under test is fail-closed: anything malformed, duplicated,
// oversized, or version-mismatched is rejected rather than repaired. The privacy
// test is equally load-bearing: the durable projection must have no field capable
// of expressing packet authority.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"unbound/engine"
	"unbound/engine/autotunevnext"
)

func validGraphState() persistedVNextGraphState {
	return persistedVNextGraphState{
		SchemaVersion:    vNextGraphSchema,
		Enabled:          true,
		ServiceID:        "svc-1",
		Backend:          "zapret2/windows",
		GraphFingerprint: "graphfp1",
		CatalogIdentity:  "catalog1",
		SavedAt:          time.Now().UTC().Format(time.RFC3339Nano),
		Nodes: []persistedGraphNode{
			{NodeID: "ENTRY", Role: "ENTRY", Required: true, Target: "https://entry.example/generate_204", StrategyID: "s1", TemplateIdentity: "t1", Fingerprint: "fp1"},
			{NodeID: "MEDIA", Role: "MEDIA", Required: true, Target: "https://media.example/", StrategyID: "s1", TemplateIdentity: "t1", Fingerprint: "fp1"},
		},
	}
}

func TestGraphManagedStateRoundTrip(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()

	want := validGraphState()
	if err := saveVNextGraphState(want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok, err := loadVNextGraphState()
	if err != nil || !ok {
		t.Fatalf("load ok=%v err=%v", ok, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
	if err := clearVNextGraphState(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, ok, err := loadVNextGraphState(); ok || err != nil {
		t.Fatalf("after clear ok=%v err=%v", ok, err)
	}
}

// The durable projection must not be able to express packet authority at all.
func TestGraphManagedStatePersistsNoAddressBearingField(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()

	if err := saveVNextGraphState(validGraphState()); err != nil {
		t.Fatalf("save: %v", err)
	}
	path, err := getVNextGraphStatePath()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	text := string(raw)
	for _, forbidden := range []string{"192.168.", "10.0.0.", "172.16.", "127.0.0.1", "fd00:", "2001:db8", "edge", "ipset", "hostlist", "--filter", "argv", "pid", "queue", "token"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Fatalf("graph managed state leaked forbidden token %q in:\n%s", forbidden, text)
		}
	}
	// Also assert structurally: no field name may look address-bearing.
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	forbiddenKeys := []string{"ip", "ips", "addresses", "scope_edges", "edges", "dns", "resolved", "filter", "argv", "pid", "queue", "token", "divert", "nft"}
	walkKeys(generic, "", forbiddenKeys, t)
}

func walkKeys(value any, prefix string, forbidden []string, t *testing.T) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			lower := strings.ToLower(key)
			for _, bad := range forbidden {
				if lower == bad {
					t.Fatalf("graph managed state exposes address-bearing key %q", prefix+key)
				}
			}
			walkKeys(child, prefix+key+".", forbidden, t)
		}
	case []any:
		for _, child := range typed {
			walkKeys(child, prefix, forbidden, t)
		}
	}
}

func TestGraphManagedStateRejectsInvalidInputs(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*persistedVNextGraphState)
	}{
		{"schema mismatch", func(s *persistedVNextGraphState) { s.SchemaVersion = 99 }},
		{"missing service id", func(s *persistedVNextGraphState) { s.ServiceID = "" }},
		{"missing backend", func(s *persistedVNextGraphState) { s.Backend = "" }},
		{"missing graph fingerprint", func(s *persistedVNextGraphState) { s.GraphFingerprint = "" }},
		{"missing catalog identity", func(s *persistedVNextGraphState) { s.CatalogIdentity = "" }},
		{"no nodes", func(s *persistedVNextGraphState) { s.Nodes = nil }},
		{"bad saved_at", func(s *persistedVNextGraphState) { s.SavedAt = "not-a-time" }},
		{"duplicate node id", func(s *persistedVNextGraphState) { s.Nodes[1].NodeID = s.Nodes[0].NodeID }},
		{"duplicate host", func(s *persistedVNextGraphState) { s.Nodes[1].Target = s.Nodes[0].Target }},
		{"malformed target", func(s *persistedVNextGraphState) { s.Nodes[1].Target = "not a url" }},
		{"non-https target", func(s *persistedVNextGraphState) { s.Nodes[1].Target = "http://media.example/" }},
		{"unknown role", func(s *persistedVNextGraphState) { s.Nodes[1].Role = "WHATEVER" }},
		{"missing node id", func(s *persistedVNextGraphState) { s.Nodes[1].NodeID = "" }},
		{"missing strategy id", func(s *persistedVNextGraphState) { s.Nodes[1].StrategyID = "" }},
		{"missing template identity", func(s *persistedVNextGraphState) { s.Nodes[1].TemplateIdentity = "" }},
		{"missing host bound fingerprint", func(s *persistedVNextGraphState) { s.Nodes[1].Fingerprint = "" }},
		{"exceeds host bound", func(s *persistedVNextGraphState) {
			extra := make([]persistedGraphNode, 0, autotunevnext.MaxServiceGraphHosts+1)
			for i := 0; i <= autotunevnext.MaxServiceGraphHosts; i++ {
				extra = append(extra, persistedGraphNode{
					NodeID:   "N" + string(rune('a'+i)),
					Role:     "MEDIA",
					Required: true,
					Target:   "https://h" + string(rune('a'+i)) + ".example/",
				})
			}
			s.Nodes = extra
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := validGraphState()
			tc.mutate(&state)
			if err := validateVNextGraphState(state); err == nil {
				t.Fatalf("expected rejection for %s", tc.name)
			}
			// Save must refuse too, so an invalid intent can never be written.
			restore := engine.SetConfigDirForTest(t.TempDir())
			defer restore()
			if err := saveVNextGraphState(state); err == nil {
				t.Fatalf("save accepted invalid state for %s", tc.name)
			}
		})
	}
}

func TestGraphManagedStateCorruptFileFailsClosed(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	path, err := getVNextGraphStatePath()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, ok, err := loadVNextGraphState(); ok || err == nil {
		t.Fatalf("corrupt graph state must fail closed, got ok=%v err=%v", ok, err)
	}
}

func TestGraphManagedStateDisabledIsNotLoaded(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	state := validGraphState()
	state.Enabled = false
	if err := saveVNextGraphState(state); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok, err := loadVNextGraphState(); ok || err != nil {
		t.Fatalf("disabled intent must not load as owned, ok=%v err=%v", ok, err)
	}
}

func TestManagedRuntimeOwnershipIsExclusive(t *testing.T) {
	enabledSingle := persistedVNextState{Enabled: true}
	disabledSingle := persistedVNextState{Enabled: false}
	enabledGraph := validGraphState()
	disabledGraph := validGraphState()
	disabledGraph.Enabled = false

	cases := []struct {
		name    string
		single  *persistedVNextState
		graph   *persistedVNextGraphState
		want    ManagedRuntimeOwner
		wantErr bool
	}{
		{"neither", nil, nil, ManagedRuntimeDirect, false},
		{"single only", &enabledSingle, nil, ManagedRuntimeSingleHost, false},
		{"graph only", nil, &enabledGraph, ManagedRuntimeGraphManaged, false},
		{"single disabled graph enabled", &disabledSingle, &enabledGraph, ManagedRuntimeGraphManaged, false},
		{"graph disabled single enabled", &enabledSingle, &disabledGraph, ManagedRuntimeSingleHost, false},
		// Two enabled states is ambiguous and must fail closed, never pick one.
		{"both enabled is ambiguous", &enabledSingle, &enabledGraph, ManagedRuntimeDirect, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveManagedRuntimeOwnership(tc.single, tc.graph)
			if tc.wantErr && err == nil {
				t.Fatalf("expected ambiguity error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("owner = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestManagedRuntimeOwnershipAmbiguityUsesSentinel(t *testing.T) {
	_, err := resolveManagedRuntimeOwnership(
		&persistedVNextState{Enabled: true},
		func() *persistedVNextGraphState { s := validGraphState(); return &s }(),
	)
	if err != ErrAmbiguousManagedOwnership {
		t.Fatalf("err = %v, want ErrAmbiguousManagedOwnership", err)
	}
}
