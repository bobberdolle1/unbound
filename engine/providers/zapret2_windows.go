//go:build windows
// +build windows

package providers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const (
	configDirName    = "Unbound"
	customScriptName = "custom_profile.lua"
)

func getCustomScriptPath() (string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(userConfigDir, configDirName)
	return filepath.Join(configPath, customScriptName), nil
}

type Zapret2WindowsProvider struct {
	status               Status
	logs                 []string
	cmd                  *exec.Cmd
	processDone          chan struct{}
	ownedPID             int
	lifecycleOperation   uint64
	mu                   sync.Mutex
	logMu                sync.Mutex
	startStopMu          sync.Mutex
	killedManually       bool
	binPath              string
	luaDir               string
	listDir              string
	expectedEngineSHA256 string
	currentProfile       string
	debugMode            bool
	gameFilter           bool
	profileMap           map[string][]string
	profileNames         []string
	onStatusChange       func(Status)
	onLogAdd             func(string)
	logFile              *os.File
	engineReady          chan bool
}

func (e *Zapret2WindowsProvider) logLifecycleLocked(operation, event string) {
	cmdState := "nil"
	if e.cmd != nil {
		cmdState = "set"
	}
	e.addLog(fmt.Sprintf(
		"[LIFECYCLE] at=%s op=%s status=%s profile=%q pid=%d cmd=%s event=%s",
		time.Now().Format(time.RFC3339Nano),
		operation,
		e.status,
		e.currentProfile,
		e.ownedPID,
		cmdState,
		event,
	))
}

func (e *Zapret2WindowsProvider) setStoppedLocked(operation string) {
	e.cmd = nil
	e.processDone = nil
	e.ownedPID = 0
	e.currentProfile = ""
	e.status = StatusStopped
	e.logLifecycleLocked(operation, "transition=STOPPED")
	if e.onStatusChange != nil {
		e.onStatusChange(e.status)
	}
}

func NewZapret2WindowsProvider(binPath, luaDir, listDir, expectedEngineSHA256 string, debugMode bool, gameFilter bool) *Zapret2WindowsProvider {
	InitLogger()

	var logFile *os.File
	if debugMode {
		logPath := filepath.Join(os.TempDir(), "unbound_debug.log")
		logFile, _ = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	}

	return &Zapret2WindowsProvider{
		status:               StatusStopped,
		binPath:              binPath,
		luaDir:               luaDir,
		listDir:              listDir,
		expectedEngineSHA256: strings.ToLower(expectedEngineSHA256),
		debugMode:            debugMode,
		gameFilter:           gameFilter,
		profileMap:           make(map[string][]string),
		logs:                 []string{"Zapret 2 Engine (Windows) initialized."},
		logFile:              logFile,
		engineReady:          make(chan bool, 1),
	}
}

func (e *Zapret2WindowsProvider) SetStatusCallback(cb func(Status)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onStatusChange = cb
}

func (e *Zapret2WindowsProvider) SetLogCallback(cb func(string)) {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	e.onLogAdd = cb
}

func (e *Zapret2WindowsProvider) RegisterProfile(name string, args []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.profileMap[name]; !exists {
		e.profileNames = append(e.profileNames, name)
	}
	e.profileMap[name] = append([]string(nil), args...)
}

func (e *Zapret2WindowsProvider) Name() string {
	return "Zapret 2 (winws)"
}

func (e *Zapret2WindowsProvider) CheckPrivileges() (bool, error) {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		return false, err
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		return false, err
	}
	return member, nil
}

func (e *Zapret2WindowsProvider) GetProfiles() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	names := make([]string, len(e.profileNames))
	copy(names, e.profileNames)
	return append(names, "Custom Profile")
}

