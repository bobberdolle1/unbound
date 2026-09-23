package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"unbound/engine/attribution"
	"unbound/engine/observatory"
)

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }

func (values *stringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("file path must not be empty")
	}
	*values = append(*values, value)
	return nil
}

// runAttribution reads only stored JSON. Analysis is pure; this path does not
// observe the network or control profiles, providers, proxies, routes, DNS, or firewalls.
func runAttribution(targetFiles, controlFiles stringList, jsonOutput bool) {
	targets, err := loadObservationFiles(targetFiles)
	if err != nil {
		fatalAttribution(err)
	}
	var controls []observatory.ObservationResult
	if len(controlFiles) > 0 {
		controls, err = loadObservationFiles(controlFiles)
		if err != nil {
			fatalAttribution(err)
		}
	}
	report := attribution.AnalyzeCohort(attribution.Cohort{Target: targets, Controls: controls})
	if jsonOutput {
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			fatalAttribution(fmt.Errorf("encode attribution report: %w", err))
		}
		return
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatalAttribution(fmt.Errorf("format attribution report: %w", err))
	}
	fmt.Println(string(data))
}

func fatalAttribution(err error) {
	fmt.Fprintln(os.Stderr, "attribute:", err)
	os.Exit(1)
}

func loadObservationFiles(paths []string) ([]observatory.ObservationResult, error) {
	var observations []observatory.ObservationResult
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", path, err)
		}
		decoded, err := decodeObservationJSON(data)
		if err != nil {
			return nil, fmt.Errorf("decode %q: %w", path, err)
		}
		observations = append(observations, decoded...)
	}
	if len(observations) == 0 {
		return nil, fmt.Errorf("no ObservationResult objects found")
	}
	return observations, nil
}

// decodeObservationJSON accepts one ObservationResult, an array of results, or
// a JSON evidence envelope containing ObservationResult objects. The recursive
// envelope reader supports archived evidence without treating non-observation
// metadata as evidence.
func decodeObservationJSON(data []byte) ([]observatory.ObservationResult, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	var observations []observatory.ObservationResult
	collectObservationJSON(value, &observations)
	if len(observations) == 0 {
		return nil, fmt.Errorf("no ObservationResult objects found")
	}
	sort.Slice(observations, func(left, right int) bool {
		if observations[left].RunID != observations[right].RunID {
			return observations[left].RunID < observations[right].RunID
		}
		return observations[left].StartedAt.Before(observations[right].StartedAt)
	})
	return observations, nil
}

func collectObservationJSON(value any, observations *[]observatory.ObservationResult) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectObservationJSON(item, observations)
		}
	case map[string]any:
		if isObservationObject(typed) {
			data, err := json.Marshal(typed)
			if err == nil {
				var observation observatory.ObservationResult
				if json.Unmarshal(data, &observation) == nil {
					*observations = append(*observations, observation)
				}
			}
			return
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			collectObservationJSON(typed[key], observations)
		}
	}
}

func isObservationObject(value map[string]any) bool {
	_, hasSchema := value["schema_version"]
	_, hasTarget := value["target"]
	_, hasAttempts := value["attempts"]
	return hasSchema && hasTarget && hasAttempts
}
