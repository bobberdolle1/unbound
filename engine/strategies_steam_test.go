package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// splitNew mirrors the provider's section semantics: winws2 treats --new as
// the section separator and every filter/payload option persists until the
// next --new.
func splitNew(args []string) [][]string {
	sections := [][]string{{}}
	for _, arg := range args {
		if arg == "--new" {
			sections = append(sections, []string{})
			continue
		}
		sections[len(sections)-1] = append(sections[len(sections)-1], arg)
	}
	return sections
}

func sectionHasPrefix(section []string, prefix string) bool {
	for _, arg := range section {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func TestSteamSafeArgsInjectsExcludesIntoTCPSections(t *testing.T) {
	listsDir := t.TempDir()
	args := []string{
		"--filter-tcp=80,443",
		"--hostlist-domains=googlevideo.com",
		"--lua-desync=multisplit:pos=1",
		"--new",
		"--filter-tcp=80,443",
		"--lua-desync=hostfakesplit:host=ozon.ru",
	}
	got := steamSafeArgs(args, listsDir)
	sections := splitNew(got)
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(sections))
	}
	wantExclude := "--hostlist-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "steam-web-exclude.txt"))
	for i, section := range sections {
		if !sectionHasPrefix(section, wantExclude) {
			t.Errorf("TCP section %d has no steam hostlist exclude: %v", i, section)
		}
	}
}

func TestSteamSafeArgsSkipsUDPSections(t *testing.T) {
	listsDir := t.TempDir()
	args := []string{
		"--filter-udp=443",
		"--ipset=" + filepath.Join("lists", "ipset-all.txt"),
		"--payload=quic_initial",
	}
	got := steamSafeArgs(args, listsDir)
	if sectionHasPrefix(got, "--hostlist-exclude=") {
		t.Fatalf("UDP section must not receive a hostlist exclude: %v", got)
	}
	wantIPExclude := "--ipset-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "ipset-steam-exclude.txt"))
	if !sectionHasPrefix(got, wantIPExclude) {
		t.Fatalf("ipset-gated section must receive the Valve range exclude: %v", got)
	}
}

func TestSteamSafeArgsInheritsTCPModeAcrossNew(t *testing.T) {
	listsDir := t.TempDir()
	// Second section has no --filter-tcp of its own; it inherits the TCP
	// filter from the first section and therefore must be guarded too.
	args := []string{
		"--filter-tcp=80,443",
		"--new",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1",
	}
	got := steamSafeArgs(args, listsDir)
	sections := splitNew(got)
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(sections))
	}
	wantExclude := "--hostlist-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "steam-web-exclude.txt"))
	if !sectionHasPrefix(sections[1], wantExclude) {
		t.Fatalf("inherited TCP section is unguarded: %v", sections[1])
	}
}

func TestSteamSafeArgsDoesNotDuplicateExcludes(t *testing.T) {
	listsDir := t.TempDir()
	existing := "--hostlist-exclude=" + filepath.ToSlash(filepath.Join(listsDir, "my-own.txt"))
	args := []string{
		"--filter-tcp=443",
		existing,
	}
	got := steamSafeArgs(args, listsDir)
	count := 0
	for _, arg := range got {
		if strings.HasPrefix(arg, "--hostlist-exclude=") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("got %d hostlist excludes, want 1 (existing preserved): %v", count, got)
	}
	if got[1] != existing {
		t.Fatalf("existing exclude reordered or replaced: %v", got)
	}
}

func TestSteamSafeArgsIsIdempotent(t *testing.T) {
	listsDir := t.TempDir()
	args := GetProfiles(t.TempDir())[0].Args
	once := steamSafeArgs(args, listsDir)
	twice := steamSafeArgs(once, listsDir)
	if len(once) != len(twice) {
		t.Fatalf("re-applying steam safety changed arg count: %d -> %d", len(once), len(twice))
	}
	for i := range once {
		if once[i] != twice[i] {
			t.Fatalf("re-applying steam safety changed args at %d: %q vs %q", i, once[i], twice[i])
		}
	}
}

