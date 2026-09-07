package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// CandidateProcessState models the lifecycle phases of an isolated candidate process.
type CandidateProcessState int

const (
	ProcessStateStarting CandidateProcessState = iota
	ProcessStateRunning
	ProcessStateExitedEarly
	ProcessStateStopped
)

// CandidateProcess manages the lifecycle of an isolated temporary winws2 execution.
type CandidateProcess interface {
	PID() int
	Argv() []string
	Stop() error
}

// CandidateRunner executes an isolated strategy candidate with a strict raw filter.
type CandidateRunner interface {
	StartCandidate(ctx context.Context, cand StrategyCandidate, rawFilter string) (CandidateProcess, error)
}

// OSZapretCandidateProcess wraps an active exec.Cmd process with asynchronous state tracking.
type OSZapretCandidateProcess struct {
	cmd       *exec.Cmd
	pid       int
	argv      []string
	stderrBuf *bytes.Buffer
	stdoutBuf *bytes.Buffer
	waitDone  chan error
	stopped   bool
	state     CandidateProcessState
	mu        sync.Mutex
}

func (p *OSZapretCandidateProcess) PID() int {
	return p.pid
}

func (p *OSZapretCandidateProcess) Argv() []string {
	return p.argv
}

func (p *OSZapretCandidateProcess) Stop() error {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return nil
	}
	p.stopped = true
	p.state = ProcessStateStopped
	p.mu.Unlock()

	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	logger := GetLogger()
	logger.Infof("Lab", "[LAB] stopping temporary candidate process (PID=%d)", p.pid)

	if runtime.GOOS == "windows" {
		// taskkill /F /T terminates the process tree immediately
		killCmd := exec.Command("taskkill.exe", "/F", "/T", "/PID", fmt.Sprintf("%d", p.pid))
		killCmd.SysProcAttr = GetHiddenSysProcAttr()
		_ = killCmd.Run()
	} else {
		_ = p.cmd.Process.Kill()
	}

	// Bounded wait for background waiter to release handles
	select {
	case <-p.waitDone:
	case <-time.After(2 * time.Second):
		logger.Warnf("Lab", "[LAB] candidate process (PID=%d) stop timeout, forcing release", p.pid)
	}

	// Driver handle unhook delay
	time.Sleep(100 * time.Millisecond)
	logger.Infof("Lab", "[LAB] temporary candidate process (PID=%d) stopped and handles released", p.pid)
	return nil
}

// DefaultCandidateRunner is the production runner for Windows and Linux.
type DefaultCandidateRunner struct {
	assets *AssetPaths
}

// NewDefaultCandidateRunner initializes the runner with verified extracted assets.
func NewDefaultCandidateRunner() (*DefaultCandidateRunner, error) {
	assets, err := ExtractAssets()
	if err != nil {
		return nil, fmt.Errorf("failed to extract assets for candidate runner: %w", err)
	}
	return &DefaultCandidateRunner{assets: assets}, nil
}

// SanitizeCandidateArgs validates that candidate arguments cannot break isolation or inject dangerous parameters.
func SanitizeCandidateArgs(args []string) ([]string, error) {
	sanitized := make([]string, 0, len(args))
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		lower := strings.ToLower(trimmed)

		// Disallow options that could override the raw filter or interface
		if strings.HasPrefix(lower, "--wf-") {
			return nil, fmt.Errorf("forbidden option %q: candidate cannot alter WinDivert filter scope", arg)
		}
		if strings.HasPrefix(lower, "--daemon") || strings.HasPrefix(lower, "--pidfile") || strings.HasPrefix(lower, "--chdir") {
			return nil, fmt.Errorf("forbidden option %q: process control flags not permitted in candidate", arg)
		}
		if strings.HasPrefix(lower, "--lua-init") {
			return nil, fmt.Errorf("forbidden option %q: custom script initialization not permitted in candidate", arg)
		}
		if strings.HasPrefix(lower, "--intercept") || strings.HasPrefix(lower, "--writable") {
			return nil, fmt.Errorf("forbidden option %q: engine operational mode cannot be overridden", arg)
		}

		sanitized = append(sanitized, trimmed)
	}
	return sanitized, nil
}

