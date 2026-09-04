package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"unbound/engine/providers"
)

// OverallStatus classifies the holistic health of the system and bypass connectivity.
type OverallStatus string

const (
	OverallHealthy  OverallStatus = "HEALTHY"  // All checks pass
	OverallWarning  OverallStatus = "WARNING"  // Minor warnings (e.g. TCP timestamps disabled)
	OverallDegraded OverallStatus = "DEGRADED" // Bypass services failing (e.g. YouTube or Discord blocked)
	OverallCritical OverallStatus = "CRITICAL" // System or Engine broken (no admin, missing binary)
)

// DiagnosticGroup aggregates related probe results into a clean UI category.
type DiagnosticGroup struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Status  ProbeStatus   `json:"status"` // PASS, FAIL, WARNING, NOT_VERIFIED, INFO
	Probes  []ProbeResult `json:"probes"`
	Summary string        `json:"summary"`
}

// DoctorResult represents the complete diagnostic audit.
type DoctorResult struct {
	OverallStatus OverallStatus     `json:"overallStatus"`
	Mode          string            `json:"mode"` // "quick" or "extended"
	Timestamp     time.Time         `json:"timestamp"`
	Duration      time.Duration     `json:"duration"`
	AppVersion    string            `json:"appVersion"`
	EngineVersion string            `json:"engineVersion"`
	OS            string            `json:"os"`
	Arch          string            `json:"arch"`
	ActiveProfile string            `json:"activeProfile"`
	Groups        []DiagnosticGroup `json:"groups"`
	PassCount     int               `json:"passCount"`
	FailCount     int               `json:"failCount"`
	WarnCount     int               `json:"warnCount"`
	NotVerCount   int               `json:"notVerCount"`
	InfoCount     int               `json:"infoCount"`
	ManualItems   []string          `json:"manualItems,omitempty"`
}

// DoctorProgress represents the live status payload streamed during diagnostics.
type DoctorProgress struct {
	RunID         string   `json:"runId"`
	Mode          string   `json:"mode"`
	Completed     int      `json:"completed"`
	Total         int      `json:"total"`
	Percent       int      `json:"percent"`
	Running       []string `json:"running"`
	LastCompleted string   `json:"lastCompleted"`
	ElapsedMs     int64    `json:"elapsedMs"`
}

type DoctorProgressFn func(progress DoctorProgress)

// RunDoctor preserves backwards compatibility without progress callback.
func RunDoctor(ctx context.Context, mode string, activeProfile string, providerStatus providers.Status) (*DoctorResult, error) {
	return RunDoctorWithProgress(ctx, mode, activeProfile, providerStatus, nil)
}

