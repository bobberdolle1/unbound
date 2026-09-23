package autotunevnext

import (
	"context"
	"fmt"
	"sort"

	"unbound/engine/backendcap"
)

// StaticAssetResolver is a closed resolver for product-managed logical asset
// handles. It intentionally exposes no filesystem, URL, Lua, or executable
// escape hatch to StrategyIR.
type StaticAssetResolver struct{ assets map[string]ResolvedAsset }

func NewStaticAssetResolver(assets []ResolvedAsset) (*StaticAssetResolver, error) {
	resolver := &StaticAssetResolver{assets: make(map[string]ResolvedAsset, len(assets))}
	for _, asset := range assets {
		if asset.ID == "" || asset.Kind == "" {
			return nil, fmt.Errorf("asset id and kind are required")
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