// StartCandidate launches an isolated temporary winws2 process with the candidate args and raw filter.
func (r *DefaultCandidateRunner) StartCandidate(ctx context.Context, cand StrategyCandidate, rawFilter string) (CandidateProcess, error) {
	// SAFETY INVARIANT: Raw filter cannot be empty!
	cleanFilter := strings.TrimSpace(rawFilter)
	if cleanFilter == "" {
		return nil, errors.New("safety invariant violated: rawFilter cannot be empty (refusing broad interception)")
	}

	cleanArgs, err := SanitizeCandidateArgs(cand.Zapret2Args)
	if err != nil {
		return nil, fmt.Errorf("candidate %q has invalid arguments: %w", cand.Name, err)
	}

	logger := GetLogger()

	var binPath string
	var baseArgs []string

	if runtime.GOOS == "windows" {
		binPath = filepath.Join(r.assets.BinDir, "winws2.exe")
		if _, err := os.Stat(binPath); err != nil {
			return nil, fmt.Errorf("winws2.exe not found at %s: %w", binPath, err)
		}

		// Canonical upstream bootstrap sequence:
		// 1. Self-termination guard timer (25s) in case parent crashes
		baseArgs = append(baseArgs, "--lua-init=timer_set('exit_guard',function(name,data) os.exit(3000); end,25000,true)")

		// 2. Core library (functions, dissectors, primitives)
		luaLib := filepath.ToSlash(filepath.Join(r.assets.LuaDir, "zapret-lib.lua"))
		baseArgs = append(baseArgs, "--lua-init=@"+luaLib)

		// 3. Anti-DPI desync engine
		luaAntidpi := filepath.ToSlash(filepath.Join(r.assets.LuaDir, "zapret-antidpi.lua"))
		baseArgs = append(baseArgs, "--lua-init=@"+luaAntidpi)

		// 4. Variables initialization
		luaInit := filepath.ToSlash(filepath.Join(r.assets.LuaDir, "init_vars.lua"))
		baseArgs = append(baseArgs, "--lua-init=@"+luaInit)

		// 5. Custom helper functions
		luaCustom := filepath.ToSlash(filepath.Join(r.assets.LuaDir, "custom_funcs.lua"))
		baseArgs = append(baseArgs, "--lua-init=@"+luaCustom)

		// Intercept empty packets on Windows for accurate counting and handshake desync
		baseArgs = append(baseArgs, "--wf-tcp-empty=1")
	} else if runtime.GOOS == "linux" {
		binPath = filepath.Join(r.assets.BinDir, "nfqws2")
		if _, err := os.Stat(binPath); err != nil {
			return nil, fmt.Errorf("nfqws2 not found at %s: %w", binPath, err)
		}

		luaLib := filepath.ToSlash(filepath.Join(r.assets.LuaDir, "zapret-lib.lua"))
		luaAntidpi := filepath.ToSlash(filepath.Join(r.assets.LuaDir, "zapret-antidpi.lua"))
		baseArgs = append(baseArgs,
			"--qnum=200",
			"--lua-init=@"+luaLib,
			"--lua-init=@"+luaAntidpi,
		)
	} else {
		return nil, fmt.Errorf("isolated candidate runner not supported on %s", runtime.GOOS)
	}

	// Assemble argv: base scripts + candidate desync actions + full raw filter
	fullArgv := append([]string(nil), baseArgs...)
	fullArgv = append(fullArgv, cleanArgs...)

	if runtime.GOOS == "windows" {
		fullArgv = append(fullArgv, "--wf-raw="+cleanFilter)
	}

	logger.Infof("Lab", "[LAB] launching candidate %q with raw filter: %s", cand.Name, cleanFilter)

	stderrBuf := new(bytes.Buffer)
	stdoutBuf := new(bytes.Buffer)
	cmd := exec.Command(binPath, fullArgv...)
	cmd.SysProcAttr = GetHiddenSysProcAttr()
	cmd.Stderr = stderrBuf
	cmd.Stdout = stdoutBuf

	if err := cmd.Start(); err != nil {
		logger.Errorf("Lab", "[LAB] failed to start candidate %q: %v", cand.Name, err)
		return nil, fmt.Errorf("failed to start candidate process: %w", err)
	}

	pid := cmd.Process.Pid
	waitDone := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		waitDone <- err
		close(waitDone)
	}()

	proc := &OSZapretCandidateProcess{
		cmd:       cmd,
		pid:       pid,
		argv:      fullArgv,
		stderrBuf: stderrBuf,
		stdoutBuf: stdoutBuf,
		waitDone:  waitDone,
		state:     ProcessStateStarting,
	}

	// Active readiness verification: monitor process for 400ms.
	// If process terminates early (invalid args, driver conflict, Lua syntax error),
	// waitDone triggers immediately.
	select {
	case err := <-waitDone:
		proc.state = ProcessStateExitedEarly
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		errMsg := strings.TrimSpace(stderrBuf.String())
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdoutBuf.String())
		}
		logger.Errorf("Lab", "[LAB] candidate %q exited prematurely (code %d, err: %v): %s",
			cand.Name, exitCode, err, errMsg)
		return nil, fmt.Errorf("candidate process exited immediately (code %d): %s", exitCode, errMsg)
	case <-time.After(400 * time.Millisecond):
		proc.state = ProcessStateRunning
	}

	logger.Infof("Lab", "[LAB] candidate %q successfully started and verified running (PID=%d)", cand.Name, pid)
	return proc, nil
}
