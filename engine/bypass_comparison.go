package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"unbound/engine/providers"
)

// BypassVerdict categorizes the impact of the active profile relative to direct ISP connection.
type BypassVerdict string

const (
	VerdictFixedByProfile    BypassVerdict = "FIXED_BY_PROFILE"    // Baseline FAIL, Profile PASS (Bypass works!)
	VerdictReachableDirectly BypassVerdict = "REACHABLE_DIRECTLY"  // Baseline PASS, Profile PASS (Direct access works)
	VerdictStillBlocked      BypassVerdict = "STILL_BLOCKED"      // Baseline FAIL, Profile FAIL (Strategy did not unblock)
	VerdictBrokenByProfile   BypassVerdict = "BROKEN_BY_PROFILE"   // Baseline PASS, Profile FAIL (Desync broke previously working service!)
	VerdictInconclusive      BypassVerdict = "INCONCLUSIVE"
)

// ComparisonItem holds the baseline and bypass results for a single target.
type ComparisonItem struct {
	ID          string        `json:"id"`
	Service     string        `json:"service"`
	Name        string        `json:"name"`
	Target      string        `json:"target"`
	Baseline    ProbeResult   `json:"baseline"`
	Profile     ProbeResult   `json:"profile"`
	Verdict     BypassVerdict `json:"verdict"`
	Explanation string        `json:"explanation"`
}

// BypassComparisonResult aggregates the complete A/B test results.
type BypassComparisonResult struct {
	Timestamp      time.Time        `json:"timestamp"`
	ProfileName    string           `json:"profileName"`
	Duration       time.Duration    `json:"duration"`
	Items          []ComparisonItem `json:"items"`
	FixedCount     int              `json:"fixedCount"`
	DirectCount    int              `json:"directCount"`
	BlockedCount   int              `json:"blockedCount"`
	BrokenCount    int              `json:"brokenCount"`
	OverallSummary string           `json:"overallSummary"`
}

// ProviderController defines the minimal lifecycle operations required for A/B testing.
type ProviderController interface {
	CurrentProfile() string
	GetStatus() providers.Status
	Start(ctx context.Context, profile string) error
	Stop() error
}

