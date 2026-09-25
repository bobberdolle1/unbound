package autotunevnext

import (
	"context"
	"fmt"
	"strings"

	"unbound/engine"
	"unbound/engine/backendcap"
)

// RunCoordinated is the production entrypoint. The experiment core itself is
// injectable; this thin boundary shares the application's AutoTune lock.
func RunCoordinated(ctx context.Context, request Request, observer Observer, executor Executor, preflight HostPreflight, assets AssetResolver) (Result, error) {
	release, err := engine.GetCoordinator().Acquire(engine.OpAutoTune, "autotune-vnext", false)
	if err != nil {
		return Result{}, err
	}
	defer release()
	operationCtx, cancel := context.WithCancel(ctx)
	engine.GetCoordinator().SetCancelFunc(cancel)
	defer cancel()
	return Run(operationCtx, request, observer, executor, preflight, assets), nil
}

// LinuxExactPlan deliberately keeps NFQUEUE acquisition out of nfqws2 argv.
type LinuxExactPlan struct {
	EngineArgv []string
	Capture    backendcap.CapturePlan
	Assets     []ResolvedAsset
}

func NewLinuxExactPlan(candidate ExecutableCandidate) (LinuxExactPlan, error) {
	if candidate.Plan.Capture.BackendKind != backendcap.CaptureNFQUEUE {
		return LinuxExactPlan{}, fmt.Errorf("expected NFQUEUE capture, got %s", candidate.Plan.Capture.BackendKind)
	}
	for _, arg := range candidate.Plan.EngineArgv {
		if strings.HasPrefix(arg, "--wf-") {
			return LinuxExactPlan{}, fmt.Errorf("Linux engine argv must not contain WinDivert capture flag %q", arg)
		}
		if strings.Contains(arg, "${asset:") {
			return LinuxExactPlan{}, fmt.Errorf("unresolved logical asset in engine argv")
		}
	}
	return LinuxExactPlan{EngineArgv: append([]string(nil), candidate.Plan.EngineArgv...), Capture: candidate.Plan.Capture, Assets: append([]ResolvedAsset(nil), candidate.Assets...)}, nil
}

// WindowsExactPlan preserves compiler output and generates capture argv only
// from structured CapturePlan; it never falls back to legacy default capture.
type WindowsExactPlan struct {
	EngineArgv  []string
	CaptureArgv []string
	Assets      []ResolvedAsset
}

func NewWindowsExactPlan(candidate ExecutableCandidate) (WindowsExactPlan, error) {
	for _, arg := range candidate.Plan.EngineArgv {
		if strings.HasPrefix(arg, "--wf-") {
			return WindowsExactPlan{}, fmt.Errorf("engine argv contains capture flag %q", arg)
		}
		if strings.Contains(arg, "${asset:") {
			return WindowsExactPlan{}, fmt.Errorf("unresolved logical asset in engine argv")
		}
	}
	captureArgv, err := backendcap.RenderWindowsCaptureArgv(candidate.Plan.Capture)
	if err != nil {
		return WindowsExactPlan{}, err
	}
	return WindowsExactPlan{EngineArgv: append([]string(nil), candidate.Plan.EngineArgv...), CaptureArgv: captureArgv, Assets: append([]ResolvedAsset(nil), candidate.Assets...)}, nil
}

// MacOSMeasurementPath describes the V1 restriction. A raw direct observer
// cannot establish that a SOCKS tpws path carried the tested request.
func MacOSMeasurementPath(backend backendcap.Backend) HostPreflightResult {
	if backend == backendcap.Zapret1TPWSDarwin {
		return HostPreflightResult{Status: PreflightUnsupported, Reasons: []Reason{{Code: "MEASUREMENT_PATH_UNSUPPORTED", Detail: "macOS tpws requires an explicit SOCKS-aware Observatory connector."}}}
	}
	return HostPreflightResult{Status: PreflightSupported}
}
