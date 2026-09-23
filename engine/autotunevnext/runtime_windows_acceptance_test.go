//go:build windows

package autotunevnext

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"unbound/engine"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

func TestWindowsExactTargetCaptureParserAcceptance(t *testing.T) {
	if os.Getenv("UNBOUND_RUN_WINDOWS_NETWORK_E2E") != "1" {
		t.Skip("Set UNBOUND_RUN_WINDOWS_NETWORK_E2E=1 for physical winws2 target-filter parser acceptance")
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
	strategy, err := AcceptanceTLSStrategy("www.youtube.com", strategyir.IPFamilyV4)
	if err != nil {
		t.Fatal(err)
	}
	compiled := backendcap.Compile(strategy, backendcap.Zapret2Windows)
	if compiled.Status != backendcap.StatusCompiled {
		t.Fatalf("compile acceptance candidate: %#v", compiled)
	}
	raw, err := RenderWindowsTargetCapture(compiled.Plan.Capture, net.ParseIP("192.0.2.7"), observatory.AddressFamilyIPv4)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := backendcap.RenderWindowsCaptureArgv(compiled.Plan.Capture)
	if err != nil {
		t.Fatal(err)
	}
	args := append([]string{"--dry-run"}, trustedLuaInitArgs(assets.LuaDir)...)
	args = append(args, capture...)
	args = append(args, "--wf-raw-filter="+raw)
	args = append(args, compiled.Plan.EngineArgv...)
	cmd := exec.Command(filepath.Join(assets.BinDir, "winws2.exe"), args...)
	cmd.Dir = assets.BinDir
	cmd.Env = append(os.Environ(), "__COMPAT_LAYER=RunAsInvoker")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("winws2 rejected exact target capture: %v\n%s\n%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(strings.ToLower(stdout.String()+"\n"+stderr.String()), "command line parameters verified") {
		t.Fatalf("winws2 did not verify exact target capture: %s\n%s", stdout.String(), stderr.String())
	}
}
