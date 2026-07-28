//go:build darwin

package providers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// pfAnchorName scopes our rules so they can be loaded and flushed without
// touching the rest of the user's firewall.
const pfAnchorName = "com.unbound.zapret"

// tpwsPort is the port that tpws listens on for transparent proxying.
const tpwsPort = "988"

// macProfile holds the pf rules and tpws arguments for a bypass strategy.
// On macOS, tpws is a transparent TCP proxy. Traffic is redirected to it
// via pf "route-to" + "rdr pass" rules (divert-packet is Linux-only).
type macProfile struct {
	// PfRules are loaded into our dedicated pf anchor.
	//
	// Two rules are always needed:
	//   1. "pass out route-to (lo0 127.0.0.1) ..." — redirects outgoing
	//      TCP packets back through loopback so the rdr rule can catch them.
	//   2. "rdr pass on lo0 ..." — rewrites the destination port to tpwsPort
	//      so tpws receives the connection.
	PfRules []string
	// Args are passed to tpws. Do NOT include --port or --bind-addr; those
	// are added by Start().
	Args []string
}

// tpwsPfRules builds the pf ruleset for a given set of TCP/UDP ports.
// Returns rules that redirect all non-root outgoing TCP on those ports to
// tpws via loopback.
func tpwsPfRules(tcpPorts string) []string {
	return []string{
		// 1. Translation (rdr) rules MUST come before filtering (pass/block) rules in pfctl!
		fmt.Sprintf("rdr pass on lo0 inet proto tcp from !127.0.0.0/8 to any port {%s} -> 127.0.0.1 port %s", tcpPorts, tpwsPort),
		// 2. Filtering (pass out / block drop out) rules come after translation rules.
		fmt.Sprintf("pass out route-to (lo0 127.0.0.1) proto tcp to port {%s} user { >0 }", tcpPorts),
		"block drop out quick proto udp to port 443",
	}
}

var macBuiltinProfiles = map[string]macProfile{
	"Ultimate Bypass (Multi-Strategy)": {
		PfRules: tpwsPfRules("80,443"),
		Args: []string{
			"--bind-addr=127.0.0.1",
			"--filter-tcp=80",
			"--split-pos=method+2",
			"--hostcase",
			"--new",
			"--filter-tcp=443",
			"--split-pos=1,midsld",
			"--disorder",
		},
	},
	"Discord Voice Optimized": {
		PfRules: tpwsPfRules("443,5222,5223,5228"),
		Args: []string{
			"--bind-addr=127.0.0.1",
			"--filter-tcp=443",
			"--split-pos=1",
			"--disorder",
			"--new",
			"--filter-tcp=5222,5223,5228",
			"--split-pos=2",
			"--disorder",
		},
	},
	"YouTube QUIC Aggressive": {
		PfRules: tpwsPfRules("80,443"),
		Args: []string{
			"--bind-addr=127.0.0.1",
			"--filter-tcp=80",
			"--split-pos=method+2",
			"--hostcase",
			"--new",
			"--filter-tcp=443",
			"--split-pos=1,midsld",
			"--tlsrec=1,midsld",
			"--disorder",
		},
	},
	"Telegram API Bypass": {
		PfRules: tpwsPfRules("443,5222,5223,5228"),
		Args: []string{
			"--bind-addr=127.0.0.1",
			"--filter-tcp=443",
			"--split-pos=1",
			"--disorder",
			"--new",
			"--filter-tcp=5222,5223,5228",
			"--split-pos=2",
			"--disorder",
		},
	},
	"Standard HTTPS/QUIC": {
		PfRules: tpwsPfRules("80,443"),
		Args: []string{
			"--bind-addr=127.0.0.1",
			"--filter-tcp=443",
			"--split-pos=1",
			"--disorder",
		},
	},
	"HTTP + HTTPS Split": {
		PfRules: tpwsPfRules("80,443"),
		Args: []string{
			"--bind-addr=127.0.0.1",
			"--filter-tcp=80",
			"--split-pos=method+2",
			"--hostcase",
			"--new",
			"--filter-tcp=443",
			"--split-pos=2",
			"--disorder",
		},
	},
}

