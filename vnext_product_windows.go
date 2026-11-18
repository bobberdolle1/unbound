//go:build windows

package main

import (
	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
)

func newPlatformVNextRuntime(provider autotunevnext.RuntimeProvider, assets *engine.AssetPaths, log func(autotunevnext.PhysicalLog)) (productVNextRuntime, error) {
	runtime, err := autotunevnext.NewWindowsRuntime(autotunevnext.RuntimeOptions{Provider: provider, Assets: assets, Log: log})
	if err != nil {
		return productVNextRuntime{}, err
	}
	return productVNextRuntime{executor: runtime, preflight: runtime, backend: backendcap.Zapret2Windows}, nil
}

func newPlatformVNextGraphRuntime(provider autotunevnext.RuntimeProvider, assets *engine.AssetPaths, log func(autotunevnext.PhysicalLog)) (productVNextGraphRuntime, error) {
	opts := autotunevnext.RuntimeOptions{Provider: provider, Assets: assets, Log: log}
	preflight, err := autotunevnext.NewWindowsRuntime(opts)
	if err != nil {
		return productVNextGraphRuntime{}, err
	}
	exec, err := autotunevnext.NewWindowsGraphExecutor(opts)
	if err != nil {
		return productVNextGraphRuntime{}, err
	}
	return productVNextGraphRuntime{
		executor:  &productGraphExecutorAdapter{inner: exec, backend: backendcap.Zapret2Windows},
		preflight: preflight,
		backend:   backendcap.Zapret2Windows,
	}, nil
}