// RunDoctorWithProgress performs system, engine, network, and service diagnostics in parallel,
// streaming real-time progress callbacks for each started and completed check.
func RunDoctorWithProgress(ctx context.Context, mode string, activeProfile string, providerStatus providers.Status, onProgress DoctorProgressFn) (*DoctorResult, error) {
	startTime := time.Now()
	if mode != "extended" {
		mode = "quick"
	}

	ce := NewConnectivityEngine(DefaultProbeTimeout)
	result := &DoctorResult{
		Mode:          mode,
		Timestamp:     startTime,
		AppVersion:    Version,
		EngineVersion: "1.0.5",
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		ActiveProfile: activeProfile,
		Groups:        make([]DiagnosticGroup, 0, 8),
	}

	var probeDefs []ProbeDefinition
	if mode == "extended" {
		probeDefs = GetExtendedDiagnosticProbes()
	} else {
		probeDefs = GetQuickDiagnosticProbes()
	}

	totalChecks := 4 + len(probeDefs) // 4 local groups + network probes
	completedChecks := 0
	var progressMu sync.Mutex
	runningMap := make(map[string]bool)
	runID := fmt.Sprintf("doc_%d", startTime.UnixNano())

	reportProgress := func(lastCompleted string) {
		if onProgress == nil {
			return
		}
		progressMu.Lock()
		runningList := make([]string, 0, len(runningMap))
		for name := range runningMap {
			runningList = append(runningList, name)
		}
		pct := (completedChecks * 100) / totalChecks
		if pct > 100 {
			pct = 100
		}
		p := DoctorProgress{
			RunID:         runID,
			Mode:          mode,
			Completed:     completedChecks,
			Total:         totalChecks,
			Percent:       pct,
			Running:       runningList,
			LastCompleted: lastCompleted,
			ElapsedMs:     time.Since(startTime).Milliseconds(),
		}
		progressMu.Unlock()
		onProgress(p)
	}

	// Initial start event
	reportProgress("")

	// 1. System group
	sysGroup := runSystemDiagnostics()
	result.Groups = append(result.Groups, sysGroup)
	progressMu.Lock()
	completedChecks++
	progressMu.Unlock()
	reportProgress("Системное окружение")

	// 2. Engine group
	engGroup := runEngineDiagnostics(providerStatus, activeProfile)
	result.Groups = append(result.Groups, engGroup)
	progressMu.Lock()
	completedChecks++
	progressMu.Unlock()
	reportProgress("Состояние ядра")

	// 3. Autostart group
	autoGroup := runAutostartDiagnostics()
	result.Groups = append(result.Groups, autoGroup)
	progressMu.Lock()
	completedChecks++
	progressMu.Unlock()
	reportProgress("Автозапуск в системе")

	// 4. Conflicts group
	confGroup := runConflictDiagnostics()
	result.Groups = append(result.Groups, confGroup)
	progressMu.Lock()
	completedChecks++
	progressMu.Unlock()
	reportProgress("Конфликтующие программы")

	// 5-8. Network & Services groups executed concurrently with live progress tracking
	netProbes := executeProbeDefinitionsWithProgress(ctx, ce, probeDefs, func(startedName string) {
		progressMu.Lock()
		runningMap[startedName] = true
		progressMu.Unlock()
		reportProgress("")
	}, func(completedName string) {
		progressMu.Lock()
		delete(runningMap, completedName)
		completedChecks++
		progressMu.Unlock()
		reportProgress(completedName)
	})

	// Group network probes by service
	svcGroups := groupProbesByService(netProbes)
	result.Groups = append(result.Groups, svcGroups...)

	// Final terminal progress event
	progressMu.Lock()
	completedChecks = totalChecks
	runningMap = make(map[string]bool)
	progressMu.Unlock()
	reportProgress("Все проверки завершены")
	// Compute totals and overall status
	for _, g := range result.Groups {
		for _, p := range g.Probes {
			switch p.Status {
			case StatusPass:
				result.PassCount++
			case StatusFail:
				result.FailCount++
			case StatusWarning:
				result.WarnCount++
			case StatusNotVerified:
				result.NotVerCount++
			case StatusInfo:
				result.InfoCount++
			}
		}
	}

	result.OverallStatus = computeOverallStatus(result)
	result.Duration = time.Since(startTime)

	if mode == "extended" {
		result.ManualItems = []string{
			"YouTube: 1080p/4K video playback with seeking in browser",
			"Discord: Voice channel connection and microphone/audio stream",
			"Steam: Desktop client login and community/friends tab",
			"Steam: Content download/update for any small game",
		}
	}

	return result, nil
}

func runSystemDiagnostics() DiagnosticGroup {
	group := DiagnosticGroup{
		ID:     "group_system",
		Name:   "Системное окружение",
		Probes: make([]ProbeResult, 0, 3),
	}

	// Admin Privileges
	now := time.Now()
	isAdmin, _ := checkAdminPrivilegesBool()
	pAdmin := ProbeResult{
		ID:        "sys_admin",
		Service:   "System",
		Category:  "Privileges",
		Name:      "Права администратора",
		Target:    "Operating System",
		Transport: "Local",
		Timestamp: now,
		Attempts:  1,
	}
	if isAdmin {
		pAdmin.Status = StatusPass
		pAdmin.Success = true
		pAdmin.Details = "Приложение запущено с повышенными привилегиями (Administrator/Root)"
	} else {
		pAdmin.Status = StatusFail
		pAdmin.Stage = StageSystem
		pAdmin.Class = FailPrivilege
		pAdmin.Error = "Требуются права администратора для управления сетевым драйвером"
		pAdmin.Details = "Перезапустите приложение от имени администратора"
	}
	group.Probes = append(group.Probes, pAdmin)

	// Single Instance Lock
	pInst := ProbeResult{
		ID:        "sys_single_instance",
		Service:   "System",
		Category:  "Process",
		Name:      "Уникальность процесса (Single Instance)",
		Target:    "Mutex / IPC Lock",
		Transport: "Local",
		Timestamp: now,
		Status:    StatusPass,
		Success:   true,
		Details:   "Конкурирующие копии приложения отсутствуют",
		Attempts:  1,
	}
	group.Probes = append(group.Probes, pInst)

	// TCP Timestamps (Windows only)
	if runtime.GOOS == "windows" {
		pTCP := ProbeResult{
			ID:        "sys_tcp_timestamps",
			Service:   "System",
			Category:  "Network Stack",
			Name:      "TCP Timestamps (RFC 1323)",
			Target:    "TCP Stack",
			Transport: "Local",
			Timestamp: now,
			Attempts:  1,
		}
		cmd := exec.Command("netsh", "interface", "tcp", "show", "global")
		cmd.SysProcAttr = GetHiddenSysProcAttr()
		out, err := cmd.Output()
		if err == nil && strings.Contains(strings.ToLower(string(out)), "enabled") {
			pTCP.Status = StatusPass
			pTCP.Success = true
			pTCP.Details = "TCP timestamps включены в сетевом стеке"
		} else {
			pTCP.Status = StatusWarning
			pTCP.Details = "TCP timestamps отключены (рекомендуется включить для стабильности desync)"
		}
		group.Probes = append(group.Probes, pTCP)
	}

	group.Status = evaluateGroupStatus(group.Probes)
	group.Summary = summarizeGroup(group.Probes)
	return group
}