// macProfileOrder keeps the UI dropdown stable; ranging over the map would
// reshuffle it on every call.
var macProfileOrder = []string{
	"Ultimate Bypass (Multi-Strategy)",
	"Discord Voice Optimized",
	"YouTube QUIC Aggressive",
	"Telegram API Bypass",
	"Standard HTTPS/QUIC",
	"HTTP + HTTPS Split",
}

type ZapretMacOSProvider struct {
	mu sync.Mutex

	status         Status
	logs           []string
	cmd            *exec.Cmd
	cancel         context.CancelFunc
	binPath        string
	currentProfile string
	anchorLoaded   bool

	customProfiles map[string][]string
	customOrder    []string

	statusCallback func(Status)
	logCallback    func(string)
}

// NewZapretMacOSProvider builds the macOS engine provider.
// binPath is the full path to the tpws executable; pass "" to auto-resolve.
func NewZapretMacOSProvider(binPath string) BypassProvider {
	return &ZapretMacOSProvider{
		status:         StatusStopped,
		binPath:        binPath,
		customProfiles: make(map[string][]string),
		logs:           []string{"Zapret Engine (macOS/tpws) инициализирован."},
	}
}

func (e *ZapretMacOSProvider) Name() string { return "Zapret (tpws)" }

func (e *ZapretMacOSProvider) CheckPrivileges() (bool, error) {
	if os.Geteuid() == 0 {
		return true, nil
	}
	out, err := exec.Command("id", "-Gn").Output()
	if err == nil {
		groups := strings.Fields(string(out))
		for _, g := range groups {
			if g == "admin" || g == "wheel" {
				return true, nil
			}
		}
	}
	return false, nil
}

func (e *ZapretMacOSProvider) GetProfiles() []string {
	e.mu.Lock()
	defer e.mu.Unlock()

	names := make([]string, 0, len(macProfileOrder)+len(e.customOrder))
	names = append(names, macProfileOrder...)
	for _, name := range e.customOrder {
		if _, builtin := macBuiltinProfiles[name]; !builtin {
			names = append(names, name)
		}
	}
	return names
}

// RegisterProfile records a runtime-registered strategy (Custom Profile builder).
func (e *ZapretMacOSProvider) RegisterProfile(name string, args []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.customProfiles[name]; !exists {
		e.customOrder = append(e.customOrder, name)
	}
	e.customProfiles[name] = args
}

func (e *ZapretMacOSProvider) SetStatusCallback(cb func(Status)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.statusCallback = cb
}

func (e *ZapretMacOSProvider) SetLogCallback(cb func(string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logCallback = cb
}

func (e *ZapretMacOSProvider) GetStatus() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status
}

func (e *ZapretMacOSProvider) GetLogs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.logs))
	copy(out, e.logs)
	return out
}

func (e *ZapretMacOSProvider) resolveProfile(name string) (macProfile, error) {
	if p, ok := macBuiltinProfiles[name]; ok {
		return p, nil
	}
	if args, ok := e.customProfiles[name]; ok {
		// Custom profiles use the same broad redirect rules as Ultimate Bypass.
		return macProfile{
			PfRules: tpwsPfRules("80,443"),
			Args:    append([]string{"--bind-addr=127.0.0.1"}, args...),
		}, nil
	}
	return macProfile{}, fmt.Errorf("профиль не найден: %s", name)
}

// ──────────────────────────────────────────────────────────────────────────────
// pf helpers
// ──────────────────────────────────────────────────────────────────────────────

