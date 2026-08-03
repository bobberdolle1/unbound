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
// Using a non-privileged port (>= 1024) allows tpws to bind as standard user.
const tpwsPort = "9888"

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

const sudoersFilePath = "/etc/sudoers.d/unbound_zapret"
const sudoersContent = "ALL ALL=(ALL) NOPASSWD: /sbin/pfctl -a com.unbound.zapret*, /sbin/pfctl -f /etc/pf.conf, /sbin/pfctl -e, /sbin/pfctl -d, /sbin/pfctl -s *\n"

// EnsureSudoersConfigured checks if /etc/sudoers.d/unbound_zapret exists.
// If missing, it prompts the user ONCE via osascript with administrator privileges to create it.
func EnsureSudoersConfigured() error {
	if _, err := os.Stat(sudoersFilePath); err == nil {
		return nil
	}

	cmdStr := fmt.Sprintf("mkdir -p /etc/sudoers.d && echo '%s' > %s && chmod 0440 %s",
		strings.TrimSpace(sudoersContent), sudoersFilePath, sudoersFilePath)

	escapedScript := strings.ReplaceAll(cmdStr, `\`, `\\`)
	escapedScript = strings.ReplaceAll(escapedScript, `"`, `\"`)
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, escapedScript)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create sudoers file: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func trySudoN(stdinInput string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmdArgs := append([]string{"-n", "pfctl"}, args...)
	cmd := exec.CommandContext(ctx, "sudo", cmdArgs...)
	if stdinInput != "" {
		cmd.Stdin = strings.NewReader(stdinInput)
	}
	return cmd.CombinedOutput()
}

// runPfctlPrivileged runs pfctl, attempting non-interactive sudo via sudoers helper
// first, falling back to osascript if needed.
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

	// First attempt non-interactive sudo.
	if out, err := trySudoN(stdinInput, args...); err == nil {
		return out, nil
	}

	// If sudo -n fails, ensure sudoers helper is configured and retry.
	if sErr := EnsureSudoersConfigured(); sErr == nil {
		if out, err := trySudoN(stdinInput, args...); err == nil {
			return out, nil
		}
	}

	// Fallback to osascript if sudo -n still fails.
	pfCmd := "pfctl " + strings.Join(args, " ")
	if stdinInput != "" {
		escaped := strings.ReplaceAll(stdinInput, "'", "'\"'\"'")
		pfCmd = fmt.Sprintf("printf '%%s' '%s' | pfctl %s", escaped, strings.Join(args, " "))
	}

	escapedScript := strings.ReplaceAll(pfCmd, `\`, `\\`)
	escapedScript = strings.ReplaceAll(escapedScript, `"`, `\"`)
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, escapedScript)

	return exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
}