func (e *Zapret2WindowsProvider) getProfileArgsLocked(profileName string) ([]string, error) {
	profileArgs, exists := e.profileMap[profileName]
	if !exists && profileName != "Custom Profile" {
		return nil, fmt.Errorf("profile not found: %s", profileName)
	}

	absLuaLib, _ := filepath.Abs(filepath.Join(e.luaDir, "zapret-lib.lua"))
	absLuaAntiDpi, _ := filepath.Abs(filepath.Join(e.luaDir, "zapret-antidpi.lua"))
	absInitVars, _ := filepath.Abs(filepath.Join(e.luaDir, "init_vars.lua"))
	absCustomFuncs, _ := filepath.Abs(filepath.Join(e.luaDir, "custom_funcs.lua"))

	luaLib := filepath.ToSlash(absLuaLib)
	luaAntiDpi := filepath.ToSlash(absLuaAntiDpi)
	initVars := filepath.ToSlash(absInitVars)
	customFuncs := filepath.ToSlash(absCustomFuncs)

	// ZAPRET 2 ARCHITECTURE (2026):
	// 1. --wf-l3 is MANDATORY (ipv4,ipv6)
	// 2. --wf-tcp-out and --wf-udp-out define WinDivert capture scope (outbound only)
	// 3. Single unified profile with auto-detection
	// 4. Port lists use COMMA separation

	args := []string{}

	// Only add base WinDivert filter if profile doesn't define its own --wf-tcp-out/--wf-udp-out
	hasWfTcp := false
	for _, arg := range profileArgs {
		if strings.HasPrefix(arg, "--wf-tcp-out=") || strings.HasPrefix(arg, "--wf-l3=") {
			hasWfTcp = true
			break
		}
	}

	if !hasWfTcp {
		args = append(args, "--wf-l3=ipv4,ipv6", "--wf-tcp-out=443", "--wf-udp-out=443,50000-65535")
	}

	// Lua initialization MUST come before any profile definitions
	args = append(args, "--lua-init=@"+luaLib)
	args = append(args, "--lua-init=@"+luaAntiDpi)
	args = append(args, "--lua-init=@"+initVars)
	// custom_funcs.lua defines extra --lua-desync functions shipped with the
	// app (e.g. tls_multisplit_sni used by the "Alternative 3" profile); it
	// was previously never loaded, leaving that profile dead at runtime.
	args = append(args, "--lua-init=@"+customFuncs)

	if e.debugMode {
		args = append(args, "--debug=1")
	}
	// REMOVED: Global hostlist/ipset causes profile 0 to match everything
	// Now using --hostlist-auto in individual profiles for dynamic detection

	if profileName == "Custom Profile" {
		customScriptPath, err := getCustomScriptPath()
		if err != nil {
			return nil, fmt.Errorf("resolve custom profile: %w", err)
		}
		absCustomScript, _ := filepath.Abs(customScriptPath)
		customScriptSlash := filepath.ToSlash(absCustomScript)
		args = append(args, "--lua-init=@"+customScriptSlash)
		args = append(args, "--filter-tcp=443", "--out-range=-d10", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1")
	} else {
		args = append(args, profileArgs...)
	}

	return args, nil
}

func (e *Zapret2WindowsProvider) Start(ctx context.Context, profileName string) error {
	// Serialize full start/stop cycles. Concurrency safety: e.mu guards the
	// state fields, but it is dropped while SyncHostlists performs network
	// I/O below — without startStopMu two concurrent Starts could both pass
	// the "already running" check and spawn two winws2.exe processes.
	e.startStopMu.Lock()
	defer e.startStopMu.Unlock()

	e.mu.Lock()
	hasPriv, err := e.CheckPrivileges()
	if err != nil || !hasPriv {
		e.mu.Unlock()
		return fmt.Errorf("administrator privileges required")
	}

	if e.status == StatusRunning && e.currentProfile == profileName {
		e.mu.Unlock()
		return nil
	}

	if e.status == StatusRunning {
		e.mu.Unlock()
		e.stopLocked()
		e.mu.Lock()
	}

	args, err := e.getProfileArgsLocked(profileName)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	e.mu.Unlock()

	// Sync hostlist files from remote sources with fallback. Runs outside
	// e.mu so Stop()/GetStatus() are never blocked behind network timeouts.
	if err := SyncHostlists(); err != nil {
		e.addLog(fmt.Sprintf("Предупреждение синхронизации списков: %v", err))
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.killedManually = false

	// State may have changed while hostlist sync ran outside the lock.
	// If the same profile already started successfully, this is a no-op.
	// If a *different* profile started, surface a clear conflict error.
	if e.status == StatusRunning {
		if e.currentProfile == profileName {
			return nil
		}
		return fmt.Errorf("another profile (%s) started while this one (%s) was in progress", e.currentProfile, profileName)
	}

	e.status = StatusStarting
	winwsPath := filepath.Join(e.binPath, "winws2.exe")

	// Refuse to execute anything that does not match the pinned checksum.
	if err := verifyFileSHA256(winwsPath, e.expectedEngineSHA256); err != nil {
		e.status = StatusError
		return fmt.Errorf("refusing to execute unverified winws2: %w", err)
	}

	// Log full command for debugging
	cmdLine := winwsPath + " " + strings.Join(args, " ")
	e.addLog(fmt.Sprintf("[CMD] %s", cmdLine))
	e.addLog("[VER] " + engineVersion(winwsPath))
	WriteLog(fmt.Sprintf("Starting winws2 with profile '%s': %s", profileName, cmdLine))

	e.cmd = exec.Command(winwsPath, args...)
	e.cmd.Dir = e.binPath
	e.cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdout, _ := e.cmd.StdoutPipe()
	stderr, _ := e.cmd.StderrPipe()

	if err := e.cmd.Start(); err != nil {
		e.status = StatusError
		return err
	}
	startedCmd := e.cmd
	processDone := make(chan struct{})
	e.processDone = processDone
	e.ownedPID = startedCmd.Process.Pid
	e.addLog(fmt.Sprintf("[PID] %d", startedCmd.Process.Pid))
	WriteLog(fmt.Sprintf("winws2 started, PID %d", startedCmd.Process.Pid))

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		e.streamLogs(stdout, "STDOUT")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		e.streamLogs(stderr, "STDERR")
	}()

	e.status = StatusRunning
	e.currentProfile = profileName
	e.logLifecycleLocked("start", fmt.Sprintf("exit profile=%q", profileName))

	go func(cmd *exec.Cmd, done chan struct{}) {
		defer close(done)
		e.mu.Lock()
		e.logLifecycleLocked("wait", fmt.Sprintf("enter pid=%d", cmd.Process.Pid))
		e.mu.Unlock()
		waitErr := cmd.Wait()
		wg.Wait()
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.cmd == cmd && e.currentProfile == profileName {
			exitCode := 0
			reason := "exited normally"
			if waitErr != nil {
				exitCode = -1
				reason = waitErr.Error()
				var exitErr *exec.ExitError
				if errors.As(waitErr, &exitErr) {
					exitCode = exitErr.ExitCode()
				}
			}
			if e.killedManually {
				e.addLog(fmt.Sprintf("[EXIT] PID %d stopped by user (code %d)", cmd.Process.Pid, exitCode))
			} else if exitCode == 3221225794 || uint32(exitCode) == 0xc0000142 {
				e.addLog(fmt.Sprintf("[EXIT] PID %d terminated by anti-cheat conflict (code 0xc0000142 STATUS_DLL_INIT_FAILED: game anti-cheat blocked cygwin memory mapping)", cmd.Process.Pid))
				WriteLog(fmt.Sprintf("winws2 PID %d terminated by anti-cheat conflict: code 0xc0000142 STATUS_DLL_INIT_FAILED (BattlEye/EAC blocked cygwin1.dll)", cmd.Process.Pid))
			} else {
				e.addLog(fmt.Sprintf("[EXIT] PID %d terminated unexpectedly (code %d, %s)", cmd.Process.Pid, exitCode, reason))
				WriteLog(fmt.Sprintf("winws2 PID %d terminated unexpectedly: code %d, %s", cmd.Process.Pid, exitCode, reason))
			}
			e.setStoppedLocked("wait")
		}
		e.logLifecycleLocked("wait", fmt.Sprintf("exit pid=%d", cmd.Process.Pid))
	}(startedCmd, processDone)

	return nil
}

// engineVersion runs `winws2.exe --version` and returns its one-line
// identification (format: "github version <tag> (<sha>) lua_compat_ver <N>")
// for the diagnostics journal. Failures degrade to a placeholder — version
// reporting must never block or fail the engine start.
func engineVersion(winwsPath string) string {
	cmd := exec.Command(winwsPath, "--version")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "unknown (version probe failed)"
	}
	return strings.TrimSpace(string(out))
}

