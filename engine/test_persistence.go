package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type TestSession struct {
	ID          string                 `json:"id"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     time.Time              `json:"end_time"`
	Duration    time.Duration          `json:"duration"`
	ProfileName string                 `json:"profile_name"`
	TestMode    string                 `json:"test_mode"`
	Results     []TestResultPersistent `json:"results"`
	Score       int                    `json:"score"`
	SuccessRate float64                `json:"success_rate"`
	BestProfile string                 `json:"best_profile,omitempty"`
}

type TestResultPersistent struct {
	URL        string        `json:"url"`
	Success    bool          `json:"success"`
	Latency    time.Duration `json:"latency"`
	Error      string        `json:"error,omitempty"`
	StatusCode int           `json:"status_code,omitempty"`
	TCPFreeze  bool          `json:"tcp_freeze,omitempty"`
}

type TestAnalytics struct {
	TotalSessions   int                      `json:"total_sessions"`
	TotalTests      int                      `json:"total_tests"`
	SuccessfulTests int                      `json:"successful_tests"`
	FailedTests     int                      `json:"failed_tests"`
	AverageScore    float64                  `json:"average_score"`
	ProfileStats    map[string]*ProfileStats `json:"profile_stats"`
	LastUpdated     time.Time                `json:"last_updated"`
}

type ProfileStats struct {
	ProfileName     string        `json:"profile_name"`
	TestCount       int           `json:"test_count"`
	SuccessCount    int           `json:"success_count"`
	FailureCount    int           `json:"failure_count"`
	AverageLatency  time.Duration `json:"average_latency"`
	AverageScore    float64       `json:"average_score"`
	LastTested      time.Time     `json:"last_tested"`
	RecommendedRank int           `json:"recommended_rank"`
}

func GetTestResultsDir() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}

	resultsDir := filepath.Join(configDir, "test_results")

	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return "", err
	}

	return resultsDir, nil
}

func SaveTestSession(session *TestSession) error {
	resultsDir, err := GetTestResultsDir()
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("test_%s_%s.json",
		session.StartTime.Format("20060102_150405"),
		session.ID)

	filepath := filepath.Join(resultsDir, filename)

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0644)
}

func LoadTestSession(sessionID string) (*TestSession, error) {
	resultsDir, err := GetTestResultsDir()
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(resultsDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		data, err := os.ReadFile(filepath.Join(resultsDir, file.Name()))
		if err != nil {
			continue
		}

		var session TestSession
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		if session.ID == sessionID {
			return &session, nil
		}
	}

	return nil, fmt.Errorf("session not found: %s", sessionID)
}

func LoadAllTestSessions() ([]*TestSession, error) {
	resultsDir, err := GetTestResultsDir()
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(resultsDir)
	if err != nil {
		return nil, err
	}

	sessions := make([]*TestSession, 0)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		data, err := os.ReadFile(filepath.Join(resultsDir, file.Name()))
		if err != nil {
			continue
		}

		var session TestSession
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		sessions = append(sessions, &session)
	}

	return sessions, nil
}

func GenerateTestAnalytics() (*TestAnalytics, error) {
	sessions, err := LoadAllTestSessions()
	if err != nil {
		return nil, err
	}

	analytics := &TestAnalytics{
		ProfileStats: make(map[string]*ProfileStats),
		LastUpdated:  time.Now(),
	}

	totalScore := 0

	for _, session := range sessions {
		analytics.TotalSessions++
		analytics.TotalTests += len(session.Results)
		totalScore += session.Score

		for _, result := range session.Results {
			if result.Success {
				analytics.SuccessfulTests++
			} else {
				analytics.FailedTests++
			}
		}

		if _, exists := analytics.ProfileStats[session.ProfileName]; !exists {
			analytics.ProfileStats[session.ProfileName] = &ProfileStats{
				ProfileName: session.ProfileName,
			}
		}

		stats := analytics.ProfileStats[session.ProfileName]
		stats.TestCount++
		stats.AverageScore = (stats.AverageScore*float64(stats.TestCount-1) + float64(session.Score)) / float64(stats.TestCount)

		successCount := 0
		totalLatency := time.Duration(0)

		for _, result := range session.Results {
			if result.Success {
				successCount++
				totalLatency += result.Latency
			}
		}

		stats.SuccessCount += successCount
		stats.FailureCount += (len(session.Results) - successCount)

		if successCount > 0 {
			avgLatency := totalLatency / time.Duration(successCount)
			stats.AverageLatency = (stats.AverageLatency*time.Duration(stats.TestCount-1) + avgLatency) / time.Duration(stats.TestCount)
		}

		if session.EndTime.After(stats.LastTested) {
			stats.LastTested = session.EndTime
		}
	}

	if analytics.TotalSessions > 0 {
		analytics.AverageScore = float64(totalScore) / float64(analytics.TotalSessions)
	}

	rankProfiles(analytics.ProfileStats)

	return analytics, nil
}

func rankProfiles(profileStats map[string]*ProfileStats) {
	type rankedProfile struct {
		name  string
		score float64
	}

	ranked := make([]rankedProfile, 0, len(profileStats))

	for name, stats := range profileStats {
		score := stats.AverageScore

		if stats.SuccessCount > 0 {
			successRate := float64(stats.SuccessCount) / float64(stats.SuccessCount+stats.FailureCount)
			score += successRate * 100
		}

		if stats.AverageLatency > 0 && stats.AverageLatency < 1*time.Second {
			score += 50
		}

		ranked = append(ranked, rankedProfile{name: name, score: score})
	}

	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].score > ranked[i].score {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	for i, rp := range ranked {
		profileStats[rp.name].RecommendedRank = i + 1
	}
}

func SaveTestAnalytics(analytics *TestAnalytics) error {
	resultsDir, err := GetTestResultsDir()
	if err != nil {
		return err
	}

	filepath := filepath.Join(resultsDir, "analytics.json")

	data, err := json.MarshalIndent(analytics, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0644)
}

func LoadTestAnalytics() (*TestAnalytics, error) {
	resultsDir, err := GetTestResultsDir()
	if err != nil {
		return nil, err
	}

	filepath := filepath.Join(resultsDir, "analytics.json")

	data, err := os.ReadFile(filepath)
	if err != nil {
		return GenerateTestAnalytics()
	}

	var analytics TestAnalytics
	if err := json.Unmarshal(data, &analytics); err != nil {
		return GenerateTestAnalytics()
	}

	return &analytics, nil
}

func CleanOldTestResults(olderThan time.Duration) error {
	resultsDir, err := GetTestResultsDir()
	if err != nil {
		return err
	}

	files, err := os.ReadDir(resultsDir)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-olderThan)

	for _, file := range files {
		if file.IsDir() || file.Name() == "analytics.json" {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(resultsDir, file.Name()))
		}
	}

	return nil
}