// runPfctlPrivileged runs pfctl, prompting for admin password via osascript if
// needed. It uses a non-interactive timeout to avoid blocking shutdown.
func runPfctlPrivileged(stdinInput string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if os.Geteuid() == 0 {
		cmd := exec.CommandContext(ctx, "pfctl", args...)
		if stdinInput != "" {
			cmd.Stdin = strings.NewReader(stdinInput)
		}
		return cmd.CombinedOutput()
	}

	// Build the shell command string.
	pfCmd := "pfctl " + strings.Join(args, " ")
	if stdinInput != "" {
		escaped := strings.ReplaceAll(stdinInput, "'", "'\"'\"'")
		pfCmd = fmt.Sprintf("printf '%%s' '%s' | pfctl %s", escaped, strings.Join(args, " "))
	}

	// Escape for embedding inside an AppleScript string literal.
	escapedScript := strings.ReplaceAll(pfCmd, `\`, `\\`)
	escapedScript = strings.ReplaceAll(escapedScript, `"`, `\"`)
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, escapedScript)

	return exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
}

// ensurePfConfAnchors patches /etc/pf.conf to add our rdr-anchor and anchor
// lines if they are not already present. This is idempotent and only modifies
// the two anchor-reference lines; it does not touch any other rules.
//
// Returns true if a reload of pf.conf is needed (i.e. we patched it).
func ensurePfConfAnchors() (bool, error) {
	const pfConf = "/etc/pf.conf"

	data, err := os.ReadFile(pfConf)
	if err != nil {
		return false, fmt.Errorf("не удалось прочитать %s: %w", pfConf, err)
	}
	content := string(data)

	needsRdr := !strings.Contains(content, `rdr-anchor "`+pfAnchorName+`"`)
	needsAnchor := !strings.Contains(content, `anchor "`+pfAnchorName+`"`)

	if !needsRdr && !needsAnchor {
		return false, nil // already patched
	}

	lines := strings.Split(content, "\n")
	var out []string
	rdrInserted := !needsRdr
	anchorInserted := !needsAnchor

	for _, line := range lines {
		// Insert rdr-anchor before the com.apple rdr-anchor line.
		if !rdrInserted && strings.Contains(line, `rdr-anchor "com.apple`) {
			out = append(out, `rdr-anchor "`+pfAnchorName+`"`)
			rdrInserted = true
		}
		// Insert anchor before the com.apple filter anchor line.
		if !anchorInserted && strings.Contains(line, `anchor "com.apple`) && !strings.Contains(line, "load") {
			out = append(out, `anchor "`+pfAnchorName+`"`)
			anchorInserted = true
		}
		out = append(out, line)
	}

	// Fallback: append if we never found a good insertion point.
	if !rdrInserted {
		out = append(out, `rdr-anchor "`+pfAnchorName+`"`)
	}
	if !anchorInserted {
		out = append(out, `anchor "`+pfAnchorName+`"`)
	}

	newContent := strings.Join(out, "\n")

	// Write via a temp file + privileged move to avoid a partial write.
	tmpFile, err := os.CreateTemp("", "pf.conf.*.tmp")
	if err != nil {
		return false, fmt.Errorf("mktemp: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(newContent); err != nil {
		tmpFile.Close()
		return false, fmt.Errorf("запись в temp-файл: %w", err)
	}
	tmpFile.Close()
	if os.Geteuid() == 0 {
		if err := os.WriteFile(pfConf, []byte(newContent), 0644); err != nil {
			return false, fmt.Errorf("не удалось записать %s: %w", pfConf, err)
		}
		return true, nil
	}

	// Copy via privileged shell.
	cpCmd := fmt.Sprintf("cp %s %s", tmpPath, pfConf)
	escapedCp := strings.ReplaceAll(cpCmd, `"`, `\"`)
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, escapedCp)
	if out2, err2 := exec.Command("osascript", "-e", script).CombinedOutput(); err2 != nil {
		return false, fmt.Errorf("не удалось записать %s: %s", pfConf, strings.TrimSpace(string(out2)))
	}

	return true, nil
}

