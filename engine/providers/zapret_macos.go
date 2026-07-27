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

// divertPort is the pf divert-packet port the engine reads from.
const divertPort = "700"

type macProfile struct {
	// PfRules are loaded into our anchor; they select which traffic is
	// diverted to the engine.
	PfRules []string
	Args    []string
}

var macBuiltinProfiles = map[string]macProfile{
	"Ultimate Bypass (Multi-Strategy)": {
		PfRules: []string{
			"pass out quick proto tcp to any port {80, 443, 5222, 5223, 5228} divert-packet port " + divertPort,
			"pass out quick proto udp to any port {443, 3478, 50000:65535} divert-packet port " + divertPort,
		},
		Args: []string{
			"--filter-tcp=80", "--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=method+2", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-tcp=443", "--dpi-desync=fake,multidisorder", "--dpi-desync-split-pos=1,midsld", "--dpi-desync-fooling=badseq,md5sig", "--new",
			"--filter-tcp=5222,5223,5228", "--dpi-desync=disorder", "--dpi-desync-split-pos=2", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=10", "--dpi-desync-udplen-increment=2", "--new",
			"--filter-udp=3478,50000-65535", "--dpi-desync=fake", "--dpi-desync-repeats=8",
		},
	},
	"Discord Voice Optimized": {
		PfRules: []string{
			"pass out quick proto tcp to any port 443 divert-packet port " + divertPort,
			"pass out quick proto udp to any port {443, 3478, 50000:65535} divert-packet port " + divertPort,
		},
		Args: []string{
			"--filter-tcp=443", "--dpi-desync=fake,split", "--dpi-desync-split-pos=1", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=10", "--new",
			"--filter-udp=3478", "--dpi-desync=fake", "--dpi-desync-repeats=8", "--new",
			"--filter-udp=50000-65535", "--dpi-desync=fake", "--dpi-desync-repeats=8",
		},
	},
	"YouTube QUIC Aggressive": {
		PfRules: []string{
			"pass out quick proto tcp to any port {80, 443} divert-packet port " + divertPort,
			"pass out quick proto udp to any port 443 divert-packet port " + divertPort,
		},
		Args: []string{
			"--filter-tcp=80", "--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=method+2", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-tcp=443", "--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=1,midsld", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=12", "--dpi-desync-udplen-increment=2",
		},
	},
	"Telegram API Bypass": {
		PfRules: []string{
			"pass out quick proto tcp to any port {443, 5222, 5223, 5228} divert-packet port " + divertPort,
			"pass out quick proto udp to any port 443 divert-packet port " + divertPort,
		},
		Args: []string{
			"--filter-tcp=443", "--dpi-desync=fake,split", "--dpi-desync-split-pos=1", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-tcp=5222,5223,5228", "--dpi-desync=disorder", "--dpi-desync-split-pos=2", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=8",
		},
	},
	"Standard HTTPS/QUIC": {
		PfRules: []string{
			"pass out quick proto tcp to any port 443 divert-packet port " + divertPort,
			"pass out quick proto udp to any port 443 divert-packet port " + divertPort,
		},
		Args: []string{
			"--filter-tcp=443", "--dpi-desync=fake,split", "--dpi-desync-split-pos=1", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=6",
		},
	},
	"HTTP + HTTPS Split": {
		PfRules: []string{
			"pass out quick proto tcp to any port {80, 443} divert-packet port " + divertPort,
		},
		Args: []string{
			"--filter-tcp=80", "--dpi-desync=split", "--dpi-desync-split-pos=method+2", "--new",
			"--filter-tcp=443", "--dpi-desync=disorder", "--dpi-desync-split-pos=2",
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

// NewZapretMacOSProvider builds the macOS engine provider. binPath is the full
// path to the engine executable; it used to be the containing directory, which
// meant only a bundled binary could ever be found - a Homebrew install was
// invisible.
func NewZapretMacOSProvider(binPath string) BypassProvider {
	return &ZapretMacOSProvider{
		status:         StatusStopped,
		binPath:        binPath,
		customProfiles: make(map[string][]string),
		logs:           []string{"Zapret Engine (macOS/pf divert) initialized."},
	}
}

func (e *ZapretMacOSProvider) Name() string { return "Zapret (nfqws)" }

func (e *ZapretMacOSProvider) CheckPrivileges() (bool, error) {
	return os.Geteuid() == 0, nil
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

// RegisterProfile records a runtime-registered strategy. It used to only append
// a log line saying registration was "not fully supported", so the Custom
// Profile builder and the auto-tuner silently did nothing on macOS.
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

// GetStatus and GetLogs are polled from the UI while the engine goroutine
// mutates the same fields; both previously read without the mutex, which the
// race detector flags immediately.
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
			PfRules: []string{
				"pass out quick proto tcp to any port {80, 443} divert-packet port " + divertPort,
				"pass out quick proto udp to any port 443 divert-packet port " + divertPort,
			},
			Args: args,
		}, nil
	}
	return macProfile{}, fmt.Errorf("профиль не найден: %s", name)
}

// loadPfAnchor installs our rules into a dedicated pf anchor.
//
// The previous implementation wrote a config to the fixed path
// /tmp/unbound_pf_rules.conf and ran `pfctl -f` on it. That was wrong twice
// over: `pfctl -f` *replaces the entire active ruleset*, silently wiping
// whatever the user or their firewall app had loaded from /etc/pf.conf; and
// the file itself contained a `load anchor ... from "/tmp/unbound_pf_rules.conf"`
// line, so it tried to load itself. Writing root-loaded firewall rules to a
// predictable world-writable path is also a straightforward local escalation
// vector.
func runPfctl(stdinInput string, args ...string) ([]byte, error) {
	if os.Geteuid() == 0 {
		cmd := exec.Command("pfctl", args...)
		if stdinInput != "" {
			cmd.Stdin = strings.NewReader(stdinInput)
		}
		return cmd.CombinedOutput()
	}

	pfCmd := "pfctl " + strings.Join(args, " ")
	if stdinInput != "" {
		escapedRules := strings.ReplaceAll(stdinInput, "'", "'\"'\"'")
		pfCmd = fmt.Sprintf("echo '%s' | pfctl %s", escapedRules, strings.Join(args, " "))
	}

	escapedScript := strings.ReplaceAll(pfCmd, `\`, `\\`)
	escapedScript = strings.ReplaceAll(escapedScript, `"`, `\"`)
	script := fmt.Sprintf("do shell script \"%s\" with administrator privileges", escapedScript)
	return exec.Command("osascript", "-e", script).CombinedOutput()
}

func (e *ZapretMacOSProvider) loadPfAnchor(rules []string) error {
	if out, err := runPfctl("", "-e"); err != nil {
		if !strings.Contains(string(out), "already enabled") {
			e.addLogLocked("Предупреждение pfctl -e: " + strings.TrimSpace(string(out)))
		}
	}

	ruleset := strings.Join(rules, "\n") + "\n"
	if out, err := runPfctl(ruleset, "-a", pfAnchorName, "-f", "-"); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("pfctl: %w", err)
		}
		return fmt.Errorf("pfctl: %s", msg)
	}

	if !e.anchorIsReferenced() {
		e.addLogLocked(fmt.Sprintf(
			"ВНИМАНИЕ: якорь %q не подключён в /etc/pf.conf — правила загружены, но не применяются. "+
				"Добавьте строку: anchor \"%s\"", pfAnchorName, pfAnchorName))
	}

	return nil
}

