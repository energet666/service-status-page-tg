package checks

import (
	"context"
	"testing"
	"time"
)

type fakeMonitorChecker struct {
	results [][]Result
	calls   int
}

func (f *fakeMonitorChecker) Check(context.Context) []Result {
	if f.calls >= len(f.results) {
		f.calls++
		return f.results[len(f.results)-1]
	}
	results := f.results[f.calls]
	f.calls++
	return results
}

type fakeAvailabilityNotifier struct {
	problemCalls  int
	recoveryCalls int
	results       [][]Result
}

func (f *fakeAvailabilityNotifier) NotifyAvailabilityProblems(results []Result) error {
	f.problemCalls++
	f.results = append(f.results, results)
	return nil
}

func (f *fakeAvailabilityNotifier) NotifyAvailabilityRecovered(results []Result) error {
	f.recoveryCalls++
	f.results = append(f.results, results)
	return nil
}

func TestMonitorNotifiesOnNewProblemOnly(t *testing.T) {
	down := []Result{{Name: "Broken", URL: "https://broken.example/", State: StateDown, Error: "timeout"}}
	checker := &fakeMonitorChecker{results: [][]Result{down, down, down}}
	notifier := &fakeAvailabilityNotifier{}
	monitor := NewMonitor(checker, notifier, time.Minute)

	monitor.CheckNow(context.Background())
	monitor.CheckNow(context.Background())

	if notifier.problemCalls != 1 {
		t.Fatalf("problem notifier calls = %d, want 1", notifier.problemCalls)
	}
}

func TestMonitorNotifiesRecoveryAndThenNewProblem(t *testing.T) {
	checker := &fakeMonitorChecker{results: [][]Result{
		{{Name: "Broken", URL: "https://broken.example/", State: StateDown, Error: "timeout"}},
		{{Name: "Broken", URL: "https://broken.example/", State: StateDown, Error: "timeout"}},
		{{Name: "Broken", URL: "https://broken.example/", State: StateUp}},
		{{Name: "Broken", URL: "https://broken.example/", State: StateDown, Error: "timeout"}},
		{{Name: "Broken", URL: "https://broken.example/", State: StateDown, Error: "timeout"}},
	}}
	notifier := &fakeAvailabilityNotifier{}
	monitor := NewMonitor(checker, notifier, time.Minute)

	monitor.CheckNow(context.Background())
	monitor.CheckNow(context.Background())
	monitor.CheckNow(context.Background())

	if notifier.problemCalls != 2 {
		t.Fatalf("problem notifier calls = %d, want 2", notifier.problemCalls)
	}
	if notifier.recoveryCalls != 1 {
		t.Fatalf("recovery notifier calls = %d, want 1", notifier.recoveryCalls)
	}
}

func TestMonitorDoesNotNotifyWhenAllTargetsAreUp(t *testing.T) {
	checker := &fakeMonitorChecker{results: [][]Result{
		{{Name: "OK", URL: "https://ok.example/", State: StateUp}},
	}}
	notifier := &fakeAvailabilityNotifier{}
	monitor := NewMonitor(checker, notifier, time.Minute)

	monitor.CheckNow(context.Background())

	if notifier.problemCalls != 0 {
		t.Fatalf("problem notifier calls = %d, want 0", notifier.problemCalls)
	}
	if notifier.recoveryCalls != 0 {
		t.Fatalf("recovery notifier calls = %d, want 0", notifier.recoveryCalls)
	}
}

func TestMonitorSkipsProblemNotificationWhenRetrySucceeds(t *testing.T) {
	checker := &fakeMonitorChecker{results: [][]Result{
		{{Name: "Flaky", URL: "https://flaky.example/", State: StateDown, Error: "timeout"}},
		{{Name: "Flaky", URL: "https://flaky.example/", State: StateUp}},
	}}
	notifier := &fakeAvailabilityNotifier{}
	monitor := NewMonitor(checker, notifier, time.Minute)

	monitor.CheckNow(context.Background())

	if notifier.problemCalls != 0 {
		t.Fatalf("problem notifier calls = %d, want 0", notifier.problemCalls)
	}
	if notifier.recoveryCalls != 0 {
		t.Fatalf("recovery notifier calls = %d, want 0", notifier.recoveryCalls)
	}
}
