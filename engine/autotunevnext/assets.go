package autotunevnext

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"unbound/engine/backendcap"
)

// StaticAssetResolver is a closed resolver for product-managed runtime values.
// StrategyIR identifies an asset only by logical ID; it cannot provide an
// EngineValue, path, URL, executable, or Lua source.
type StaticAssetResolver struct{ assets map[string]ResolvedAsset }

func NewStaticAssetResolver(assets []ResolvedAsset) (*StaticAssetResolver, error) {
	resolver := &StaticAssetResolver{assets: make(map[string]ResolvedAsset, len(assets))}
	for _, asset := range assets {
		if asset.ID == "" || asset.Kind == "" || asset.EngineValue == "" {
			return nil, fmt.Errorf("asset id, kind, and engine value are required")
		}
		if strings.ContainsAny(asset.EngineValue, "\x00\r\n") || strings.Contains(asset.EngineValue, "${asset:") {
			return nil, fmt.Errorf("invalid trusted engine value for %q", asset.ID)
		}
		if _, duplicate := resolver.assets[asset.ID]; duplicate {
			return nil, fmt.Errorf("duplicate trusted asset %q", asset.ID)
		}
		resolver.assets[asset.ID] = asset
	}
	return resolver, nil
}

func (r *StaticAssetResolver) Resolve(_ context.Context, _ backendcap.Backend, ids []string) ([]ResolvedAsset, error) {
	if r == nil {
		return nil, ErrMissingAsset
	}
	unique := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		unique[id] = struct{}{}
	}
	ordered := make([]string, 0, len(unique))
	for id := range unique {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	resolved := make([]ResolvedAsset, 0, len(ordered))
	for _, id := range ordered {
		asset, ok := r.assets[id]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingAsset, id)
		}
		resolved = append(resolved, asset)
	}
	return resolved, nil
}

// MaterializeEngineArgv substitutes only compiler-emitted ${asset:<id>}
// placeholders. It is pure: output remains argv elements, and it never invokes
// a shell, parses a command line, or expands environment variables.
func MaterializeEngineArgv(argv []string, assets []ResolvedAsset) ([]string, error) {
	resolved := make(map[string]ResolvedAsset, len(assets))
	for _, asset := range assets {
		if asset.ID == "" || asset.Kind == "" || asset.EngineValue == "" {
			return nil, fmt.Errorf("INVALID_RESOLVED_ASSET: incomplete resolved asset")
		}
		if strings.ContainsAny(asset.EngineValue, "\x00\r\n") || strings.Contains(asset.EngineValue, "${asset:") {
			return nil, fmt.Errorf("INVALID_RESOLVED_ASSET: invalid engine value for %q", asset.ID)
		}
		if prior, exists := resolved[asset.ID]; exists && (prior.Kind != asset.Kind || prior.EngineValue != asset.EngineValue) {
			return nil, fmt.Errorf("INVALID_RESOLVED_ASSET: conflicting duplicate %q", asset.ID)
		}
		resolved[asset.ID] = asset
	}
	out := make([]string, len(argv))
	for i, arg := range argv {
		expected, err := expectedAssetKinds(arg)
		if err != nil {
			return nil, err
		}
		for id, kind := range expected {
			asset, ok := resolved[id]
			if !ok {
				return nil, fmt.Errorf("MISSING_ASSET: %s", id)
			}
			if asset.Kind != kind {
				return nil, fmt.Errorf("INVALID_RESOLVED_ASSET: %s requires %s, got %s", id, kind, asset.Kind)
			}
			arg = strings.ReplaceAll(arg, "${asset:"+id+"}", asset.EngineValue)
		}
		if strings.Contains(arg, "${asset:") {
			return nil, fmt.Errorf("MALFORMED_ASSET_PLACEHOLDER")
		}
		out[i] = arg
	}
	return out, nil
}

func expectedAssetKinds(arg string) (map[string]AssetKind, error) {
	expected := map[string]AssetKind{}
	for rest := arg; ; {
		start := strings.Index(rest, "${asset:")
		if start < 0 {
			return expected, nil
		}
		end := strings.Index(rest[start:], "}")
		if end < 0 {
			return nil, fmt.Errorf("MALFORMED_ASSET_PLACEHOLDER")
		}
		end += start
		id := rest[start+len("${asset:") : end]
		if id == "" || strings.ContainsAny(id, "${}") {
			return nil, fmt.Errorf("MALFORMED_ASSET_PLACEHOLDER")
		}
		token := "${asset:" + id + "}"
		prefix := rest[:start]
		kind, ok := assetKindForContext(prefix)
		if !ok {
			return nil, fmt.Errorf("INVALID_RESOLVED_ASSET: unsupported placeholder context")
		}
		if previous, exists := expected[id]; exists && previous != kind {
			return nil, fmt.Errorf("INVALID_RESOLVED_ASSET: conflicting expected kinds for %q", id)
		}
		expected[id] = kind
		rest = rest[end+1:]
		_ = token // documents exact token boundary used by the replacement pass.
	}
}

func assetKindForContext(prefix string) (AssetKind, bool) {
	switch {
	case strings.HasPrefix(prefix, "--hostlist-auto="):
		return AssetKindAutoHostlistFile, true
	case strings.HasPrefix(prefix, "--hostlist="), strings.HasPrefix(prefix, "--hostlist-exclude="):
		return AssetKindHostlistFile, true
	case strings.HasPrefix(prefix, "--ipset="), strings.HasPrefix(prefix, "--ipset-exclude="):
		return AssetKindIPSetFile, true
	case strings.Contains(prefix, "fake:blob="), strings.Contains(prefix, ":seqovl_pattern="):
		return AssetKindBlobSymbol, true
	default:
		return "", false
	}
}
