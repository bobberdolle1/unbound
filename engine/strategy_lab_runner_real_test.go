package engine

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestRealWinws2DryRunVerifiesArgvAndLuaBootstrap tests the bundled winws2 executable
// with authentic argument vectors and Lua scripts without requiring network interception or elevation.
func TestRealWinws2DryRunVerifiesArgvAndLuaBootstrap(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("winws2.exe dry-run verification is specific to Windows")
	}

	assets, err := ExtractAssets()
	if err != nil {
		t.Fatalf("ExtractAssets failed: %v", err)
	}

	winwsPath := filepath.Join(assets.BinDir, "winws2.exe")
	if _, err := os.Stat(winwsPath); err != nil {
		t.Fatalf("winws2.exe not found at %s: %v", winwsPath, err)
	}

	luaLib := filepath.ToSlash(filepath.Join(assets.LuaDir, "zapret-lib.lua"))
	luaAntidpi := filepath.ToSlash(filepath.Join(assets.LuaDir, "zapret-antidpi.lua"))
	luaInit := filepath.ToSlash(filepath.Join(assets.LuaDir, "init_vars.lua"))
	luaCustom := filepath.ToSlash(filepath.Join(assets.LuaDir, "custom_funcs.lua"))

	t.Run("CandidateBootstrapAndRawFilter", func(t *testing.T) {
		args := []string{
			"--dry-run",
			"--lua-init=timer_set('exit_guard',function(name,data) os.exit(3000); end,25000,true)",
			"--lua-init=@" + luaLib,
			"--lua-init=@" + luaAntidpi,
			"--lua-init=@" + luaInit,
			"--lua-init=@" + luaCustom,
			"--wf-raw=((tcp.DstPort == 443 or tcp.SrcPort == 443) and (ip.DstAddr == 142.250.186.206 or ip.SrcAddr == 142.250.186.206))",
			"--filter-tcp=443",
			"--payload=tls_client_hello",
			"--lua-desync=hostfakesplit:midhost=midsld:repeats=2",
		}

		cmd := exec.Command(winwsPath, args...)
		cmd.Env = append(os.Environ(), "__COMPAT_LAYER=RunAsInvoker")
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		err := cmd.Run()
		combined := outBuf.String() + "\n" + errBuf.String()
		if err != nil {
			t.Fatalf("winws2 dry-run failed (err: %v): %s", err, combined)
		}
		if !strings.Contains(combined, "command line parameters verified") {
			t.Errorf("Expected 'command line parameters verified' in output: %s", combined)
		}
	})

	t.Run("AdaptiveProfileArgvVerification", func(t *testing.T) {
		_ = EnsureHostlistFiles()
		listsDir, _ := GetListsDir()
		prof := GetAdaptiveProfile(assets.LuaDir, listsDir)

		baseArgs := []string{
			"--dry-run",
			"--lua-init=@" + luaLib,
			"--lua-init=@" + luaAntidpi,
			"--lua-init=@" + luaInit,
			"--lua-init=@" + luaCustom,
		}
		fullArgs := append(baseArgs, prof.Args...)

		cmd := exec.Command(winwsPath, fullArgs...)
		cmd.Env = append(os.Environ(), "__COMPAT_LAYER=RunAsInvoker")
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		err := cmd.Run()
		combined := outBuf.String() + "\n" + errBuf.String()
		if err != nil {
			t.Fatalf("Adaptive profile dry-run failed (err: %v): %s", err, combined)
		}
		if !strings.Contains(combined, "command line parameters verified") {
			t.Errorf("Expected 'command line parameters verified' in output: %s", combined)
		}
	})

	t.Run("AutoHostlistProfileArgvVerification", func(t *testing.T) {
		_ = EnsureHostlistFiles()
		listsDir, _ := GetListsDir()
		prof := GetAutoHostlistProfile(listsDir)
		baseArgs := []string{
			"--dry-run",
			"--lua-init=@" + luaLib,
			"--lua-init=@" + luaAntidpi,
			"--lua-init=@" + luaInit,
			"--lua-init=@" + luaCustom,
		}
		fullArgs := append(baseArgs, prof.Args...)

		cmd := exec.Command(winwsPath, fullArgs...)
		cmd.Env = append(os.Environ(), "__COMPAT_LAYER=RunAsInvoker")
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		err := cmd.Run()
		combined := outBuf.String() + "\n" + errBuf.String()
		if err != nil {
			t.Fatalf("AutoHostlist profile dry-run failed (err: %v): %s", err, combined)
		}
		if !strings.Contains(combined, "command line parameters verified") {
			t.Errorf("Expected 'command line parameters verified' in output: %s", combined)
		}
	})

	t.Run("InvalidArgvFailsFast", func(t *testing.T) {
		args := []string{"--dry-run", "--wf-tcp=80,443"} // Ambiguous flag must be rejected
		cmd := exec.Command(winwsPath, args...)
		cmd.Env = append(os.Environ(), "__COMPAT_LAYER=RunAsInvoker")
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		err := cmd.Run()
		if err == nil {
			t.Fatal("Expected ambiguous --wf-tcp flag to fail, but winws2 succeeded")
		}
		combined := outBuf.String() + "\n" + errBuf.String()
		if !strings.Contains(combined, "ambiguous option -- wf-tcp") {
			t.Errorf("Expected ambiguous option error, got: %s", combined)
		}
	})
}