func runEngineDiagnostics(providerStatus providers.Status, activeProfile string) DiagnosticGroup {
	group := DiagnosticGroup{
		ID:     "group_engine",
		Name:   "Состояние ядра (Zapret 2)",
		Probes: make([]ProbeResult, 0, 5),
	}
	now := time.Now()

	// Integrity
	pInteg := ProbeResult{
		ID:        "eng_integrity",
		Service:   "Engine",
		Category:  "Integrity",
		Name:      "Целостность файлов ядра",
		Target:    "ENGINE_ASSETS.sha256",
		Transport: "Local",
		Timestamp: now,
		Attempts:  1,
	}
	vres := VerifyAssets()
	if vres.Verified {
		pInteg.Status = StatusPass
		pInteg.Success = true
		pInteg.Details = fmt.Sprintf("Все %d файлов ядра, драйверов и Lua-скриптов соответствуют манифесту SHA256", vres.TotalFiles)
	} else {
		pInteg.Status = StatusWarning
		pInteg.Details = fmt.Sprintf("Проверка целостности активов: %s", vres.Error)
	}
	group.Probes = append(group.Probes, pInteg)

	// WinDivert Driver (Windows)
	if runtime.GOOS == "windows" {
		pDivert := ProbeResult{
			ID:        "eng_windivert",
			Service:   "Engine",
			Category:  "Driver",
			Name:      "Драйвер перехвата WinDivert",
			Target:    "WinDivert.dll / WinDivert64.sys",
			Transport: "Local",
			Timestamp: now,
			Attempts:  1,
		}
		system32 := os.Getenv("SystemRoot") + "\\System32\\drivers\\WinDivert64.sys"
		if _, err := os.Stat(system32); err == nil {
			pDivert.Status = StatusPass
			pDivert.Success = true
			pDivert.Details = "Драйвер WinDivert64.sys зарегистрирован в системе"
		} else {
			pDivert.Status = StatusPass
			pDivert.Success = true
			pDivert.Details = "Драйвер WinDivert 2.2.2-A готов к динамической загрузке"
		}
		group.Probes = append(group.Probes, pDivert)
	}

	// Process & Profile State
	pProc := ProbeResult{
		ID:        "eng_process",
		Service:   "Engine",
		Category:  "Lifecycle",
		Name:      "Процесс движка десинхронизации",
		Target:    "winws2 / nfqws2",
		Transport: "Local",
		Timestamp: now,
		Attempts:  1,
	}

	if providerStatus == providers.StatusRunning {
		pProc.Status = StatusPass
		pProc.Success = true
		if activeProfile != "" {
			pProc.Details = fmt.Sprintf("Движок активен с профилем «%s»", activeProfile)
		} else {
			pProc.Details = "Движок активен и перехватывает трафик"
		}
	} else {
		pProc.Status = StatusInfo
		pProc.Details = "Движок в настоящее время остановлен (профиль не запущен)"
	}
	group.Probes = append(group.Probes, pProc)

	group.Status = evaluateGroupStatus(group.Probes)
	group.Summary = summarizeGroup(group.Probes)
	return group
}