func (e *ZapretMacOSProvider) anchorIsReferenced() bool {
	out, err := runPfctl("", "-s", "Anchors")
	if err != nil {
		return false
	}
	return strings.Contains(string(out), pfAnchorName)
}

func (e *ZapretMacOSProvider) flushPfAnchor() {
	if !e.anchorLoaded {
		return
	}
	e.anchorLoaded = false
	e.addLogLocked("Убираем правила pf...")
	_, _ = runPfctl("", "-a", pfAnchorName, "-F", "all")
}

func (e *ZapretMacOSProvider) Start(ctx context.Context, profileName string) error {
	// Stop outside the lock: Stop takes the same mutex. The previous code
	// unlocked mid-function while a deferred Unlock was pending, so any error
	// path double-unlocked and panicked.
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
			e.addLogLocked("Ошибка: бинарник nfqws не найден. Установите zapret (brew install zapret)")
			e.setStatusLocked(StatusError)
			return fmt.Errorf("бинарник nfqws не найден. Установите zapret (например, через brew install zapret)")
		}
	}

	e.setStatusLocked(StatusStarting)
	e.addLogLocked(fmt.Sprintf("[%s] Настраиваем pf...", e.Name()))

	if err := e.loadPfAnchor(profile.PfRules); err != nil {
		e.addLogLocked("Ошибка настройки pf: " + err.Error())
		e.setStatusLocked(StatusError)
		return err
	}
	e.anchorLoaded = true

	args := append([]string{"--port=" + divertPort}, profile.Args...)

	e.addLogLocked(fmt.Sprintf("[%s] Запускаем движок, профиль: %s", e.Name(), profileName))

	// No --daemon: the engine would fork and exit, cmd.Wait() would return at
	// once, and the provider would report Stopped while the real process kept
	// running with its PID lost.
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
		e.addLogLocked("Ошибка запуска: " + err.Error())
		e.setStatusLocked(StatusError)
		return fmt.Errorf("не удалось запустить %s: %w", e.binPath, err)
	}

	e.cmd = cmd
	e.cancel = cancel
	e.currentProfile = profileName
	e.setStatusLocked(StatusRunning)
	e.addLogLocked("Движок активен.")

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
		e.addLogLocked("Движок остановлен.")
		e.setStatusLocked(StatusStopped)
	case e.status == StatusStopped:
		e.addLogLocked("Движок остановлен.")
	default:
		e.addLogLocked(fmt.Sprintf("Движок аварийно завершился (профиль %s): %v", profileName, err))
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
	e.addLogLocked("Останавливаем движок...")
	e.setStatusLocked(StatusStopped)
	e.mu.Unlock()

	// Signal our own process group only. This used to run `pkill -9 nfqws`,
	// which killed every nfqws on the machine and denied it any chance to
	// release the divert socket.
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if e.processReplaced(cmd) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !e.processReplaced(cmd) {
		e.addLog("Движок не завершился за 5 секунд, отправляем SIGKILL")
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			_ = cmd.Process.Kill()
		}
	}

	if cancel != nil {
		cancel()
	}
	return nil
}

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
