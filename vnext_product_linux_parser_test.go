//go:build linux

package main

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
)

func TestLinuxProductionVNextCatalogParser(t *testing.T) {
	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := engine.CleanupExtractedAssets(); err != nil {
			t.Errorf("cleanup extracted parser assets: %v", err)
		}
	})
	nfqws := filepath.Join(assets.BinDir, "nfqws2")
	help, err := exec.Command(nfqws, "--help").CombinedOutput()
	if err != nil && !strings.Contains(string(help), "--dry-run") {
		t.Fatalf("nfqws2 --help: %v\n%s", err, help)
	}
	if !strings.Contains(string(help), "--dry-run") {
		t.Skip("SKIPPED_UNSUPPORTED: pinned nfqws2 has no --dry-run parser-only mode")
	}
	resolver, err := autotunevnext.NewProductAssetResolver(assets)
	if err != nil {
		t.Fatal(err)
	}
	luaInits := []string{}
	for _, name := range []string{"zapret-lib.lua", "zapret-antidpi.lua", "init_vars.lua", "custom_funcs.lua"} {
		luaInits = append(luaInits, "--lua-init=@"+filepath.ToSlash(filepath.Join(assets.LuaDir, name)))
	}
	catalog, err := productionVNextStrategyCatalog("parser.example.test")
	if err != nil {
		t.Fatal(err)
	}
	for _, strategy := range catalog {
		t.Run(strategy.ID, func(t *testing.T) {
			compiled := backendcap.Compile(strategy, backendcap.Zapret2Linux)
			if compiled.Status != backendcap.StatusCompiled {
				t.Fatalf("compile=%#v", compiled)
			}
			resolved, err := resolver.Resolve(context.Background(), backendcap.Zapret2Linux, compiled.RequiredAssets)
			if err != nil {
				t.Fatal(err)
			}
			argv, err := autotunevnext.MaterializeEngineArgv(compiled.Plan.EngineArgv, resolved)
			if err != nil {
				t.Fatal(err)
			}
			args := append([]string{"--dry-run", "--qnum=200"}, luaInits...)
			args = append(args, argv...)
			cmd := exec.Command(nfqws, args...)
			cmd.Dir = assets.BinDir
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("nfqws2 rejected materialized argv: %v\n%s\n%s", err, stdout.String(), stderr.String())
			}
		})
	}
}
