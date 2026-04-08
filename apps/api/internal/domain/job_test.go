package domain

import "testing"

func TestValidTransitionAllowed(t *testing.T) {
	cases := []struct {
		from JobStatus
		to   JobStatus
	}{
		{JobQueued, JobRunning},
		{JobQueued, JobCancelled},
		{JobQueued, JobExpired},
		{JobRunning, JobSucceeded},
		{JobRunning, JobFailed},
		{JobRunning, JobCancelled},
	}
	for _, tc := range cases {
		if err := ValidTransition(tc.from, tc.to); err != nil {
			t.Errorf("ValidTransition(%s → %s) should be allowed, got: %v", tc.from, tc.to, err)
		}
	}
}

func TestValidTransitionRejected(t *testing.T) {
	cases := []struct {
		from JobStatus
		to   JobStatus
	}{
		{JobQueued, JobSucceeded},
		{JobQueued, JobFailed},
		{JobRunning, JobQueued},
		{JobRunning, JobExpired},
		{JobSucceeded, JobRunning},
		{JobSucceeded, JobFailed},
		{JobFailed, JobRunning},
		{JobCancelled, JobRunning},
	}
	for _, tc := range cases {
		if err := ValidTransition(tc.from, tc.to); err == nil {
			t.Errorf("ValidTransition(%s → %s) should be rejected", tc.from, tc.to)
		}
	}
}

func TestJobStatusIsTerminal(t *testing.T) {
	terminal := []JobStatus{JobSucceeded, JobFailed, JobCancelled, JobExpired}
	nonTerminal := []JobStatus{JobQueued, JobRunning}

	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("expected %s to be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("expected %s to be non-terminal", s)
		}
	}
}
