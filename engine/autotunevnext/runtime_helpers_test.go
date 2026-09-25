package autotunevnext

import (
	"context"
	"testing"
	"time"
)

func TestCandidateExecutionContextPreservesCallerDeadline(t *testing.T) {
	deadline := time.Now().Add(3 * time.Minute)
	caller, cancelCaller := context.WithDeadline(context.Background(), deadline)
	defer cancelCaller()
	candidate, cancelCandidate := candidateExecutionContext(caller)
	defer cancelCandidate()
	got, ok := candidate.Deadline()
	if !ok || !got.Equal(deadline) {
		t.Fatalf("candidate deadline=%v present=%t, want caller deadline=%v", got, ok, deadline)
	}
}
