package main

// Bounded service-graph managed intent persistence.
//
// This is Option B of the persistence-compatibility decision: a SEPARATELY
// versioned graph managed state, not a versioned extension of the single-host
// state. The single-host struct is a flat seven-field intent; a graph intent is
// structurally different (nodes, roles, requiredness, template identity, graph
// fingerprint). Extending the single-host schema would have put single-host
// compatibility at risk for no benefit, and would have made the on-disk meaning
// ambiguous.
//
// PRIVACY CONTRACT: this file is the durable projection and it deliberately has
// no field capable of holding a resolved address. There is no scope-edge field,
// no IP field, no DNS field, no filter field, no argv field, no PID or queue
// field, and no Apply token. A saved graph intent can never become packet
// authority, because it physically cannot express packet authority.
//
// The Target string is a user-authored HTTPS URL, not packet authority. It may
// name a literal host and a non-default port, and the only address-bearing data
// in a live capture is the freshly resolved edge set, which exists solely in
// memory and is never persisted. A restart always re-resolves every node, so a
// stale or literal Target cannot become a stale or literal capture.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"unbound/engine"
	"unbound/engine/autotunevnext"
)

const (
	vNextGraphStateFile = "autotune_vnext_graph_state.json"
	vNextGraphSchema    = 1
)

// persistedGraphNode is the durable logical identity of one graph node.
// It has no address-bearing field by construction.
type persistedGraphNode struct {
	NodeID   string `json:"node_id"`
	Role     string `json:"role"`
	Required bool   `json:"required"`
	Target   string `json:"target"`
	// StrategyID and TemplateIdentity are logical catalog references. The
	// host-bound fingerprint is stored only when the catalog can bind one
	// template to several hosts, so catalog drift is detectable.
	StrategyID       string `json:"strategy_id"`
	TemplateIdentity string `json:"template_identity"`
	Fingerprint      string `json:"fingerprint,omitempty"`
	// OptionalAbsentAllowed records that the validated logical graph explicitly
	// permitted this non-required node to be absent after restart. Without it an
	// absent optional node is rejected rather than quietly dropped, and a later
	// reappearance still requires fresh validation.
	OptionalAbsentAllowed bool `json:"optional_absent_allowed,omitempty"`
}

// persistedVNextGraphState is the durable bounded-graph managed intent.
// Schema is versioned separately from the single-host state.
type persistedVNextGraphState struct {
	SchemaVersion    int                  `json:"schema_version"`
	Enabled          bool                 `json:"enabled"`
	ServiceID        string               `json:"service_id"`
	Backend          string               `json:"backend"`
	GraphFingerprint string               `json:"graph_fingerprint"`
	CatalogIdentity  string               `json:"catalog_identity"`
	Nodes            []persistedGraphNode `json:"nodes"`
	SavedAt          string               `json:"saved_at"`
}

func getVNextGraphStatePath() (string, error) {
	dir, err := engine.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, vNextGraphStateFile), nil
}

// validateVNextGraphState performs every load-time check the mission requires.
// It fails closed: an unparsable node target, a duplicate node ID, a duplicate
// host, or an oversized graph is rejected rather than silently repaired.
func validateVNextGraphState(state persistedVNextGraphState) error {
	if state.SchemaVersion != vNextGraphSchema {
		return fmt.Errorf("unsupported graph managed schema %d", state.SchemaVersion)
	}
	if state.ServiceID == "" || state.Backend == "" || state.GraphFingerprint == "" || state.CatalogIdentity == "" {
		return errors.New("incomplete graph managed intent")
	}
	if len(state.Nodes) == 0 {
		return errors.New("graph managed intent has no nodes")
	}
	if len(state.Nodes) > autotunevnext.MaxServiceGraphHosts {
		return fmt.Errorf("graph managed intent exceeds host bound: %d > %d", len(state.Nodes), autotunevnext.MaxServiceGraphHosts)
	}
	if _, err := time.Parse(time.RFC3339Nano, state.SavedAt); err != nil {
		return fmt.Errorf("invalid graph managed saved_at: %w", err)
	}
	seenNode := make(map[string]bool, len(state.Nodes))
	seenHost := make(map[string]string, len(state.Nodes))
	for _, node := range state.Nodes {
		// A node that will restart as VALIDATED or ACTIVE must carry both a
		// strategy identity and its host-bound fingerprint, otherwise catalog
		// drift could not be detected. Requiring them at load time is fail-closed.
		if node.NodeID == "" || node.StrategyID == "" || node.TemplateIdentity == "" || node.Fingerprint == "" {
			return errors.New("graph managed node is missing logical identity")
		}
		if seenNode[node.NodeID] {
			return fmt.Errorf("duplicate graph node id: %s", node.NodeID)
		}
		seenNode[node.NodeID] = true
		if err := autotunevnext.ServiceNodeRole(node.Role).Validate(); err != nil {
			return fmt.Errorf("invalid graph node role %q: %w", node.Role, err)
		}
		// A malformed target is rejected outright; it must never be repaired
		// into something the user did not intend.
		host, err := normalizedVNextHost(node.Target)
		if err != nil {
			return fmt.Errorf("invalid graph node target: %w", err)
		}
		if prev, dup := seenHost[host]; dup {
			return fmt.Errorf("duplicate graph host %q on nodes %s and %s", host, prev, node.NodeID)
		}
		seenHost[host] = node.NodeID
	}
	return nil
}