func TestCatalogGuardsEveryProfileAgainstSteamDesync(t *testing.T) {
	assets, err := ExtractAssets()
	if err != nil {
		t.Skipf("ExtractAssets unavailable: %v", err)
	}
	registrar := &fakeRegistrar{}
	profiles := RegisterWindowsProfileCatalog(registrar, assets.LuaDir)
	if len(profiles) == 0 {
		t.Fatal("profile catalog is empty")
	}
	gameProfileFound := false
	for _, profile := range profiles {
		if profile.Name == "Games & Steam (Game Filter)" {
			gameProfileFound = true
			continue // deliberately desyncs Valve ranges: no Steam guarding
		}
		sections := splitNew(profile.Args)
		for i, section := range sections {
			if sectionHasPrefix(section, "--filter-udp=") {
				continue // explicitly UDP
			}
			// Determine the effective filter mode: --filter-tcp/-udp persist
			// across --new, so walk back to the last declaration.
			mode := ""
			for j := i; j >= 0; j-- {
				if sectionHasPrefix(sections[j], "--filter-tcp=") {
					mode = "tcp"
					break
				}
				if sectionHasPrefix(sections[j], "--filter-udp=") {
					mode = "udp"
					break
				}
			}
			if mode != "tcp" {
				continue
			}
			if !sectionHasPrefix(section, "--hostlist-exclude=") {
				t.Errorf("profile %q TCP section %d is not Steam-guarded: %v", profile.Name, i, section)
			}
		}
	}
	if !gameProfileFound {
		t.Fatal("Games & Steam profile missing from catalog")
	}
}

type fakeRegistrar struct {
	mu        sync.Mutex
	registerd map[string][]string
}

func (f *fakeRegistrar) RegisterProfile(name string, args []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.registerd == nil {
		f.registerd = map[string][]string{}
	}
	f.registerd[name] = args
}

// TestWinws2DryRunAcceptsAllProfiles validates that every generated winws2
// command line parses on the real bundled engine (option names, values, Lua
// function resolution and blob references). It skips when the bundle is not
// extracted or when the engine cannot run at all in this context — the
// winws2 manifest requires elevation even for --dry-run, so non-elevated
// shells skip the check instead of failing.
func TestWinws2DryRunAcceptsAllProfiles(t *testing.T) {
	winws := os.Getenv("UNBOUND_WINWS2")
	if winws == "" {
		winws = filepath.Join("core_bin", "windows", "winws2.exe")
	}
	winwsAbs, err := filepath.Abs(winws)
	if err != nil {
		t.Skipf("resolve winws2 path: %v", err)
	}
	if _, err := os.Stat(winwsAbs); err != nil {
		t.Skipf("winws2 binary not available: %v", err)
	}

	assets, err := ExtractAssets()
	if err != nil {
		t.Skipf("ExtractAssets unavailable: %v", err)
	}

	luaInits := []string{}
	for _, lua := range []string{"zapret-lib.lua", "zapret-antidpi.lua", "init_vars.lua", "custom_funcs.lua"} {
		luaInits = append(luaInits, "--lua-init=@"+filepath.ToSlash(filepath.Join(assets.LuaDir, lua)))
	}

	registrar := &fakeRegistrar{}
	profiles := RegisterWindowsProfileCatalog(registrar, assets.LuaDir)
	for _, profile := range profiles {
		args := append([]string{"--dry-run"}, luaInits...)
		// Mirror the provider's base-filter defaulting.
		hasWf := false
		for _, arg := range profile.Args {
			if strings.HasPrefix(arg, "--wf-tcp-out=") || strings.HasPrefix(arg, "--wf-l3=") {
				hasWf = true
				break
			}
		}
		if !hasWf {
			args = append(args, "--wf-l3=ipv4,ipv6", "--wf-tcp-out=443", "--wf-udp-out=443,50000-65535")
		}
		args = append(args, profile.Args...)

		cmd := exec.Command(winwsAbs, args...)
		cmd.Dir = filepath.Dir(winwsAbs)
		out, err := cmd.CombinedOutput()
		if err == nil {
			continue
		}
		if strings.Contains(err.Error(), "elevation") || strings.Contains(string(out), "administrator") {
			t.Skipf("engine requires elevation for --dry-run: %v", err)
		}
		t.Errorf("profile %q rejected by winws2 --dry-run: %v\n%s", profile.Name, err, out)
	}
}
