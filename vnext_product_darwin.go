//go:build darwin

package main

import (
	"unbound/engine"
	"unbound/engine/autotunevnext"
)

func newPlatformVNextRuntime(autotunevnext.RuntimeProvider, *engine.AssetPaths, func(autotunevnext.PhysicalLog)) (productVNextRuntime, error) {
	return productVNextRuntime{}, errVNextMeasurementPathUnsupported
}
