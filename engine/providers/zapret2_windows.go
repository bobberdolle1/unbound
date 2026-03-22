//go:build windows
// +build windows

package providers

import (
	"bufio"
	"context"
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
	status         Status
	logs           []string
	cmd            *exec.Cmd
	mu             sync.Mutex
	binPath        string
	luaDir         string
	listDir        string
	currentProfile string
	debugMode      bool
	gameFilter     bool
	profileMap     map[string][]string
	profileNames   []string
	onStatusChange func(Status)
	logFile        *os.File
	engineReady    chan bool
}

func NewZapret2WindowsProvider(binPath, luaDir, listDir string, debugMode bool, gameFilter bool) *Zapret2WindowsProvider {
	InitLogger()

	var logFile *os.File
	if debugMode {
		logPath := filepath.Join(os.TempDir(), "unbound_debug.log")
		logFile, _ = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	}

	return &Zapret2WindowsProvider{
		status:      StatusStopped,
		binPath:     binPath,
		luaDir:      luaDir,
		listDir:     listDir,
		debugMode:   debugMode,
		gameFilter:  gameFilter,
		profileMap:  make(map[string][]string),
		logs:        []string{"Zapret 2 Engine (Windows) initialized."},
		logFile:     logFile,
		engineReady: make(chan bool, 1),
	}
}

func (e *Zapret2WindowsProvider) SetStatusCallback(cb func(Status)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onStatusChange = cb
}

func (e *Zapret2WindowsProvider) RegisterProfile(name string, args []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.profileMap[name] = args
	e.profileNames = append(e.profileNames, name)
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

func (e *Zapret2WindowsProvider) getProfileArgsLocked(profileName string) []string {
	profileArgs, exists := e.profileMap[profileName]

	if !exists && profileName != "Custom Profile" {
		// Fallback profile if not found
		profileArgs = []string{
			"--filter-tcp=443",
			"--out-range=-d10",
			"--payload=tls_client_hello",
			"--lua-desync=multisplit:pos=1",
		}
	}

	absLuaLib, _ := filepath.Abs(filepath.Join(e.luaDir, "zapret-lib.lua"))
	absLuaAntiDpi, _ := filepath.Abs(filepath.Join(e.luaDir, "zapret-antidpi.lua"))

	luaLib := filepath.ToSlash(absLuaLib)
	luaAntiDpi := filepath.ToSlash(absLuaAntiDpi)

	// ZAPRET 2 ARCHITECTURE (2026):
	// 1. --wf-l3 is MANDATORY (ipv4,ipv6)
	// 2. --wf-tcp-in/out and --wf-udp-in/out define WinDivert capture scope
	// 3. Desync profiles created by --filter-tcp/udp in profile args
	// 4. Port lists use COMMA separation

	args := []string{
		"--wf-l3=ipv4,ipv6",
		"--wf-tcp-in=80,443,2053,2083,2087,2096,5222,5223,5228,8443,8888",
		"--wf-tcp-out=80,443,2053,2083,2087,2096,5222,5223,5228,8443,8888",
		"--wf-udp-in=443,8888,50000-65535",
		"--wf-udp-out=443,8888,50000-65535",
	}

	// Lua initialization
	args = append(args, "--lua-init=@"+luaLib)
	args = append(args, "--lua-init=@"+luaAntiDpi)

	if e.debugMode {
		args = append(args, "--debug=1")
	}

	// REMOVED: Global hostlist/ipset causes profile 0 to match everything
	// Now using --hostlist-auto in individual profiles for dynamic detection

	if profileName == "Custom Profile" {
		customScriptPath, err := getCustomScriptPath()
		if err == nil {
			absCustomScript, _ := filepath.Abs(customScriptPath)
			customScriptSlash := filepath.ToSlash(absCustomScript)
			args = append(args, "--lua-init=@"+customScriptSlash)
			args = append(args, "--filter-tcp=443", "--out-range=-d10", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1")
		}
	} else {
		args = append(args, profileArgs...)
	}

	return args
}

func (e *Zapret2WindowsProvider) Start(ctx context.Context, profileName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	hasPriv, err := e.CheckPrivileges()
	if err != nil || !hasPriv {
		return fmt.Errorf("administrator privileges required")
	}

	if e.status == StatusRunning && e.currentProfile == profileName {
		return nil
	}

	if e.status == StatusRunning {
		e.mu.Unlock()
		e.Stop()
		e.mu.Lock()
	}

	// Sync hostlist files from remote sources with fallback
	if err := SyncHostlists(); err != nil {
		return fmt.Errorf("failed to sync hostlist files: %w", err)
	}

	e.engineReady = make(chan bool, 1)
	e.status = StatusStarting
	winwsPath := filepath.Join(e.binPath, "winws2.exe")
	args := e.getProfileArgsLocked(profileName)

	e.cmd = exec.Command(winwsPath, args...)
	e.cmd.Dir = e.binPath
	e.cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdout, _ := e.cmd.StdoutPipe()
	stderr, _ := e.cmd.StderrPipe()

	if err := e.cmd.Start(); err != nil {
		e.status = StatusError
		return err
	}

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

	go func() {
		e.cmd.Wait()
		wg.Wait()
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.currentProfile == profileName {
			e.status = StatusStopped
			if e.onStatusChange != nil {
				e.onStatusChange(e.status)
			}
		}
	}()

	return nil
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
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmd != nil && e.cmd.Process != nil {
		exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", e.cmd.Process.Pid)).Run()
		time.Sleep(500 * time.Millisecond)
		exec.Command("taskkill", "/F", "/T", "/IM", "winws2.exe").Run()
		e.cmd = nil
	}

	if e.logFile != nil {
		e.logFile.Close()
		e.logFile = nil
	}

	e.status = StatusStopped
	e.currentProfile = ""
	return nil
}

func (e *Zapret2WindowsProvider) GetStatus() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status
}

func (e *Zapret2WindowsProvider) GetLogs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.logs
}

func (e *Zapret2WindowsProvider) addLog(msg string) {
	e.logs = append(e.logs, msg)
	if len(e.logs) > 100 {
		e.logs = e.logs[1:]
	}
}
