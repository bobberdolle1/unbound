//go:build linux

package providers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// nfqueueNum is the netfilter queue Unbound claims. --queue-bypass is set on
// every rule so that if nfqws dies the kernel forwards packets normally
// instead of blackholing the user's entire connection.
const nfqueueNum = "200"

// fwmark marks packets nfqws has already re-injected, so our own rules do not
// queue them a second time and loop.
const fwmarkMatch = "0x40000000/0x40000000"

// packetFilter describes which traffic to divert into the NFQUEUE. Building
// these as structured values rather than pre-formatted shell strings means the
// iptables and nftables backends can render the same profile, and there is no
// string-splitting step that mangles arguments.
type packetFilter struct {
	Proto string // "tcp" or "udp"
	Ports string // multiport list, e.g. "80,443" or "50000:65535"
	// HandshakeOnly limits the rule to the first packets of a connection.
	// DPI only inspects the handshake, so queueing the whole stream would
	// push every byte of a download through userspace for no benefit.
	HandshakeOnly bool
}

type linuxProfile struct {
	Filters []packetFilter
	Args    []string
}

// builtinProfiles are the strategies shipped with the Linux build. Each entry
// pairs the netfilter rules that select traffic with the nfqws desync
// arguments applied to it.
var builtinProfiles = map[string]linuxProfile{
	"Ultimate Bypass (Multi-Strategy)": {
		Filters: []packetFilter{
			{Proto: "tcp", Ports: "80,443,5222,5223,5228", HandshakeOnly: true},
			{Proto: "udp", Ports: "443,3478,50000:65535"},
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
		Filters: []packetFilter{
			{Proto: "tcp", Ports: "443", HandshakeOnly: true},
			{Proto: "udp", Ports: "443,3478,50000:65535"},
		},
		Args: []string{
			"--filter-tcp=443", "--dpi-desync=fake,split", "--dpi-desync-split-pos=1", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=10", "--new",
			"--filter-udp=3478", "--dpi-desync=fake", "--dpi-desync-repeats=8", "--new",
			"--filter-udp=50000-65535", "--dpi-desync=fake", "--dpi-desync-repeats=8",
		},
	},
	"YouTube QUIC Aggressive": {
		Filters: []packetFilter{
			{Proto: "tcp", Ports: "80,443", HandshakeOnly: true},
			{Proto: "udp", Ports: "443"},
		},
		Args: []string{
			"--filter-tcp=80", "--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=method+2", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-tcp=443", "--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=1,midsld", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=12", "--dpi-desync-udplen-increment=2",
		},
	},
	"Telegram API Bypass": {
		Filters: []packetFilter{
			{Proto: "tcp", Ports: "443,5222,5223,5228", HandshakeOnly: true},
			{Proto: "udp", Ports: "443"},
		},
		Args: []string{
			"--filter-tcp=443", "--dpi-desync=fake,split", "--dpi-desync-split-pos=1", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-tcp=5222,5223,5228", "--dpi-desync=disorder", "--dpi-desync-split-pos=2", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=8",
		},
	},
	"Standard HTTPS/QUIC": {
		Filters: []packetFilter{
			{Proto: "tcp", Ports: "443", HandshakeOnly: true},
			{Proto: "udp", Ports: "443"},
		},
		Args: []string{
			"--filter-tcp=443", "--dpi-desync=fake,split", "--dpi-desync-split-pos=1", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=6",
		},
	},
	"HTTP + HTTPS Split": {
		Filters: []packetFilter{
			{Proto: "tcp", Ports: "80,443", HandshakeOnly: true},
		},
		Args: []string{
			"--filter-tcp=80", "--dpi-desync=split", "--dpi-desync-split-pos=method+2", "--new",
			"--filter-tcp=443", "--dpi-desync=disorder", "--dpi-desync-split-pos=2",
		},
	},
}

// profileOrder keeps the UI dropdown stable; ranging over a map would shuffle
// the list on every call.
var profileOrder = []string{
	"Ultimate Bypass (Multi-Strategy)",
	"Discord Voice Optimized",
	"YouTube QUIC Aggressive",
	"Telegram API Bypass",
	"Standard HTTPS/QUIC",
	"HTTP + HTTPS Split",
}

type ZapretLinuxProvider struct {
	mu sync.Mutex

	status         Status
	logs           []string
	cmd            *exec.Cmd
	cancel         context.CancelFunc
	binPath        string
	listsDir       string
	currentProfile string
	firewall       linuxFirewall

	// customProfiles holds strategies registered at runtime (the Custom
	// Profile builder and the auto-tuner). Previously the Linux provider had
	// no RegisterProfile at all, so those features silently did nothing.
	customProfiles map[string][]string
	customOrder    []string

	onStatus func(Status)
	onLog    func(string)
}

// NewZapretLinuxProvider builds the Linux engine provider. binPath is the
// resolved path to the nfqws executable; listsDir holds the hostlists.
func NewZapretLinuxProvider(binPath, listsDir string) BypassProvider {
	return &ZapretLinuxProvider{
		status:         StatusStopped,
		binPath:        binPath,
		listsDir:       listsDir,
		customProfiles: make(map[string][]string),
		logs:           []string{"Zapret Engine (Linux/nfqws) initialized."},
	}
}

func (e *ZapretLinuxProvider) Name() string { return "Zapret (nfqws)" }

func (e *ZapretLinuxProvider) CheckPrivileges() (bool, error) {
	return syscall.Geteuid() == 0, nil
}

func (e *ZapretLinuxProvider) SetStatusCallback(fn func(Status)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onStatus = fn
}

func (e *ZapretLinuxProvider) SetLogCallback(fn func(string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onLog = fn
}

func (e *ZapretLinuxProvider) RegisterProfile(name string, args []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.customProfiles[name]; !exists {
		e.customOrder = append(e.customOrder, name)
	}
	e.customProfiles[name] = args
}

func (e *ZapretLinuxProvider) GetProfiles() []string {
	e.mu.Lock()
	defer e.mu.Unlock()

	names := make([]string, 0, len(profileOrder)+len(e.customOrder))
	names = append(names, profileOrder...)
	for _, name := range e.customOrder {
		if _, builtin := builtinProfiles[name]; !builtin {
			names = append(names, name)
		}
	}
	return names
}

// GetStatus and GetLogs are called from the UI poll loop while the engine
// goroutine mutates the same fields; both previously read without holding the
// mutex, which is a data race the race detector flags immediately.
func (e *ZapretLinuxProvider) GetStatus() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status
}

func (e *ZapretLinuxProvider) GetLogs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.logs))
	copy(out, e.logs)
	return out
}

