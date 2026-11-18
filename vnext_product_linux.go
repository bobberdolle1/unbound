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

func newPlatformVNextGraphRuntime(provider autotunevnext.RuntimeProvider, assets *engine.AssetPaths, log func(autotunevnext.PhysicalLog)) (productVNextGraphRuntime, error) {
	opts := autotunevnext.RuntimeOptions{Provider: provider, Assets: assets, Log: log}
	preflight, err := autotunevnext.NewLinuxRuntime(opts)
	if err != nil {
		return productVNextGraphRuntime{}, err
	}
	exec, err := autotunevnext.NewLinuxGraphExecutor(opts)
	if err != nil {
		return productVNextGraphRuntime{}, err
	}
	return productVNextGraphRuntime{
		executor:  &productGraphExecutorAdapter{inner: exec, backend: backendcap.Zapret2Linux},
		preflight: preflight,
		backend:   backendcap.Zapret2Linux,
	}, nil
}