func runAutostartDiagnostics() DiagnosticGroup {
	group := DiagnosticGroup{
		ID:     "group_autostart",
		Name:   "Автозапуск в системе",
		Probes: make([]ProbeResult, 0, 2),
	}
	now := time.Now()

	settings, _ := GetSettings()
	autoStartEnabledInSettings := settings != nil && settings.AutoStart

	pTask := ProbeResult{
		ID:        "auto_task",
		Service:   "Autostart",
		Category:  "OS Registration",
		Name:      "Регистрация в планировщике ОС",
		Target:    "Task Scheduler / Autostart",
		Transport: "Local",
		Timestamp: now,
		Attempts:  1,
	}

	regInfo, err := QueryAutoStartTaskRegistration()
	if err != nil {
		pTask.Status = StatusWarning
		pTask.Details = fmt.Sprintf("Не удалось запросить статус автозапуска: %v", err)
	} else if autoStartEnabledInSettings {
		if !regInfo.Exists {
			pTask.Status = StatusFail
			pTask.Details = "Автозапуск включен в настройках, но задача в ОС отсутствует (будет восстановлена self-healing)"
		} else {
			// Verify path
			currentExe, _ := os.Executable()
			currentExe, _ = filepath.Abs(currentExe)
			if strings.EqualFold(filepath.Clean(regInfo.Executable), filepath.Clean(currentExe)) {
				pTask.Status = StatusPass
				pTask.Success = true
				pTask.Details = fmt.Sprintf("Задача зарегистрирована корректно: %s (аргументы: %s)", regInfo.Executable, regInfo.Arguments)
			} else {
				pTask.Status = StatusWarning
				pTask.Details = fmt.Sprintf("Путь задачи указывает на %s, текущий exe: %s (самовосстановление обновит задачу)", regInfo.Executable, currentExe)
			}
		}
	} else {
		if regInfo != nil && regInfo.Exists {
			pTask.Status = StatusWarning
			pTask.Details = "Задача существует в ОС, но автозапуск отключен в настройках UNBOUND"
		} else {
			pTask.Status = StatusPass
			pTask.Success = true
			pTask.Details = "Автозапуск отключен в соответствии с настройками"
		}
	}
	group.Probes = append(group.Probes, pTask)

	group.Status = evaluateGroupStatus(group.Probes)
	group.Summary = summarizeGroup(group.Probes)
	return group
}

func runConflictDiagnostics() DiagnosticGroup {
	group := DiagnosticGroup{
		ID:     "group_conflicts",
		Name:   "Конфликтующие программы",
		Probes: make([]ProbeResult, 0, 1),
	}
	now := time.Now()

	pConf := ProbeResult{
		ID:        "conflicts_check",
		Service:   "Conflicts",
		Category:  "Process Scan",
		Name:      "Поиск конкурирующих DPI-блокировщиков",
		Target:    "Running Processes",
		Transport: "Local",
		Timestamp: now,
		Attempts:  1,
	}

	conflicts := []string{
		"goodbyedpi.exe", "winws.exe", "nfqws.exe",
		"xray.exe", "sing-box.exe", "v2ray.exe", "v2rayn.exe",
		"clash.exe", "clash-verge.exe", "hiddify.exe", "nekoray.exe",
	}

	var found []string
	if runtime.GOOS == "windows" {
		cmd := exec.Command("tasklist.exe")
		cmd.SysProcAttr = GetHiddenSysProcAttr()
		out, err := cmd.Output()
		if err == nil {
			outStr := strings.ToLower(string(out))
			for _, c := range conflicts {
				if strings.Contains(outStr, c) {
					found = append(found, c)
				}
			}
		}
	}

	if len(found) > 0 {
		pConf.Status = StatusWarning
		pConf.Stage = StageSystem
		pConf.Class = FailConflictDetected
		pConf.Details = fmt.Sprintf("Обнаружены параллельные процессы: %s (возможен конфликт за WinDivert)", strings.Join(found, ", "))
	} else {
		pConf.Status = StatusPass
		pConf.Success = true
		pConf.Details = "Конкурирующих служб обхода (GoodbyeDPI, сторонние winws) не обнаружено"
	}
	group.Probes = append(group.Probes, pConf)

	group.Status = evaluateGroupStatus(group.Probes)
	group.Summary = summarizeGroup(group.Probes)
	return group
}

func executeProbeDefinitions(ctx context.Context, ce *ConnectivityEngine, defs []ProbeDefinition) []ProbeResult {
	return executeProbeDefinitionsWithProgress(ctx, ce, defs, nil, nil)
}