// resolveProfile returns the nfqws arguments and packet filters for a profile.
func (e *ZapretLinuxProvider) resolveProfile(name string) (linuxProfile, error) {
	if p, ok := builtinProfiles[name]; ok {
		return p, nil
	}
	if args, ok := e.customProfiles[name]; ok {
		// A registered profile carries nfqws arguments but no netfilter rules,
		// so queue the ports any DPI-bypass strategy cares about.
		return linuxProfile{
			Filters: []packetFilter{
				{Proto: "tcp", Ports: "80,443", HandshakeOnly: true},
				{Proto: "udp", Ports: "443"},
			},
			Args: args,
		}, nil
	}
	return linuxProfile{}, fmt.Errorf("профиль не найден: %s", name)
}

func (e *ZapretLinuxProvider) Start(ctx context.Context, profileName string) error {
	// Stop outside the lock: Stop takes the same mutex, and the previous
	// implementation unlocked mid-function while holding a deferred unlock,
	// which double-unlocks as soon as any error path is taken.
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

	e.setStatusLocked(StatusStarting)

	firewall, err := newLinuxFirewall()
	if err != nil {
		e.addLogLocked("Ошибка: " + err.Error())
		e.setStatusLocked(StatusError)
		return err
	}

	e.addLogLocked(fmt.Sprintf("[%s] Применяем правила %s...", e.Name(), firewall.Name()))
	if err := firewall.Apply(profile.Filters); err != nil {
		e.addLogLocked("Ошибка настройки firewall: " + err.Error())
		e.setStatusLocked(StatusError)
		return fmt.Errorf("не удалось применить правила %s: %w", firewall.Name(), err)
	}
	e.firewall = firewall

	args := append([]string{"--qnum=" + nfqueueNum}, profile.Args...)
	if e.listsDir != "" {
		args = append(args, "--hostlist-auto="+e.listsDir+"/autohostlist.txt")
	}

	e.addLogLocked(fmt.Sprintf("[%s] Запускаем nfqws, профиль: %s", e.Name(), profileName))

	// No --daemon: nfqws would fork and exit, cmd.Wait() would return
	// immediately, and the provider would report the engine as stopped while
	// it was in fact running - unkillable, because we had lost its PID.
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	cmd := exec.CommandContext(runCtx, e.binPath, args...)
	// Own process group so Stop() signals nfqws and its children, never the
	// whole session.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		firewall.Flush()
		e.firewall = nil
		e.setStatusLocked(StatusError)
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		firewall.Flush()
		e.firewall = nil
		e.setStatusLocked(StatusError)
		return err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		firewall.Flush()
		e.firewall = nil
		e.addLogLocked("Ошибка запуска: " + err.Error())
		e.setStatusLocked(StatusError)
		return fmt.Errorf("не удалось запустить %s: %w", e.binPath, err)
	}

	e.cmd = cmd
	e.cancel = cancel
	e.currentProfile = profileName
	e.setStatusLocked(StatusRunning)
	e.addLogLocked("Движок активен.")

	// nfqws writes diagnostics to both streams; surfacing them is the only way
	// a user can tell why a strategy is not working.
	go e.pipeToLogs(stdout, "")
	go e.pipeToLogs(stderr, "[stderr] ")

	go e.reap(cmd, profileName)

	return nil
}

