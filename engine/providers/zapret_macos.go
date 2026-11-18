//go:build darwin

package providers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// tpwsPort is the local SOCKS5 listener port.
const tpwsPort = "9888"

// macProfile holds the DPI arguments and optional QUIC fallback policy for a
// SOCKS-mode tpws profile. SOCKS mode is mutually exclusive with transparent
// PF redirection: clients must send a SOCKS handshake to tpws themselves.
type macProfile struct {
	// BlockQUIC makes this profile request a narrowly scoped PF UDP/443 block.
	// It is used only where TCP fallback is part of the profile's purpose.
	BlockQUIC bool
	// Args are passed to tpws. Do NOT include --port; Start adds it.
	Args []string
}

var macBuiltinProfiles = map[string]macProfile{
	"Ultimate Bypass (Multi-Strategy)": {
		BlockQUIC: true,
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
	"Discord TCP Bypass (Web / Gateway)": {
		Args: []string{
			"--bind-addr=127.0.0.1",
			"--filter-tcp=443",
			"--split-pos=1,midsld",
			"--tlsrec=1,midsld",
			"--disorder",
			"--new",
			"--filter-tcp=5222,5223,5228",
			"--split-pos=2",
			"--disorder",
		},
	},
	"YouTube QUIC Aggressive": {
		BlockQUIC: true,
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
		Args: []string{
			"--bind-addr=127.0.0.1",
			"--filter-tcp=443",
			"--split-pos=1",
			"--disorder",
		},
	},
	"HTTP + HTTPS Split": {
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
	"Discord TCP Bypass (Web / Gateway)",
	"YouTube QUIC Aggressive",
	"Telegram API Bypass",
	"Standard HTTPS/QUIC",
	"HTTP + HTTPS Split",
}

type ZapretMacOSProvider struct {
	mu sync.Mutex

	status           Status
	logs             []string
	cmd              *exec.Cmd
	cancel           context.CancelFunc
	binPath          string
	currentProfile   string
	modifiedServices []socksProxyState
	customProfiles   map[string][]string
	customOrder      []string

	statusCallback func(Status)
	logCallback    func(string)
}

// getActiveNetworkServices returns the service bound to the default-route
// interface. Configuring inactive hardware and virtual VPN services creates
// stale proxy settings without helping the active network path.
func getActiveNetworkServices() []string {
	route, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return []string{"Wi-Fi"}
	}

	var device string
	for _, line := range strings.Split(string(route), "\n") {
		field := strings.TrimSpace(line)
		if strings.HasPrefix(field, "interface:") {
			device = strings.TrimSpace(strings.TrimPrefix(field, "interface:"))
			break
		}
	}
	if device == "" {
		return []string{"Wi-Fi"}
	}

	out, err := exec.Command("networksetup", "-listnetworkserviceorder").Output()
	if err != nil {
		return []string{"Wi-Fi"}
	}

	var service string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "(") && !strings.Contains(line, "Hardware Port:") {
			parts := strings.SplitN(line, ") ", 2)
			if len(parts) == 2 {
				service = strings.TrimPrefix(strings.TrimSpace(parts[1]), "*")
			}
			continue
		}
		if service != "" && strings.Contains(line, "Device: "+device) {
			return []string{service}
		}
	}
	return []string{"Wi-Fi"}
}

// socksProxyState is the complete SOCKS setting for a service before UNBOUND
// changes it. Only a captured state may be restored; this prevents shutdown
// from clearing an unrelated user proxy configuration.
type socksProxyState struct {
	Service string
	Enabled bool
	Host    string
	Port    string
}

func snapshotSystemSocks(service string) (socksProxyState, error) {
	out, err := exec.Command("networksetup", "-getsocksfirewallproxy", service).Output()
	if err != nil {
		return socksProxyState{}, err
	}
	return parseSystemSocksOutput(service, string(out))
}

