package main

import (
	"context"
	"fmt"
	"testing"
	"time"
	"unbound/engine"
	"unbound/engine/providers"
)

func TestVerifyBypass(t *testing.T) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🔍 СТАРТ ГЛУБОКОЙ ПРОВЕРКИ ОБХОДА (PRO EDITION)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	assets, _ := engine.ExtractAssets()
	p := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, true, true)
	
	p.SetStatusCallback(func(s providers.Status) {
		fmt.Printf("[STATUS] %s\n", s)
	})

	for _, prof := range engine.GetProfiles(assets.LuaDir) {
		p.RegisterProfile(prof.Name, prof.Args)
	}

	targetProfile := "Unbound Ultimate (God Mode)"
	fmt.Printf("🚀 Запуск %s...\n", targetProfile)

	ctx := context.Background()
	p.Start(ctx, targetProfile)

	// Периодически выводим логи движка
	stopLogs := make(chan bool)
	go func() {
		lastIdx := 0
		for {
			select {
			case <-stopLogs:
				return
			case <-time.After(1 * time.Second):
				logs := p.GetLogs()
				if len(logs) > lastIdx {
					for _, line := range logs[lastIdx:] {
						fmt.Printf("   [ENGINE] %s\n", line)
					}
					lastIdx = len(logs)
				}
			}
		}
	}()

	time.Sleep(5 * time.Second)

	targets := []string{
		"https://youtube.com", 
		"https://discord.com",
		"https://web.telegram.org",
	}
	
	for _, target := range targets {
		fmt.Printf("📡 Проверка %s... ", target)
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		res, err := engine.ProbeConnection(probeCtx, target, nil)
		cancel()

		if err == nil && res.Success {
			fmt.Printf("✅ РАБОТАЕТ! (Пинг: %v, Серт: %s)\n", res.Latency.Truncate(time.Millisecond), res.CertIssuer)
		} else {
			fmt.Printf("❌ ОШИБКА (%v)\n", err)
		}
	}

	close(stopLogs)
	p.Stop()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
