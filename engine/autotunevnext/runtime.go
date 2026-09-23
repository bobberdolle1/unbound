package autotunevnext

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

// NewProductAssetResolver materializes only assets extracted and hash-verified
// from the embedded product bundle. Logical StrategyIR IDs never select paths.
func NewProductAssetResolver(paths *engine.AssetPaths) (*StaticAssetResolver, error) {
	if err := engine.VerifyExtractedAssets(paths); err != nil {
		return nil, fmt.Errorf("verify product assets: %w", err)
	}
	entries, err := os.ReadDir(paths.ListDir)
	if err != nil {
		return nil, fmt.Errorf("read managed asset directory: %w", err)
	}
	assets := make([]ResolvedAsset, 0, len(entries)+5)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".txt")
		kind := AssetKindHostlistFile
		if id == "autohostlist" {
			kind = AssetKindAutoHostlistFile
		} else if strings.HasPrefix(id, "ipset-") || strings.HasSuffix(id, "_ips") {
			kind = AssetKindIPSetFile
		}
		assets = append(assets, ResolvedAsset{ID: id, Kind: kind, EngineValue: filepath.ToSlash(filepath.Join(paths.ListDir, entry.Name()))})
	}
	// These are pinned names accepted by the bundled Zapret2 Lua runtime. They
	// are symbols, not filesystem paths supplied by a strategy.
	for id, symbol := range map[string]string{
		"fake-default-udp":        "fake_default_udp",
		"quic-google":             "quic_google",
		"stun-pat":                "stun_pat",
		"tls-clienthello-default": "fake_default_tls",
		"tls-google":              "tls_google",
	} {
		assets = append(assets, ResolvedAsset{ID: id, Kind: AssetKindBlobSymbol, EngineValue: symbol})
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

const physicalCandidateTimeout = 45 * time.Second
