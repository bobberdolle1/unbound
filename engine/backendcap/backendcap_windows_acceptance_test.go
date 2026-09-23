package backendcap

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"unbound/engine"
	"unbound/engine/strategyir"
)

// TestWindowsCompiledRepresentativePlansDryRun parses every compiled Zapret2
// representative plan with the pinned winws2 binary. It is explicitly physical
// acceptance: no network traffic is generated, but --dry-run may still require
// WinDivert privileges. Candidates are invoked serially and extracted assets are
// removed after the final parser invocation.
func TestWindowsCompiledRepresentativePlansDryRun(t *testing.T) {
	if os.Getenv("UNBOUND_RUN_WINDOWS_NETWORK_E2E") != "1" {
		t.Skip("Set UNBOUND_RUN_WINDOWS_NETWORK_E2E=1 for physical winws2 parser acceptance")
	}
	if runtime.GOOS != "windows" {
		t.Skip("winws2 parser acceptance is specific to Windows")
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
	winws := filepath.Join(assets.BinDir, "winws2.exe")
	if _, err := os.Stat(winws); err != nil {
		t.Fatal(err)
	}

	luaInits := []string{}
	for _, name := range []string{"zapret-lib.lua", "zapret-antidpi.lua", "init_vars.lua", "custom_funcs.lua"} {
		luaInits = append(luaInits, "--lua-init=@"+filepath.ToSlash(filepath.Join(assets.LuaDir, name)))
	}
	for name, strategy := range strategyir.RepresentativeFixtures() {
		t.Run(name, func(t *testing.T) {
			result := Compile(strategy, Zapret2Windows)
			if result.Status != StatusCompiled {
				t.Fatalf("compile: %#v", result)
			}
			args := append([]string{"--dry-run"}, luaInits...)
			args = append(args, resolveParserAssets(t, result.Plan.Argv)...)
			cmd := exec.Command(winws, args...)
			cmd.Dir = filepath.Dir(winws)
			cmd.Env = append(os.Environ(), "__COMPAT_LAYER=RunAsInvoker")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("winws2 rejected compiled argv: %v\n%s\n%s", err, stdout.String(), stderr.String())
			}
			combined := stdout.String() + "\n" + stderr.String()
			if !strings.Contains(combined, "command line parameters verified") {
				t.Fatalf("winws2 did not verify compiled argv: %s", combined)
			}
		})
	}
}

func resolveParserAssets(t *testing.T, argv []string) []string {
	t.Helper()
	payloads := map[string]string{
		"fake-default-udp":       "fake_default_udp",
		"quic-google":            "quic_google",
		"stun-pat":                "stun_pat",
		"tls-clienthello-default": "fake_default_tls",
		"tls-google":             "tls_google",
	}
	logicalDir := t.TempDir()
	out := append([]string(nil), argv...)
	for i, arg := range out {
		for asset, payload := range payloads {
			arg = strings.ReplaceAll(arg, "${asset:"+asset+"}", payload)
		}
		for {
			start := strings.Index(arg, "${asset:")
			if start < 0 {
				break
			}
			end := strings.Index(arg[start:], "}")
			if end < 0 {
				t.Fatalf("malformed logical asset placeholder %q", arg)
			}
			end += start
			asset := arg[start+len("${asset:") : end]
			path := filepath.ToSlash(filepath.Join(logicalDir, asset+".txt"))
			if err := os.WriteFile(filepath.FromSlash(path), []byte("# parser-only logical scope\n"), 0600); err != nil {
				t.Fatal(err)
			}
			arg = arg[:start] + path + arg[end+1:]
		}
		out[i] = arg
	}
	return out
}