func (e *Zapret2WindowsProvider) streamLogs(reader io.Reader, source string) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		timestamp := time.Now().Format("15:04:05.000")
		logLine := fmt.Sprintf("[%s][%s] %s", timestamp, source, line)

		e.mu.Lock()
		e.addLog(logLine)

		if e.logFile != nil {
			e.logFile.WriteString(logLine + "\n")
		}

		if strings.Contains(line, "winws2 started") ||
			strings.Contains(line, "filter initialized") ||
			strings.Contains(line, "WinDivert") ||
			strings.Contains(line, "packet: id=") {
			select {
			case e.engineReady <- true:
			default:
			}
		}
		e.mu.Unlock()
	}
}

func (e *Zapret2WindowsProvider) WaitReady(timeout time.Duration) bool {
	select {
	case <-e.engineReady:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (e *Zapret2WindowsProvider) Stop() error {
	e.startStopMu.Lock()
	defer e.startStopMu.Unlock()
	return e.stopLocked()
}

// stopLocked performs the actual termination. Callers must already hold
// startStopMu (Stop) or be inside Start, which holds it for the whole
// start/stop cycle. killedManually tells the Wait goroutine whether the exit
// was user-requested (taskkill produces a non-zero exit code that would
// otherwise read as a crash).
func isWindowsPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	cmd := exec.Command("tasklist.exe", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf("%d", pid))
}

func (e *Zapret2WindowsProvider) stopLocked() error {
	e.mu.Lock()
	e.lifecycleOperation++
	operation := fmt.Sprintf("stop-%d", e.lifecycleOperation)
	e.logLifecycleLocked(operation, "enter")
	cmd := e.cmd
	done := e.processDone
	pid := e.ownedPID
	if cmd == nil || cmd.Process == nil {
		e.setStoppedLocked(operation)
		e.mu.Unlock()
		return nil
	}
	e.killedManually = true
	e.status = StatusStopping
	e.logLifecycleLocked(operation, fmt.Sprintf("transition=STOPPING pid=%d", pid))
	e.mu.Unlock()

	killCmd := exec.Command("taskkill.exe", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
	killCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if killErr := killCmd.Run(); killErr != nil && isWindowsPIDAlive(pid) {
		e.mu.Lock()
		e.status = StatusError
		e.logLifecycleLocked(operation, fmt.Sprintf("taskkill-error=%v", killErr))
		e.mu.Unlock()
		return fmt.Errorf("terminate owned process PID %d: %w", pid, killErr)
	}

	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	select {
	case <-done:
	case <-deadline.C:
		e.mu.Lock()
		e.status = StatusError
		e.logLifecycleLocked(operation, fmt.Sprintf("wait-timeout pid=%d", pid))
		e.mu.Unlock()
		return fmt.Errorf("owned process PID %d did not complete its reaper within one second", pid)
	}

	if isWindowsPIDAlive(pid) {
		e.mu.Lock()
		e.status = StatusError
		e.logLifecycleLocked(operation, fmt.Sprintf("pid-still-alive=%d", pid))
		e.mu.Unlock()
		return fmt.Errorf("owned process PID %d remained alive after reaper completion", pid)
	}

	scCmd := exec.Command("sc", "stop", "WinDivert")
	scCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = scCmd.Run()

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.status != StatusStopped || e.cmd != nil || e.currentProfile != "" || e.ownedPID != 0 {
		e.status = StatusError
		e.logLifecycleLocked(operation, "invariant-violation-after-stop")
		return fmt.Errorf("provider lifecycle invariant violated after stop")
	}
	if e.logFile != nil {
		e.logFile.Close()
		e.logFile = nil
	}
	e.logLifecycleLocked(operation, "exit")
	return nil
}

func (e *Zapret2WindowsProvider) GetStatus() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status
}

func (e *Zapret2WindowsProvider) GetLogs() []string {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	return append([]string(nil), e.logs...)
}

func (e *Zapret2WindowsProvider) addLog(msg string) {
	e.logMu.Lock()
	e.logs = append(e.logs, msg)
	if len(e.logs) > 100 {
		copy(e.logs, e.logs[len(e.logs)-100:])
		e.logs = e.logs[:100]
	}
	callback := e.onLogAdd
	e.logMu.Unlock()
	if callback != nil {
		callback(msg)
	}
}

func (e *Zapret2WindowsProvider) CurrentProfile() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentProfile
}
