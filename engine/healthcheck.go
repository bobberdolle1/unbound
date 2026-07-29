//go:build windows

package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"unbound/engine/providers"
)

func RunHealthCheck() error {
	assets, err := ExtractAssets()
	if err != nil {
		return errors.New("failed to extract assets for health check: " + err.Error())
	}

	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, assets.EngineSHA256, false, true)
	catalog := RegisterWindowsProfileCatalog(provider, assets.LuaDir)
	if len(catalog) == 0 {
		return errors.New("Windows profile catalog is empty")
	}
	if err := provider.Start(context.Background(), catalog[0].Name); err != nil {
		return err
	}
	defer provider.Stop()

	time.Sleep(2 * time.Second)
	for _, targetURL := range []string{"https://googlevideo.com", "https://discord.com"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, probeErr := ProbeConnection(ctx, targetURL)
		cancel()
		if probeErr != nil {
			return fmt.Errorf("healthcheck failed on %s: %w", targetURL, probeErr)
		}
		if !result.Success || !result.CertValid {
			return fmt.Errorf("healthcheck failed on %s: verified TLS connection was not established", targetURL)
		}
	}
	return nil
}