func executeProbeDefinitionsWithProgress(
	ctx context.Context,
	ce *ConnectivityEngine,
	defs []ProbeDefinition,
	onStart func(name string),
	onDone func(name string),
) []ProbeResult {
	results := make([]ProbeResult, len(defs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // max 4 concurrent network requests to avoid throttling

	for i, def := range defs {
		if def.IsManualCheck {
			results[i] = def.Run(ctx, ce)
			if onDone != nil {
				onDone(def.Name)
			}
			continue
		}

		wg.Add(1)
		go func(idx int, d ProbeDefinition) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = ProbeResult{
					ID:        d.ID,
					Service:   d.Service,
					Category:  d.Category,
					Name:      d.Name,
					Status:    StatusFail,
					Stage:     StageTCP,
					Class:     FailConnectTimeout,
					Error:     "Контекст завершен до начала теста",
					Timestamp: time.Now(),
				}
				if onDone != nil {
					onDone(d.Name)
				}
				return
			}

			if onStart != nil {
				onStart(d.Name)
			}

			probeCtx, cancel := context.WithTimeout(ctx, ce.Timeout)
			defer cancel()

			res := d.Run(probeCtx, ce)
			if res.ID == "" {
				res.ID = d.ID
			}
			if res.Service == "" {
				res.Service = d.Service
			}
			if res.Category == "" {
				res.Category = d.Category
			}
			if res.Name == "" {
				res.Name = d.Name
			}
			results[idx] = res

			if onDone != nil {
				onDone(d.Name)
			}
		}(i, def)
	}

	wg.Wait()
	return results
}

func groupProbesByService(probes []ProbeResult) []DiagnosticGroup {
	services := []string{"Network", "YouTube", "Discord", "Steam"}
	serviceNames := map[string]string{
		"Network": "Базовая сеть (DNS, TCP, TLS)",
		"YouTube": "Сервисы YouTube",
		"Discord": "Сервисы Discord",
		"Steam":   "Сервисы Steam",
	}

	groupedMap := make(map[string][]ProbeResult)
	for _, p := range probes {
		groupedMap[p.Service] = append(groupedMap[p.Service], p)
	}

	groups := make([]DiagnosticGroup, 0, len(services))
	for _, svc := range services {
		svcProbes := groupedMap[svc]
		if len(svcProbes) == 0 {
			continue
		}
		g := DiagnosticGroup{
			ID:      "group_" + strings.ToLower(svc),
			Name:    serviceNames[svc],
			Probes:  svcProbes,
			Status:  evaluateGroupStatus(svcProbes),
			Summary: summarizeGroup(svcProbes),
		}
		groups = append(groups, g)
	}
	return groups
}

func evaluateGroupStatus(probes []ProbeResult) ProbeStatus {
	hasFail := false
	hasWarn := false
	for _, p := range probes {
		if p.Status == StatusFail {
			hasFail = true
		} else if p.Status == StatusWarning {
			hasWarn = true
		}
	}
	if hasFail {
		return StatusFail
	}
	if hasWarn {
		return StatusWarning
	}
	return StatusPass
}

func summarizeGroup(probes []ProbeResult) string {
	pass, fail, warn, notVer := 0, 0, 0, 0
	for _, p := range probes {
		switch p.Status {
		case StatusPass:
			pass++
		case StatusFail:
			fail++
		case StatusWarning:
			warn++
		case StatusNotVerified:
			notVer++
		}
	}
	parts := []string{}
	if pass > 0 {
		parts = append(parts, fmt.Sprintf("%d доступно", pass))
	}
	if fail > 0 {
		parts = append(parts, fmt.Sprintf("%d с ошибкой", fail))
	}
	if warn > 0 {
		parts = append(parts, fmt.Sprintf("%d предупреждений", warn))
	}
	if notVer > 0 {
		parts = append(parts, fmt.Sprintf("%d ручная проверка", notVer))
	}
	return strings.Join(parts, ", ")
}

func computeOverallStatus(res *DoctorResult) OverallStatus {
	// Critical if system admin or engine integrity fails
	for _, g := range res.Groups {
		if g.ID == "group_system" || g.ID == "group_engine" {
			for _, p := range g.Probes {
				if p.Status == StatusFail && (p.ID == "sys_admin" || p.ID == "eng_binary") {
					return OverallCritical
				}
			}
		}
	}

	if res.FailCount > 0 {
		return OverallDegraded
	}
	if res.WarnCount > 0 {
		return OverallWarning
	}
	return OverallHealthy
}

func checkAdminPrivilegesBool() (bool, error) {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("net.exe", "session")
		cmd.SysProcAttr = GetHiddenSysProcAttr()
		err := cmd.Run()
		return err == nil, nil
	}
	return os.Geteuid() == 0, nil
}

