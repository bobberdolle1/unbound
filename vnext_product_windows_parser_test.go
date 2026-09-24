//go:build windows

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
)

func TestWindowsProductionVNextCatalogParser(t *testing.T) {
	if os.Getenv("UNBOUND_RUN_WINDOWS_PARSER") != "1" {
		t.Skip("Set UNBOUND_RUN_WINDOWS_PARSER=1 for pinned winws2 parser-only acceptance")
	}
	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := engine.CleanupExtractedAssets(); err != nil {
			t.Errorf("cleanup extracted parser assets: %v", err)
		}
	})
	resolver, err := autotunevnext.NewProductAssetResolver(assets)
	if err != nil {
		t.Fatal(err)
	}
	winws := filepath.Join(assets.BinDir, "winws2.exe")
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
			compiled := backendcap.Compile(strategy, backendcap.Zapret2Windows)
			if compiled.Status != backendcap.StatusCompiled {
				t.Fatalf("compile=%#v", compiled)
			}
			resolved, err := resolver.Resolve(context.Background(), backendcap.Zapret2Windows, compiled.RequiredAssets)
			if err != nil {
				t.Fatal(err)
			}
			argv, err := autotunevnext.MaterializeEngineArgv(compiled.Plan.EngineArgv, resolved)
			if err != nil {
				t.Fatal(err)
			}
			capture, err := backendcap.RenderWindowsCaptureArgv(compiled.Plan.Capture)
			if err != nil {
				t.Fatal(err)
			}
			args := append([]string{"--dry-run"}, luaInits...)
			args = append(args, capture...)
			args = append(args, argv...)
			cmd := exec.Command(winws, args...)
			cmd.Dir = assets.BinDir
			cmd.Env = append(os.Environ(), "__COMPAT_LAYER=RunAsInvoker")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("winws2 rejected materialized argv: %v\n%s\n%s", err, stdout.String(), stderr.String())
			}
			if output := stdout.String() + "\n" + stderr.String(); !strings.Contains(output, "command line parameters verified") {
				t.Fatalf("winws2 did not verify materialized argv: %s", output)
			}
		})
	}
}
