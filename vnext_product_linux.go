//go:build linux

package main

import (
	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
)

func newPlatformVNextRuntime(provider autotunevnext.RuntimeProvider, assets *engine.AssetPaths, log func(autotunevnext.PhysicalLog)) (productVNextRuntime, error) {
	runtime, err := autotunevnext.NewLinuxRuntime(autotunevnext.RuntimeOptions{Provider: provider, Assets: assets, Log: log})
	if err != nil {
		return productVNextRuntime{}, err
	}
	return productVNextRuntime{executor: runtime, preflight: runtime, backend: backendcap.Zapret2Linux}, nil
}
