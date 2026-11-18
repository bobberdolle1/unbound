package autotunevnext

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"unbound/engine"
	"unbound/engine/providers"
	"unbound/engine/strategyir"
)

// RuntimeProvider is the narrow product boundary used only by platform
// adapters. The experiment core never depends on provider internals.
type RuntimeProvider interface {
	CheckPrivileges() (bool, error)
	Start(context.Context, string) error
	Stop() error
	GetStatus() providers.Status
	CurrentProfile() string
	Name() string
}

// RuntimeOptions wires verified, product-owned dependencies into a platform
// executor. It intentionally has no user-supplied executable or asset paths.
type RuntimeOptions struct {
	Provider RuntimeProvider
	Assets   *engine.AssetPaths
	Log      func(PhysicalLog)
}

// PhysicalLog is a sanitized lifecycle record. It deliberately excludes raw
// URLs, headers, credentials, unrelated process data, and packet contents.
type PhysicalLog struct {
	ExperimentID string
	Backend      string
	StrategyID   string
	Fingerprint  string
	Edge         string
	Phase        string
	PID          int
	Outcome      string
	Detail       string
}

func (o RuntimeOptions) emit(entry PhysicalLog) {
	if o.Log != nil {
		o.Log(entry)
	}
}

// NewProductAssetResolver materializes managed hostlist/ipset assets, checking
// user-synced engine.GetListsDir() first (when populated with regular .txt
// files) before falling back to hash-verified extracted paths.ListDir. Logical
// StrategyIR IDs never select arbitrary paths.
func NewProductAssetResolver(paths *engine.AssetPaths) (*StaticAssetResolver, error) {
	if err := engine.VerifyExtractedAssets(paths); err != nil {
		return nil, fmt.Errorf("verify product assets: %w", err)
	}

	pinnedSymbols := map[string]string{
		"fake-default-udp":        "fake_default_udp",
		"quic-google":             "quic_google",
		"stun-pat":                "stun_pat",
		"tls-clienthello-default": "fake_default_tls",
		"tls-google":              "tls_google",
	}

	seen := make(map[string]struct{}, len(pinnedSymbols))
	assets := make([]ResolvedAsset, 0, len(pinnedSymbols)+16)
	for id, symbol := range pinnedSymbols {
		seen[id] = struct{}{}
		assets = append(assets, ResolvedAsset{ID: id, Kind: AssetKindBlobSymbol, EngineValue: symbol})
	}

	collectListDir := func(dir string, requireRegular bool) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
				continue
			}
			if requireRegular && !entry.Type().IsRegular() {
				continue
			}
			id := strings.TrimSuffix(entry.Name(), ".txt")
			if id == "" {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			kind := AssetKindHostlistFile
			if id == "autohostlist" {
				kind = AssetKindAutoHostlistFile
			} else if strings.HasPrefix(id, "ipset-") || strings.HasSuffix(id, "_ips") {
				kind = AssetKindIPSetFile
			}
			seen[id] = struct{}{}
			assets = append(assets, ResolvedAsset{ID: id, Kind: kind, EngineValue: filepath.ToSlash(filepath.Join(dir, entry.Name()))})
		}
		return nil
	}

	if userListsDir, err := engine.GetListsDir(); err == nil && strings.TrimSpace(userListsDir) != "" {
		_ = collectListDir(userListsDir, true)
	}
	if err := collectListDir(paths.ListDir, false); err != nil {
		return nil, fmt.Errorf("read managed asset directory: %w", err)
	}

	sort.Slice(assets, func(i, j int) bool { return assets[i].ID < assets[j].ID })
	return NewStaticAssetResolver(assets)
}

// AcceptanceTLSStrategy is intentionally narrow and developer-only. It is not
// registered in legacy catalogs or production UI selection.
func AcceptanceTLSStrategy(hostname string, family strategyir.IPFamily) (strategyir.Strategy, error) {
	hostname = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	if hostname == "" || family == strategyir.IPFamilyAny {
		return strategyir.Strategy{}, fmt.Errorf("acceptance candidate needs explicit hostname and IP family")
	}
	position := 1
	candidate := strategyir.Strategy{
		SchemaVersion: strategyir.SchemaVersion,
		ID:            "autotune-acceptance-tls-split-v1",
		Name:          "AutoTune vNext acceptance TLS split",
		Transport:     []strategyir.Transport{strategyir.TransportTCP},
		Selector: strategyir.TrafficSelector{
			ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationTLS},
			IPFamilies:           []strategyir.IPFamily{family},
			Direction:            strategyir.DirectionOutbound,
			TCPPorts:             []strategyir.PortRange{{Start: 443, End: 443}},
			Scope:                strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeExplicit, Hosts: []string{hostname}}},
		},
		Operations: []strategyir.Operation{{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: &position}}}},
		Safety:     strategyir.SafetyPolicy{Aggressiveness: "LOW", TargetOnly: true, Experimental: true},
	}
	return strategyir.Canonicalize(candidate)
}
