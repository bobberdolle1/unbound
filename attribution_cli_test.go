package main

import (
	"os"
	"testing"
)

func TestDecodeAttributionFixtureEnvelope(t *testing.T) {
	data, err := os.ReadFile("engine/attribution/testdata/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	observations, err := decodeObservationJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) < 10 {
		t.Fatalf("decoded %d observations, want fixture envelope contents", len(observations))
	}
}

func TestAttributionFlagDoesNotRequireElevation(t *testing.T) {
	if requiresElevationForMode(false, false, false, true, false, false, false, false, false, false, false) {
		t.Fatal("stored evidence attribution must not require elevation")
	}
}