// loadPfAnchor enables pf (idempotent), optionally patches pf.conf anchors,
// reloads pf.conf, then loads our ruleset into the anchor.
func (e *ZapretMacOSProvider) loadPfAnchor(rules []string) error {
	ruleset := strings.Join(rules, "\n") + "\n"
	rulesTmp, err := os.CreateTemp("", "unbound_pf_rules_*.tmp")
	if err != nil {
		return fmt.Errorf("mktemp pf rules: %w", err)
	}
	rulesPath := rulesTmp.Name()
	defer os.Remove(rulesPath)

	if _, err := rulesTmp.WriteString(ruleset); err != nil {
		rulesTmp.Close()
		return fmt.Errorf("write pf rules: %w", err)
	}
	rulesTmp.Close()

	patched, pfConfTmpPath, err := preparePfConfPatch()
	if err != nil {
		e.addLogLocked("Предупреждение подготовки pf.conf: " + err.Error())
	}
	if pfConfTmpPath != "" {
		defer os.Remove(pfConfTmpPath)
	}

	if os.Geteuid() == 0 {
		_ = exec.Command("pfctl", "-e").Run()
		if patched && pfConfTmpPath != "" {
			if content, rErr := os.ReadFile(pfConfTmpPath); rErr == nil {
				if err := os.WriteFile("/etc/pf.conf", content, 0644); err == nil {
					_ = exec.Command("pfctl", "-f", "/etc/pf.conf").Run()
				}
			}
		}
		out, err := exec.Command("pfctl", "-a", pfAnchorName, "-f", rulesPath).CombinedOutput()
		if err != nil {
			return fmt.Errorf("pfctl loading anchor: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}

	// Try non-interactive sudo (with pfctl) if sudoers is configured and pf.conf does not need patching.
	if sErr := EnsureSudoersConfigured(); sErr == nil && !patched {
		_ = exec.Command("sudo", "-n", "pfctl", "-e").Run()
		if _, err := exec.Command("sudo", "-n", "pfctl", "-a", pfAnchorName, "-f", rulesPath).CombinedOutput(); err == nil {
			return nil
		}
	}

	// Fallback / First-run setup: combine all setup tasks into a single privileged shell invocation so osascript prompts ONCE.
	var scriptCmds []string
	scriptCmds = append(scriptCmds, fmt.Sprintf("mkdir -p /etc/sudoers.d && echo '%s' > %s && chmod 0440 %s",
		strings.TrimSpace(sudoersContent), sudoersFilePath, sudoersFilePath))
	scriptCmds = append(scriptCmds, "pfctl -e 2>/dev/null || true")
	if patched && pfConfTmpPath != "" {
		scriptCmds = append(scriptCmds, fmt.Sprintf("cp %s /etc/pf.conf && pfctl -f /etc/pf.conf", pfConfTmpPath))
	}
	scriptCmds = append(scriptCmds, fmt.Sprintf("pfctl -a %s -f %s", pfAnchorName, rulesPath))

	compoundCmd := strings.Join(scriptCmds, " && ")
	escapedScript := strings.ReplaceAll(compoundCmd, `\`, `\\`)
	escapedScript = strings.ReplaceAll(escapedScript, `"`, `\"`)
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, escapedScript)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("pfctl single-prompt setup: %w", err)
		}
		return fmt.Errorf("pfctl single-prompt setup: %s", msg)
	}

	return nil
}

func preparePfConfPatch() (bool, string, error) {
	const pfConf = "/etc/pf.conf"
	data, err := os.ReadFile(pfConf)
	if err != nil {
		return false, "", err
	}
	content := string(data)
	needsRdr := !strings.Contains(content, `rdr-anchor "`+pfAnchorName+`"`)
	needsAnchor := !strings.Contains(content, `anchor "`+pfAnchorName+`"`)
	if !needsRdr && !needsAnchor {
		return false, "", nil
	}
	lines := strings.Split(content, "\n")
	var out []string
	rdrInserted := !needsRdr
	anchorInserted := !needsAnchor

	for _, line := range lines {
		if !rdrInserted && strings.Contains(line, `rdr-anchor "com.apple`) {
			out = append(out, `rdr-anchor "`+pfAnchorName+`"`)
			rdrInserted = true
		}
		if !anchorInserted && strings.Contains(line, `anchor "com.apple`) && !strings.Contains(line, "load") {
			out = append(out, `anchor "`+pfAnchorName+`"`)
			anchorInserted = true
		}
		out = append(out, line)
	}

	if !rdrInserted {
		out = append(out, `rdr-anchor "`+pfAnchorName+`"`)
	}
	if !anchorInserted {
		out = append(out, `anchor "`+pfAnchorName+`"`)
	}

	tmpFile, err := os.CreateTemp("", "pf.conf.*.tmp")
	if err != nil {
		return false, "", err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.WriteString(strings.Join(out, "\n")); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return false, "", err
	}
	tmpFile.Close()
	return true, tmpPath, nil
}

// sendMacOSNotification displays a native macOS desktop notification toast.
func sendMacOSNotification(title, message string) {
	escapedTitle := strings.ReplaceAll(title, `"`, `\"`)
	escapedMsg := strings.ReplaceAll(message, `"`, `\"`)
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, escapedMsg, escapedTitle)
	_ = exec.Command("osascript", "-e", script).Run()
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
	go func() {
		_, _ = runPfctlPrivileged("", "-a", pfAnchorName, "-F", "all")
		sendMacOSNotification("UNBOUND", "Правила обхода pf успешно очищены")
	}()
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

func (e *ZapretMacOSProvider) CurrentProfile() string {
	return e.currentProfileName()
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