// RunBypassComparison performs an A/B test comparing baseline (no bypass) vs current profile.
// It is strictly transactional: defer guarantees that the original profile and running state
// are restored upon completion, error, panic, or cancellation.
func RunBypassComparison(ctx context.Context, pc ProviderController, probeDefs []ProbeDefinition) (res *BypassComparisonResult, retErr error) {
	if pc == nil {
		return nil, errors.New("provider controller is required for A/B comparison")
	}
	if len(probeDefs) == 0 {
		probeDefs = GetQuickDiagnosticProbes()
	}

	startTime := time.Now()
	initialProfile := pc.CurrentProfile()
	initialStatus := pc.GetStatus()

	// Transactional rollback: ALWAYS restore original engine state and profile
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if initialStatus == providers.StatusRunning && initialProfile != "" {
			if pc.GetStatus() != providers.StatusRunning || pc.CurrentProfile() != initialProfile {
				_ = pc.Start(cleanupCtx, initialProfile)
			}
		} else if initialStatus == providers.StatusStopped {
			if pc.GetStatus() == providers.StatusRunning {
				_ = pc.Stop()
			}
		}
	}()

	profileToTest := initialProfile
	if profileToTest == "" {
		profileToTest = "Recommended (hostfakesplit)"
	}

	ce := NewConnectivityEngine(DefaultProbeTimeout)

	// Step 1: Baseline measurement (without bypass engine)
	if initialStatus == providers.StatusRunning {
		if err := pc.Stop(); err != nil {
			return nil, fmt.Errorf("failed to stop engine for baseline: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}

	baselineResults := executeProbeDefinitions(ctx, ce, probeDefs)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Step 2: Profile measurement (with bypass active)
	if err := pc.Start(ctx, profileToTest); err != nil {
		return nil, fmt.Errorf("failed to start profile %q for comparison: %w", profileToTest, err)
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(500 * time.Millisecond):
	}

	profileResults := executeProbeDefinitions(ctx, ce, probeDefs)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Step 3: Match and evaluate verdicts
	items := make([]ComparisonItem, 0, len(probeDefs))
	fixedCount := 0
	directCount := 0
	blockedCount := 0
	brokenCount := 0

	for i := range probeDefs {
		baseRes := baselineResults[i]
		profRes := profileResults[i]

		item := evaluateComparisonItem(probeDefs[i], baseRes, profRes)
		switch item.Verdict {
		case VerdictFixedByProfile:
			fixedCount++
		case VerdictReachableDirectly:
			directCount++
		case VerdictStillBlocked:
			blockedCount++
		case VerdictBrokenByProfile:
			brokenCount++
		}
		items = append(items, item)
	}

	summary := fmt.Sprintf("Восстановлено профилем: %d | Прямой доступ: %d | Остаются заблокированы: %d",
		fixedCount, directCount, blockedCount)
	if brokenCount > 0 {
		summary += fmt.Sprintf(" | ⚠ СЛОМАНО ПРОФИЛЕМ: %d", brokenCount)
	}

	return &BypassComparisonResult{
		Timestamp:      startTime,
		ProfileName:    profileToTest,
		Duration:       time.Since(startTime),
		Items:          items,
		FixedCount:     fixedCount,
		DirectCount:    directCount,
		BlockedCount:   blockedCount,
		BrokenCount:    brokenCount,
		OverallSummary: summary,
	}, nil
}

func evaluateComparisonItem(def ProbeDefinition, baseRes, profRes ProbeResult) ComparisonItem {
	item := ComparisonItem{
		ID:       def.ID,
		Service:  def.Service,
		Name:     def.Name,
		Baseline: baseRes,
		Profile:  profRes,
	}

	baseOK := baseRes.Status == StatusPass
	profOK := profRes.Status == StatusPass

	switch {
	case !baseOK && profOK:
		item.Verdict = VerdictFixedByProfile
		item.Explanation = "Заблокировано провайдером напрямую; профиль успешно восстановил доступ."
	case baseOK && profOK:
		item.Verdict = VerdictReachableDirectly
		item.Explanation = "Сервис доступен напрямую и продолжает работать через выбранный профиль."
	case !baseOK && !profOK:
		item.Verdict = VerdictStillBlocked
		item.Explanation = "Сервис недоступен напрямую и текущий профиль не смог восстановить соединение."
	case baseOK && !profOK:
		item.Verdict = VerdictBrokenByProfile
		item.Explanation = "ВНИМАНИЕ: Сервис работал напрямую, но профиль нарушил его работу (регрессия desync)."
	default:
		item.Verdict = VerdictInconclusive
		item.Explanation = "Результат неоднозначен (ручная или сервисная проверка)."
	}

	return item
}

// FormatMarkdown converts comparison results to a neat Markdown table.
func (bcr *BypassComparisonResult) FormatMarkdown() string {
	var sb strings.Builder
	sb.WriteString("# UNBOUND A/B Bypass Comparison Report\n\n")
	sb.WriteString(fmt.Sprintf("- **Date / Time**: %s\n", bcr.Timestamp.Format("2006-01-02 15:04:05 MST")))
	sb.WriteString(fmt.Sprintf("- **Tested Profile**: %s\n", bcr.ProfileName))
	sb.WriteString(fmt.Sprintf("- **Duration**: %v\n", bcr.Duration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("- **Summary**: %s\n\n", bcr.OverallSummary))

	sb.WriteString("| Service | Check | Без обхода (Baseline) | С профилем | Вердикт |\n")
	sb.WriteString("| :--- | :--- | :---: | :---: | :--- |\n")

	for _, item := range bcr.Items {
		baseStr := string(item.Baseline.Status)
		profStr := string(item.Profile.Status)
		verdictStr := string(item.Verdict)

		switch item.Verdict {
		case VerdictFixedByProfile:
			verdictStr = "⭐ Восстановлено профилем"
		case VerdictReachableDirectly:
			verdictStr = "✓ Доступно напрямую"
		case VerdictStillBlocked:
			verdictStr = "✕ Заблокировано"
		case VerdictBrokenByProfile:
			verdictStr = "⚠ СЛОМАНО ПРОФИЛЕМ"
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			item.Service, item.Name, baseStr, profStr, verdictStr))
	}
	sb.WriteString("\n")
	return sb.String()
}