func parseSystemSocksOutput(service, output string) (socksProxyState, error) {
	state := socksProxyState{Service: service}
	sawEnabled := false
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Enabled":
			sawEnabled = true
			switch strings.TrimSpace(value) {
			case "Yes":
				state.Enabled = true
			case "No":
				state.Enabled = false
			default:
				return socksProxyState{}, fmt.Errorf("unexpected SOCKS enabled state for %s: %q", service, value)
			}
		case "Server":
			state.Host = strings.TrimSpace(value)
		case "Port":
			state.Port = strings.TrimSpace(value)
		}
	}
	if !sawEnabled {
		return socksProxyState{}, fmt.Errorf("missing SOCKS enabled state for %s", service)
	}
	if state.Enabled && (state.Host == "" || state.Port == "") {
		return socksProxyState{}, fmt.Errorf("incomplete enabled SOCKS state for %s", service)
	}
	return state, nil
}

func enableSystemSocks(port string) []socksProxyState {
	var modified []socksProxyState
	for _, service := range getActiveNetworkServices() {
		state, err := snapshotSystemSocks(service)
		if err != nil {
			continue
		}
		if err := exec.Command("networksetup", "-setsocksfirewallproxy", service, "127.0.0.1", port).Run(); err != nil {
			continue
		}
		if err := exec.Command("networksetup", "-setsocksfirewallproxystate", service, "on").Run(); err != nil {
			restoreSystemSocks([]socksProxyState{state})
			continue
		}
		modified = append(modified, state)
	}
	return modified
}

func restoreSystemSocks(states []socksProxyState) {
	for _, state := range states {
		if state.Host != "" && state.Port != "" {
			_ = exec.Command("networksetup", "-setsocksfirewallproxy", state.Service, state.Host, state.Port).Run()
		}
		enabled := "off"
		if state.Enabled {
			enabled = "on"
		}
		_ = exec.Command("networksetup", "-setsocksfirewallproxystate", state.Service, enabled).Run()
	}
}

func socksProxyServiceNames(states []socksProxyState) []string {
	names := make([]string, 0, len(states))
	for _, state := range states {
		names = append(names, state.Service)
	}
	return names
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
		return macProfile{
			Args: append([]string{"--bind-addr=127.0.0.1"}, args...),
		}, nil
	}

	nameTrimmed := strings.TrimSpace(name)
	nameLower := strings.ToLower(nameTrimmed)
	if nameLower != "" {
		// Case-insensitive exact match
		for k, p := range macBuiltinProfiles {
			if strings.ToLower(k) == nameLower {
				return p, nil
			}
		}

		// Common aliases and shortcuts
		aliases := map[string]string{
			"ultimate":    "Ultimate Bypass (Multi-Strategy)",
			"youtube":     "YouTube QUIC Aggressive",
			"discord":     "Discord TCP Bypass (Web / Gateway)",
			"https":       "Standard HTTPS/QUIC",
			"standard":    "Standard HTTPS/QUIC",
			"split":       "HTTP + HTTPS Split",
			"http":        "HTTP + HTTPS Split",
			"recommended": "Ultimate Bypass (Multi-Strategy)",
			"universal":   "Ultimate Bypass (Multi-Strategy)",
			"general":     "Ultimate Bypass (Multi-Strategy)",
			"autotune":    "Ultimate Bypass (Multi-Strategy)",
		}
		if target, ok := aliases[nameLower]; ok {
			if p, ok := macBuiltinProfiles[target]; ok {
				return p, nil
			}
		}

		// Ordered aliases by descending key length to prevent shorter substrings (like "http")
		// from greedily intercepting longer keys (like "https")
		orderedAliases := []struct{ alias, target string }{
			{"recommended", "Ultimate Bypass (Multi-Strategy)"},
			{"universal", "Ultimate Bypass (Multi-Strategy)"},
			{"ultimate", "Ultimate Bypass (Multi-Strategy)"},
			{"autotune", "Ultimate Bypass (Multi-Strategy)"},
			{"standard", "Standard HTTPS/QUIC"},
			{"telegram", "Telegram API Bypass"},
			{"youtube", "YouTube QUIC Aggressive"},
			{"discord", "Discord TCP Bypass (Web / Gateway)"},
			{"https", "Standard HTTPS/QUIC"},
			{"split", "HTTP + HTTPS Split"},
			{"http", "HTTP + HTTPS Split"},
		}
		for _, pair := range orderedAliases {
			if strings.Contains(nameLower, pair.alias) {
				if p, ok := macBuiltinProfiles[pair.target]; ok {
					return p, nil
				}
			}
		}

		// Substring match
		for k, p := range macBuiltinProfiles {
			if strings.Contains(strings.ToLower(k), nameLower) {
				return p, nil
			}
		}
	}

	// Safe fallback to default profile if requested profile is not found
	if def, ok := macBuiltinProfiles["Ultimate Bypass (Multi-Strategy)"]; ok {
		return def, nil
	}

	return macProfile{}, fmt.Errorf("профиль не найден: %s", name)
}

