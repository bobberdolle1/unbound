package main

import (
	"reflect"
	"testing"
)

func TestProductVNextExperimentRegistryIsDeterministicAndValid(t *testing.T) {
	first := productVNextExperimentConfig()
	second := productVNextExperimentConfig()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("registry is non-deterministic: first=%+v second=%+v", first, second)
	}
	if len(first.Targets) != 3 {
		t.Fatalf("targets=%d, want 3", len(first.Targets))
	}
	for _, preset := range append(append([]AutoTuneVNextTargetPreset(nil), first.Targets...), first.DefaultControl) {
		_, public, err := normalizeVNextTarget(preset.Target)
		if err != nil {
			t.Fatalf("preset %q target %q: %v", preset.ID, preset.Target, err)
		}
		if public != preset.Target {
			t.Fatalf("preset %q public target=%q, want %q", preset.ID, public, preset.Target)
		}
	}

	first.Targets[0].Target = "https://mutated.example/"
	if got := productVNextExperimentConfig().Targets[0].Target; got != "https://www.youtube.com/generate_204" {
		t.Fatalf("registry mutation leaked into product state: %q", got)
	}
}

func TestResolveProductVNextExperimentTargetUsesDefaultControl(t *testing.T) {
	target, controls, public, err := resolveProductVNextExperimentTarget("youtube-web", "")
	if err != nil {
		t.Fatal(err)
	}
	if target != "https://www.youtube.com/generate_204" || public != target {
		t.Fatalf("target=%q public=%q", target, public)
	}
	if len(controls) != 1 || controls[0] != "https://cloudflare.com/cdn-cgi/trace" {
		t.Fatalf("controls=%v", controls)
	}
}

func TestResolveProductVNextExperimentTargetValidatesCustomHTTPS(t *testing.T) {
	target, controls, public, err := resolveProductVNextExperimentTarget("custom", "https://Example.test/path?token=secret#fragment")
	if err != nil {
		t.Fatal(err)
	}
	if target != "https://example.test/path?token=secret" || public != "https://example.test/path" || len(controls) != 1 {
		t.Fatalf("target=%q public=%q controls=%v", target, public, controls)
	}
	for _, raw := range []string{
		"http://example.test/",
		"https://user:password@example.test/",
		"https:///missing-host",
	} {
		if _, _, _, err := resolveProductVNextExperimentTarget("custom", raw); err == nil {
			t.Fatalf("accepted unsafe custom target %q", raw)
		}
	}
	if _, _, _, err := resolveProductVNextExperimentTarget("unknown", ""); err == nil {
		t.Fatal("accepted unknown preset")
	}
}
