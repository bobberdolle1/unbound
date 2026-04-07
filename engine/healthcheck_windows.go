//go:build windows
// +build windows

package engine

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"time"

	"unbound/engine/providers"
)

func RunHealthCheck() error {
	assets, err := ExtractAssets()
	if err != nil {
		return errors.New("failed to extract assets for health check: " + err.Error())
	}

	p := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, false, true)

	err = p.Start(context.Background(), "Unbound Ultimate (God Mode)")
	if err != nil {
		return err
	}
	defer p.Stop()

	time.Sleep(2 * time.Second)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get("https://www.youtube.com")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("health check failed: unexpected response")
	}

	return nil
}