// ──────────────────────────────────────────────────────────────────────────────

// SocksPortInUseError identifies a listener collision before tpws is started.
// OwnerPID is zero when macOS cannot disclose the owning process.
type SocksPortInUseError struct {
	Code     string
	Port     int
	OwnerPID int
}

func (e *SocksPortInUseError) Error() string {
	if e.OwnerPID != 0 {
		return fmt.Sprintf("%s: port %d is already in use (owner PID %d)", e.Code, e.Port, e.OwnerPID)
	}
	return fmt.Sprintf("%s: port %d is already in use", e.Code, e.Port)
}

func checkTpwsPortAvailable(port string) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
	if err == nil {
		return listener.Close()
	}
	if !errors.Is(err, syscall.EADDRINUSE) {
		return err
	}
	portNumber, parseErr := strconv.Atoi(port)
	if parseErr != nil {
		return err
	}
	return &SocksPortInUseError{
		Code:     "SOCKS_PORT_IN_USE",
		Port:     portNumber,
		OwnerPID: listeningProcessPID(port),
	}
}

func listeningProcessPID(port string) int {
	out, err := exec.Command("lsof", "-nP", "-t", "-iTCP:"+port, "-sTCP:LISTEN").Output()
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]))
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

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

	// tpws args: run in SOCKS5 proxy mode on tpwsPort, then DPI desync flags.
	args := append([]string{"--socks", "--port=" + tpwsPort}, profile.Args...)

	e.addLogLocked(fmt.Sprintf("[%s] Запускаем tpws (SOCKS5), профиль: %s", e.Name(), profileName))
	e.addLogLocked(fmt.Sprintf("  Команда: tpws %s", strings.Join(args, " ")))

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	cmd := exec.CommandContext(runCtx, e.binPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		e.setStatusLocked(StatusError)
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		e.setStatusLocked(StatusError)
		return err
	}

	if err := checkTpwsPortAvailable(tpwsPort); err != nil {
		cancel()
		e.addLogLocked("Ошибка запуска tpws: " + err.Error())
		e.setStatusLocked(StatusError)
		return err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		e.addLogLocked("Ошибка запуска tpws: " + err.Error())
		e.setStatusLocked(StatusError)
		return fmt.Errorf("не удалось запустить %s: %w", e.binPath, err)
	}

	e.cmd = cmd
	e.cancel = cancel
	e.currentProfile = profileName
	e.setStatusLocked(StatusRunning)
	e.addLogLocked("tpws активен. Активируем системный SOCKS5 прокси...")

	// Enable the SOCKS proxy only for the active default-route service, after
	// capturing its exact prior state for restoration on every stop path.
	e.modifiedServices = enableSystemSocks(tpwsPort)
	if len(e.modifiedServices) > 0 {
		e.addLogLocked(fmt.Sprintf("Системный SOCKS5 прокси включен для: %s (127.0.0.1:%s)", strings.Join(socksProxyServiceNames(e.modifiedServices), ", "), tpwsPort))
	}

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

	restoreSystemSocks(e.modifiedServices)
	e.modifiedServices = nil
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

	restoreSystemSocks(e.modifiedServices)
	e.modifiedServices = nil

	if e.cmd == nil || e.cmd.Process == nil {
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

	// Signal the process group to exit.
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
