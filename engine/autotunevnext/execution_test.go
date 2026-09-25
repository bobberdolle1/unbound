package autotunevnext

import (
	"context"
	"testing"

	"unbound/engine"
)

func TestRunCoordinatedRespectsExistingAutoTuneOwner(t *testing.T) {
	release, err := engine.GetCoordinator().Acquire(engine.OpAutoTune, "another-owner", false)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	executor := &fakeExecutor{}
	_, err = RunCoordinated(context.Background(), request(tlsStrategy("tls")), &fakeObserver{}, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if err == nil {
		t.Fatal("coordinated run bypassed existing AutoTune owner")
	}
	if len(executor.calls) != 0 {
		t.Fatalf("coordinator conflict mutated executor: %v", executor.calls)
	}
}