// loadPfAnchor enables pf (idempotent), optionally patches pf.conf anchors,
// reloads pf.conf, then loads our ruleset into the anchor.
func (e *ZapretMacOSProvider) loadPfAnchor(rules []string) error {
	// 1. Enable pf (macOS disables it by default).
	if out, err := runPfctlPrivileged("", "-e"); err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "already enabled") {
			e.addLogLocked("Предупреждение pfctl -e: " + msg)
		}
	}

	// 2. Ensure our anchors are referenced in /etc/pf.conf.
	patched, err := ensurePfConfAnchors()
	if err != nil {
		e.addLogLocked("ВНИМАНИЕ: " + err.Error() +
			" — правила загружены без якоря; перезапустите с правами root.")
	} else if patched {
		e.addLogLocked("Добавлены якоря в /etc/pf.conf, перезагружаем конфиг...")
		// Reload pf.conf so the new anchor references take effect.
		if out, err := runPfctlPrivileged("", "-f", "/etc/pf.conf"); err != nil {
			e.addLogLocked("Предупреждение pfctl -f pf.conf: " + strings.TrimSpace(string(out)))
		}
	}

	// 3. Load our ruleset into the anchor.
	ruleset := strings.Join(rules, "\n") + "\n"
	if out, err := runPfctlPrivileged(ruleset, "-a", pfAnchorName, "-f", "-"); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("pfctl загрузка якоря: %w", err)
		}
		return fmt.Errorf("pfctl загрузка якоря: %s", msg)
	}

	return nil
}

// flushPfAnchor removes our anchor rules and disables our anchor references
// from pf. It is non-blocking: it runs pfctl in the background so Stop()
// returns immediately.
func (e *ZapretMacOSProvider) flushPfAnchor() {
	if !e.anchorLoaded {
		return
	}
	e.anchorLoaded = false
	e.addLogLocked("Убираем правила pf...")
	_, _ = runPfctlPrivileged("", "-a", pfAnchorName, "-F", "all")
}

func (e *ZapretMacOSProvider) anchorIsReferenced() bool {
	out, err := runPfctlPrivileged("", "-s", "Anchors")
	if err != nil {
		return false
	}
	return strings.Contains(string(out), pfAnchorName)
}

// ──────────────────────────────────────────────────────────────────────────────
// Lifecycle
// ──────────────────────────────────────────────────────────────────────────────