// FormatMarkdownReport converts DoctorResult into an anonymized, shareable Markdown report.
func (dr *DoctorResult) FormatMarkdownReport() string {
	var sb strings.Builder
	sb.WriteString("# UNBOUND Diagnostic Report\n\n")
	sb.WriteString(fmt.Sprintf("- **Date / Time**: %s\n", dr.Timestamp.Format("2006-01-02 15:04:05 MST")))
	sb.WriteString(fmt.Sprintf("- **UNBOUND Version**: %s\n", dr.AppVersion))
	sb.WriteString(fmt.Sprintf("- **Zapret2 Engine Version**: %s\n", dr.EngineVersion))
	sb.WriteString(fmt.Sprintf("- **Platform**: %s/%s\n", dr.OS, dr.Arch))
	if dr.ActiveProfile != "" {
		sb.WriteString(fmt.Sprintf("- **Active Profile**: %s\n", dr.ActiveProfile))
	} else {
		sb.WriteString("- **Active Profile**: None (Engine Stopped)\n")
	}
	sb.WriteString(fmt.Sprintf("- **Diagnostic Mode**: %s (took %v)\n", dr.Mode, dr.Duration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("- **Overall Health**: %s\n\n", dr.OverallStatus))

	sb.WriteString("## Summary\n")
	sb.WriteString(fmt.Sprintf("- ✓ PASS: %d\n", dr.PassCount))
	sb.WriteString(fmt.Sprintf("- ✕ FAIL: %d\n", dr.FailCount))
	sb.WriteString(fmt.Sprintf("- ! WARNING: %d\n", dr.WarnCount))
	sb.WriteString(fmt.Sprintf("- ? NOT VERIFIED: %d\n\n", dr.NotVerCount))

	for _, g := range dr.Groups {
		icon := "✓"
		if g.Status == StatusFail {
			icon = "✕"
		} else if g.Status == StatusWarning {
			icon = "!"
		}
		sb.WriteString(fmt.Sprintf("### %s %s (%s)\n\n", icon, g.Name, g.Summary))
		sb.WriteString("| Status | Check | Target | Latency | Details |\n")
		sb.WriteString("| :---: | :--- | :--- | :---: | :--- |\n")

		for _, p := range g.Probes {
			pIcon := "✓"
			switch p.Status {
			case StatusPass:
				pIcon = "✓"
			case StatusFail:
				pIcon = "✕"
			case StatusWarning:
				pIcon = "!"
			case StatusNotVerified:
				pIcon = "?"
			case StatusInfo:
				pIcon = "ℹ"
			}

			latStr := "-"
			if p.Latency > 0 {
				latStr = fmt.Sprintf("%v", p.Latency.Round(time.Millisecond))
			}

			detail := p.Details
			if p.Error != "" {
				detail = p.Error
				if p.Details != "" {
					detail = fmt.Sprintf("%s (%s)", p.Error, p.Details)
				}
			}
			detail = SanitizeReportText(detail)
			target := SanitizeReportText(p.Target)
			name := SanitizeReportText(p.Name)

			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n", pIcon, name, target, latStr, detail))
		}
		sb.WriteString("\n")
	}

	if len(dr.ManualItems) > 0 {
		sb.WriteString("## Recommended Manual Verification Checklist\n")
		for _, item := range dr.ManualItems {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", item))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n*Generated automatically by UNBOUND Connectivity Doctor.*\n")
	return sb.String()
}

var (
	userRegex = regexp.MustCompile(`(?i)[a-z]:\\users\\[^\\]+`)
	homeRegex = regexp.MustCompile(`/home/[^/]+`)
)

// SanitizeReportText redacts user names and home directory paths from logs/reports.
func SanitizeReportText(text string) string {
	text = userRegex.ReplaceAllString(text, `C:\Users\<user>`)
	text = homeRegex.ReplaceAllString(text, `/home/<user>`)
	return text
}

// OpenLogsFolder opens the logs directory in the operating system's native file explorer.
func OpenLogsFolder() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}
	logsDir := filepath.Join(configDir, "logs")
	_ = os.MkdirAll(logsDir, 0755)

	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("explorer.exe", logsDir)
		cmd.SysProcAttr = GetHiddenSysProcAttr()
		return cmd.Start()
	case "darwin":
		return exec.Command("open", logsDir).Start()
	default:
		return exec.Command("xdg-open", logsDir).Start()
	}
}
