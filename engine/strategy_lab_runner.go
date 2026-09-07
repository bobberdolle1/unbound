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

// OSZapretCandidateProcess wraps an active exec.Cmd process.
type OSZapretCandidateProcess struct {
	cmd       *exec.Cmd
	pid       int
	argv      []string
	stderrBuf *bytes.Buffer
	stopped   bool
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
	defer p.mu.Unlock()

	if p.stopped {
		return nil
	}
	p.stopped = true

	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	logger := GetLogger()
	logger.Infof("Lab", "[LAB] stopping temporary candidate process (PID=%d)", p.pid)

	if runtime.GOOS == "windows" {
		// On Windows, taskkill /F /T kills child processes and releases WinDivert handles immediately
		killCmd := exec.Command("taskkill.exe", "/F", "/T", "/PID", fmt.Sprintf("%d", p.pid))
		killCmd.SysProcAttr = GetHiddenSysProcAttr()
		_ = killCmd.Run()
	} else {
		_ = p.cmd.Process.Kill()
	}

	_ = p.cmd.Wait()
	time.Sleep(200 * time.Millisecond) // WinDivert driver handle release
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

// StartCandidate launches an isolated temporary winws2 process with the candidate args and raw filter.
func (r *DefaultCandidateRunner) StartCandidate(ctx context.Context, cand StrategyCandidate, rawFilter string) (CandidateProcess, error) {
	// SAFETY INVARIANT: Raw filter cannot be empty!
	cleanFilter := strings.TrimSpace(rawFilter)
	if cleanFilter == "" {
		return nil, errors.New("safety invariant violated: rawFilter cannot be empty (refusing broad interception)")
	}

	logger := GetLogger()

	var binPath string
	var baseArgs []string

	if runtime.GOOS == "windows" {
		binPath = filepath.Join(r.assets.BinDir, "winws2.exe")
		if _, err := os.Stat(binPath); err != nil {
			return nil, fmt.Errorf("winws2.exe not found at %s: %w", binPath, err)
		}

		luaInit := filepath.Join(r.assets.LuaDir, "init_vars.lua")
		luaAntidpi := filepath.Join(r.assets.LuaDir, "zapret-antidpi.lua")
		luaCustom := filepath.Join(r.assets.LuaDir, "custom_funcs.lua")

		baseArgs = []string{
			"--lua-init=@" + luaInit,
			"--lua-init=@" + luaAntidpi,
			"--lua-init=@" + luaCustom,
		}
	} else if runtime.GOOS == "linux" {
		binPath = filepath.Join(r.assets.BinDir, "nfqws2")
		if _, err := os.Stat(binPath); err != nil {
			return nil, fmt.Errorf("nfqws2 not found at %s: %w", binPath, err)
		}
	} else {
		return nil, fmt.Errorf("isolated candidate runner not supported on %s", runtime.GOOS)
	}

	// Assemble argv: base lua scripts + candidate args + raw filter
	fullArgv := append([]string(nil), baseArgs...)
	fullArgv = append(fullArgv, cand.Zapret2Args...)

	// Append raw filter
	fullArgv = append(fullArgv, "--wf-raw-filter="+cleanFilter)

	logger.Infof("Lab", "[LAB] launching candidate %q with raw filter: %s", cand.Name, cleanFilter)
	logger.Infof("Lab", "[LAB] argv: %s %s", binPath, strings.Join(fullArgv, " "))

	stderrBuf := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, binPath, fullArgv...)
	cmd.SysProcAttr = GetHiddenSysProcAttr()
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		logger.Errorf("Lab", "[LAB] failed to start candidate %q: %v", cand.Name, err)
		return nil, fmt.Errorf("failed to start candidate process: %w", err)
	}

	pid := cmd.Process.Pid
	proc := &OSZapretCandidateProcess{
		cmd:       cmd,
		pid:       pid,
		argv:      fullArgv,
		stderrBuf: stderrBuf,
	}

	// Readiness check: wait 300ms to verify process didn't crash on invalid args or driver conflict
	time.Sleep(300 * time.Millisecond)

	// Check if process exited prematurely
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		proc.Stop()
		return nil, fmt.Errorf("candidate exited immediately: stderr=%s", stderrBuf.String())
	}

	logger.Infof("Lab", "[LAB] candidate %q successfully started and ready (PID=%d)", cand.Name, pid)
	return proc, nil
}