func (e *ZapretMacOSProvider) Start(ctx context.Context, profileName string) error {
	// Stop outside the lock: Stop acquires the same mutex.
	if e.GetStatus() == StatusRunning {
		if e.currentProfileName() == profileName {
			return nil
		}
		if err := e.Stop(); err != nil {
			return err
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	profile, err := e.resolveProfile(profileName)
	if err != nil {
		e.setStatusLocked(StatusError)
		return err
	}

	if e.binPath == "" {
		if resolved, resolveErr := ResolveEngineBinary(MacOSEngineBinary, ""); resolveErr == nil {
			e.binPath = resolved
		} else {
			e.addLogLocked("Ошибка: tpws не найден. Установите zapret (brew install zapret или собери вручную)")
			e.setStatusLocked(StatusError)
			return fmt.Errorf("tpws не найден. Установите zapret (brew install zapret или собрать вручную из https://github.com/bol-van/zapret)")
		}
	}

	e.setStatusLocked(StatusStarting)
	e.addLogLocked(fmt.Sprintf("[%s] Настраиваем pf (route-to + rdr)...", e.Name()))

	if err := e.loadPfAnchor(profile.PfRules); err != nil {
		e.addLogLocked("Ошибка настройки pf: " + err.Error())
		e.setStatusLocked(StatusError)
		return err
	}
	e.anchorLoaded = true

	// tpws args: --port=PORT, bind addr, then DPI desync flags.
	args := append([]string{"--port=" + tpwsPort}, profile.Args...)

	e.addLogLocked(fmt.Sprintf("[%s] Запускаем tpws, профиль: %s", e.Name(), profileName))
	e.addLogLocked(fmt.Sprintf("  Команда: tpws %s", strings.Join(args, " ")))

	// No --daemon: the engine would fork and exit, cmd.Wait() would return
	// immediately, and the provider would report Stopped while the real
	// process kept running with its PID lost.
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	cmd := exec.CommandContext(runCtx, e.binPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		e.flushPfAnchor()
		e.setStatusLocked(StatusError)
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		e.flushPfAnchor()
		e.setStatusLocked(StatusError)
		return err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		e.flushPfAnchor()
		e.addLogLocked("Ошибка запуска tpws: " + err.Error())
		e.setStatusLocked(StatusError)
		return fmt.Errorf("не удалось запустить %s: %w", e.binPath, err)
	}

	e.cmd = cmd
	e.cancel = cancel
	e.currentProfile = profileName
	e.setStatusLocked(StatusRunning)
	e.addLogLocked("tpws активен. Трафик перенаправлен.")

	go e.pipeToLogs(stdout, "")
	go e.pipeToLogs(stderr, "[stderr] ")
	go e.reap(cmd, profileName)

	return nil
}

func (e *ZapretMacOSProvider) pipeToLogs(r io.ReadCloser, prefix string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		e.addLog(prefix + scanner.Text())
	}
}

func (e *ZapretMacOSProvider) reap(cmd *exec.Cmd, profileName string) {
	err := cmd.Wait()

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmd != cmd {
		return // a newer Start() already replaced this process
	}

	e.flushPfAnchor()
	e.cmd = nil
	e.cancel = nil
	e.currentProfile = ""

	switch {
	case err == nil:
		e.addLogLocked("tpws остановлен.")
		e.setStatusLocked(StatusStopped)
	case e.status == StatusStopped:
		e.addLogLocked("tpws остановлен.")
	default:
		e.addLogLocked(fmt.Sprintf("tpws аварийно завершился (профиль %s): %v", profileName, err))
		e.setStatusLocked(StatusError)
	}
}

func (e *ZapretMacOSProvider) Stop() error {
	e.mu.Lock()

	if e.cmd == nil || e.cmd.Process == nil {
		e.flushPfAnchor()
		e.setStatusLocked(StatusStopped)
		e.mu.Unlock()
		return nil
	}

	cmd := e.cmd
	cancel := e.cancel
	pid := cmd.Process.Pid
	e.addLogLocked("Останавливаем tpws...")
	e.setStatusLocked(StatusStopped)
	e.mu.Unlock()

	// Signal the entire process group to ensure all child processes exit.
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}

	// Wait up to 5 seconds for graceful shutdown.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if e.processReplaced(cmd) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force-kill if still running.
	if !e.processReplaced(cmd) {
		e.addLog("tpws не завершился за 5 секунд, принудительное завершение...")
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			_ = cmd.Process.Kill()
		}
	}

	if cancel != nil {
		cancel()
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────────────────────

func (e *ZapretMacOSProvider) processReplaced(cmd *exec.Cmd) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cmd != cmd
}

func (e *ZapretMacOSProvider) currentProfileName() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentProfile
}

func (e *ZapretMacOSProvider) setStatusLocked(status Status) {
	if e.status == status {
		return
	}
	e.status = status
	if e.statusCallback != nil {
		cb := e.statusCallback
		go cb(status)
	}
}

func (e *ZapretMacOSProvider) addLog(msg string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.addLogLocked(msg)
}

func (e *ZapretMacOSProvider) addLogLocked(msg string) {
	line := strings.TrimRight(msg, "\r\n")
	e.logs = append(e.logs, line)
	if len(e.logs) > maxLogLines {
		e.logs = e.logs[len(e.logs)-maxLogLines:]
	}
	if e.logCallback != nil {
		cb := e.logCallback
		go cb(line)
	}
}