func loadVNextGraphState() (persistedVNextGraphState, bool, error) {
	path, err := getVNextGraphStatePath()
	if err != nil {
		return persistedVNextGraphState{}, false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return persistedVNextGraphState{}, false, nil
	}
	if err != nil {
		return persistedVNextGraphState{}, false, err
	}
	var state persistedVNextGraphState
	if err := json.Unmarshal(data, &state); err != nil {
		return persistedVNextGraphState{}, false, fmt.Errorf("corrupt graph managed state: %w", err)
	}
	if err := validateVNextGraphState(state); err != nil {
		return persistedVNextGraphState{}, false, err
	}
	if !state.Enabled {
		return persistedVNextGraphState{}, false, nil
	}
	return state, true, nil
}

func saveVNextGraphState(state persistedVNextGraphState) error {
	if err := validateVNextGraphState(state); err != nil {
		return err
	}
	path, err := getVNextGraphStatePath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".autotune-vnext-graph-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

func clearVNextGraphState() error {
	path, err := getVNextGraphStatePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// ManagedRuntimeOwner is the single exclusive owner of the managed packet
// runtime. Exactly one owner is ever active.
type ManagedRuntimeOwner string

const (
	ManagedRuntimeDirect       ManagedRuntimeOwner = "DIRECT"
	ManagedRuntimeSingleHost   ManagedRuntimeOwner = "SINGLE_HOST_MANAGED"
	ManagedRuntimeGraphManaged ManagedRuntimeOwner = "GRAPH_MANAGED"
)

// ErrAmbiguousManagedOwnership is returned when both the single-host and the
// graph managed intent claim ownership. It is a fail-closed condition: the
// caller must return to direct state rather than guess.
var ErrAmbiguousManagedOwnership = errors.New("ambiguous managed runtime ownership")

// resolveManagedRuntimeOwnership decides exactly one owner. When both states are
// present and enabled it refuses to choose, so two managed packet runtimes can
// never start concurrently.
func resolveManagedRuntimeOwnership(singleHost *persistedVNextState, graph *persistedVNextGraphState) (ManagedRuntimeOwner, error) {
	singleHostOwns := singleHost != nil && singleHost.Enabled
	graphOwns := graph != nil && graph.Enabled
	switch {
	case singleHostOwns && graphOwns:
		return ManagedRuntimeDirect, ErrAmbiguousManagedOwnership
	case graphOwns:
		return ManagedRuntimeGraphManaged, nil
	case singleHostOwns:
		return ManagedRuntimeSingleHost, nil
	default:
		return ManagedRuntimeDirect, nil
	}
}

// normalizedVNextHost returns the validated lowercase host of a target URL. It
// deliberately reuses normalizeVNextTarget rather than adding a second target
// normalizer, so a graph node target and a single-host target can never be
// accepted by different rules.
func normalizedVNextHost(raw string) (string, error) {
	_, public, err := normalizeVNextTarget(raw)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(public)
	if err != nil {
		return "", err
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", errors.New("target has no host")
	}
	return host, nil
}
