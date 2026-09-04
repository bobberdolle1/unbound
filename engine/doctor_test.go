package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"unbound/engine/providers"
)

func TestSanitizeReportText(t *testing.T) {
	input := `Task registered at C:\Users\kiril\AppData\Local\Unbound\unbound.exe, user home is /home/john/projects`
	sanitized := SanitizeReportText(input)

	if strings.Contains(sanitized, `C:\Users\kiril`) {
		t.Errorf("Username not sanitized in Windows path: %s", sanitized)
	}
	if strings.Contains(sanitized, `/home/john`) {
		t.Errorf("Username not sanitized in Linux path: %s", sanitized)
	}
	if !strings.Contains(sanitized, `C:\Users\<user>`) {
		t.Errorf("Expected placeholder in Windows path: %s", sanitized)
	}
	if !strings.Contains(sanitized, `/home/<user>`) {
		t.Errorf("Expected placeholder in Linux path: %s", sanitized)
	}
}

func TestEvaluateGroupStatus(t *testing.T) {
	// All PASS
	gPass := []ProbeResult{
		{Status: StatusPass},
		{Status: StatusPass},
	}
	if st := evaluateGroupStatus(gPass); st != StatusPass {
		t.Errorf("Expected StatusPass, got %s", st)
	}

	// Contains Warning
	gWarn := []ProbeResult{
		{Status: StatusPass},
		{Status: StatusWarning},
	}
	if st := evaluateGroupStatus(gWarn); st != StatusWarning {
		t.Errorf("Expected StatusWarning, got %s", st)
	}

	// Contains Fail (Fail trumps Warning)
	gFail := []ProbeResult{
		{Status: StatusPass},
		{Status: StatusWarning},
		{Status: StatusFail},
	}
	if st := evaluateGroupStatus(gFail); st != StatusFail {
		t.Errorf("Expected StatusFail, got %s", st)
	}
}

func TestComputeOverallStatus(t *testing.T) {
	// Healthy
	rHealthy := &DoctorResult{PassCount: 10}
	if st := computeOverallStatus(rHealthy); st != OverallHealthy {
		t.Errorf("Expected OverallHealthy, got %s", st)
	}

	// Warning
	rWarn := &DoctorResult{PassCount: 10, WarnCount: 1}
	if st := computeOverallStatus(rWarn); st != OverallWarning {
		t.Errorf("Expected OverallWarning, got %s", st)
	}

	// Degraded
	rDegraded := &DoctorResult{PassCount: 10, FailCount: 2}
	if st := computeOverallStatus(rDegraded); st != OverallDegraded {
		t.Errorf("Expected OverallDegraded, got %s", st)
	}

	// Critical when admin probe fails
	rCritical := &DoctorResult{
		Groups: []DiagnosticGroup{
			{
				ID: "group_system",
				Probes: []ProbeResult{
					{ID: "sys_admin", Status: StatusFail},
				},
			},
		},
		FailCount: 1,
	}
	if st := computeOverallStatus(rCritical); st != OverallCritical {
		t.Errorf("Expected OverallCritical, got %s", st)
	}
}

func TestFormatMarkdownReport(t *testing.T) {
	dr := &DoctorResult{
		OverallStatus: OverallHealthy,
		Mode:          "quick",
		Timestamp:     time.Now(),
		Duration:      1200 * time.Millisecond,
		AppVersion:    "0.5.0",
		EngineVersion: "1.0.5",
		OS:            "windows",
		Arch:          "amd64",
		ActiveProfile: "Recommended (hostfakesplit)",
		PassCount:     5,
		FailCount:     0,
		WarnCount:     0,
		Groups: []DiagnosticGroup{
			{
				Name:    "YouTube",
				Status:  StatusPass,
				Summary: "2 доступно",
				Probes: []ProbeResult{
					{
						Name:    "YouTube Web",
						Target:  "https://www.youtube.com/generate_204",
						Status:  StatusPass,
						Latency: 45 * time.Millisecond,
						Details: "HTTP 204",
					},
				},
			},
		},
		ManualItems: []string{
			"YouTube 1080p playback check",
		},
	}

	md := dr.FormatMarkdownReport()
	if !strings.Contains(md, "# UNBOUND Diagnostic Report") {
		t.Error("Missing title in report")
	}
	if !strings.Contains(md, "UNBOUND Version**: 0.5.0") {
		t.Error("Missing app version in report")
	}
	if !strings.Contains(md, "YouTube Web") {
		t.Error("Missing probe name in report table")
	}
	if !strings.Contains(md, "YouTube 1080p playback check") {
		t.Error("Missing manual checklist in report")
	}
}

func TestRunDoctorQuickExecution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := RunDoctor(ctx, "quick", "Recommended (hostfakesplit)", providers.StatusRunning)
	if err != nil {
		t.Fatalf("RunDoctor failed: %v", err)
	}

	if res.OverallStatus == "" {
		t.Error("OverallStatus is empty")
	}
	if len(res.Groups) == 0 {
		t.Error("No groups returned by RunDoctor")
	}
	t.Logf("Doctor completed in %v, overall status: %s (PASS=%d FAIL=%d WARN=%d INFO=%d)",
		res.Duration, res.OverallStatus, res.PassCount, res.FailCount, res.WarnCount, res.InfoCount)
}
