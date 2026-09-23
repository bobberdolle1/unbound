package observatory

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SaveObservation persists one explicitly requested, privacy-redacted result
// under the owned Unbound user configuration path. Observe never calls this;
// callers opt in to persistence.
func SaveObservation(result ObservationResult) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	directory := filepath.Join(configDir, "Unbound", "observations")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", fmt.Errorf("create observation directory: %w", err)
	}
	result = sanitizeForPersistence(result)
	if result.RunID == "" {
		result.RunID = newRunID()
	}
	name := result.StartedAt.UTC().Format("20060102T150405.000000000Z") + "-" + result.RunID + ".json"
	path := filepath.Join(directory, name)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode observation: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".observation-*.json")
	if err != nil {
		return "", err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return "", err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return "", err
	}
	return path, nil
}

func sanitizeForPersistence(result ObservationResult) ObservationResult {
	result.Target.URL = sanitizeURLString(result.Target.URL)
	for attemptIndex := range result.Attempts {
		for stageIndex := range result.Attempts[attemptIndex].Stages {
			stage := &result.Attempts[attemptIndex].Stages[stageIndex]
			stage.ResponseHeaders = allowedResponseHeaderMap(stage.ResponseHeaders)
			stage.Detail = boundedDetail(stage.Detail)
			stage.Redirect = boundedDetail(stage.Redirect)
		}
	}
	return result
}

func sanitizeURLString(raw string) string {
	if index := strings.Index(raw, "?"); index >= 0 {
		raw = raw[:index]
	}
	if scheme := strings.Index(raw, "://"); scheme >= 0 {
		prefix, rest := raw[:scheme+3], raw[scheme+3:]
		if at := strings.Index(rest, "@"); at >= 0 {
			raw = prefix + rest[at+1:]
		}
	}
	return raw
}

func allowedResponseHeaderMap(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string]string, 2)
	for _, key := range []string{"content-type", "location"} {
		if value := headers[key]; value != "" {
			result[key] = boundedDetail(value)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func newRunID() string {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}