// pipeToLogs streams a child process pipe into the log ring buffer.
func (e *ZapretLinuxProvider) pipeToLogs(r io.ReadCloser, prefix string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		e.addLog(prefix + scanner.Text())
	}
}

// reap waits for the engine to exit and tears down the firewall rules it
// installed, so a crash cannot leave the user's netfilter tables full of
// queue rules pointing at a process that no longer exists.
func (e *ZapretLinuxProvider) reap(cmd *exec.Cmd, profileName string) {
	err := cmd.Wait()

	e.mu.Lock()
	defer e.mu.Unlock()

	// A newer Start() already replaced this process; leave its state alone.
	if e.cmd != cmd {
		return
	}

	if e.firewall != nil {
		e.addLogLocked("Убираем правила firewall...")
		if ferr := e.firewall.Flush(); ferr != nil {
			e.addLogLocked("Не удалось убрать правила: " + ferr.Error())
		}
		e.firewall = nil
	}

	e.cmd = nil
	e.cancel = nil
	e.currentProfile = ""

	switch {
	case err == nil:
		e.addLogLocked("Движок остановлен.")
		e.setStatusLocked(StatusStopped)
	case e.status == StatusStopped:
		// Expected: Stop() signalled the process.
		e.addLogLocked("Движок остановлен.")
	default:
		e.addLogLocked(fmt.Sprintf("Движок аварийно завершился (профиль %s): %v", profileName, err))
		e.setStatusLocked(StatusError)
	}
}

func (e *ZapretLinuxProvider) Stop() error {
	e.mu.Lock()

	if e.cmd == nil || e.cmd.Process == nil {
		// Still drop any rules left behind by a previous crash.
		if e.firewall != nil {
			_ = e.firewall.Flush()
			e.firewall = nil
		}
		e.setStatusLocked(StatusStopped)
		e.mu.Unlock()
		return nil
	}

	cmd := e.cmd
	cancel := e.cancel
	pid := cmd.Process.Pid
	e.addLogLocked("Останавливаем nfqws...")
	e.setStatusLocked(StatusStopped)
	e.mu.Unlock()

	// Signal our own process group only. The previous implementation ran
	// `pkill -9 nfqws`, which killed every nfqws on the machine - including a
	// system zapret service the user runs independently - and SIGKILL denied
	// the process any chance to clean up after itself.
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			if e.GetStatus() != StatusRunning && e.processExited(cmd) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		e.addLog("nfqws не завершился за 5 секунд, отправляем SIGKILL")
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			_ = cmd.Process.Kill()
		}
	}

	if cancel != nil {
		cancel()
	}
	return nil
}

func (e *ZapretLinuxProvider) processExited(cmd *exec.Cmd) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cmd != cmd
}

func (e *ZapretLinuxProvider) currentProfileName() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentProfile
}

func (e *ZapretLinuxProvider) CurrentProfile() string {
	return e.currentProfileName()
}

func (e *ZapretLinuxProvider) setStatusLocked(status Status) {
	if e.status == status {
		return
	}
	e.status = status
	if e.onStatus != nil {
		// Emit outside the lock: the callback re-enters Wails, and a slow
		// consumer would otherwise stall the engine goroutine.
		cb := e.onStatus
		go cb(status)
	}
}

func (e *ZapretLinuxProvider) addLog(msg string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.addLogLocked(msg)
}

func (e *ZapretLinuxProvider) addLogLocked(msg string) {
	line := strings.TrimRight(msg, "\r\n")
	e.logs = append(e.logs, line)
	if len(e.logs) > maxLogLines {
		e.logs = e.logs[len(e.logs)-maxLogLines:]
	}
	if e.onLog != nil {
		cb := e.onLog
		go cb(line)
	}
}
