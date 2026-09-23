package backendcap

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"unbound/engine"
	"unbound/engine/strategyir"
)

// TestLinuxCompiledRepresentativePlansDryRun invokes only nfqws2's documented
// dry-run parser. It never creates NFQUEUE ownership or firewall rules; those
// remain executor responsibilities outside the compiler boundary.
func TestLinuxCompiledRepresentativePlansDryRun(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("nfqws2 parser acceptance is specific to Linux")
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
	nfqws := filepath.Join(assets.BinDir, "nfqws2")
	help, err := exec.Command(nfqws, "--help").CombinedOutput()
	if err != nil && !strings.Contains(string(help), "--dry-run") {
		t.Fatalf("nfqws2 --help: %v\n%s", err, help)
	}
	if !strings.Contains(string(help), "--dry-run") {
		t.Skip("SKIPPED_UNSUPPORTED: pinned nfqws2 --help has no --dry-run parser-only mode")
	}

	luaInits := []string{}
	for _, name := range []string{"zapret-lib.lua", "zapret-antidpi.lua", "init_vars.lua", "custom_funcs.lua"} {
		luaInits = append(luaInits, "--lua-init=@"+filepath.ToSlash(filepath.Join(assets.LuaDir, name)))
	}
	for name, strategy := range strategyir.RepresentativeFixtures() {
		t.Run(name, func(t *testing.T) {
			result := Compile(strategy, Zapret2Linux)
			if result.Status != StatusCompiled {
				t.Fatalf("compile: %#v", result)
			}
			if containsPrefix(result.Plan.EngineArgv, "--wf-") {
				t.Fatalf("Linux EngineArgv leaked WinDivert flags: %v", result.Plan.EngineArgv)
			}
			args := append([]string{"--dry-run", "--qnum=200"}, luaInits...)
			args = append(args, resolveParserAssets(t, result.Plan.EngineArgv)...)
			cmd := exec.Command(nfqws, args...)
			cmd.Dir = filepath.Dir(nfqws)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("nfqws2 rejected compiled EngineArgv: %v\n%s\n%s", err, stdout.String(), stderr.String())
			}
		})
	}
}
