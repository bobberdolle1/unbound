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

	urls := []string{"https://googlevideo.com", "https://discord.com"}
	for _, u := range urls {
		resp, err := client.Get(u)
		if err != nil {
			return errors.New("healthcheck failed on " + u + ": " + err.Error())
		}
		resp.Body.Close()
	}

	return nil
}
