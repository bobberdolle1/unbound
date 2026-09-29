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

func TestCandidateExecutionContextFollowsCallerCancellation(t *testing.T) {
	caller, cancelCaller := context.WithCancel(context.Background())
	candidate, cancelCandidate := candidateExecutionContext(caller)
	defer cancelCandidate()
	cancelCaller()
	select {
	case <-candidate.Done():
	default:
		t.Fatal("candidate context outlived canceled experiment caller")
	}
}
