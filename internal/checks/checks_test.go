package checks

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestLoadTargetsUsesDefaultsWhenFileMissing(t *testing.T) {
	targets, err := LoadTargets(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}

	if len(targets) != len(DefaultTargets) {
		t.Fatalf("targets = %d, want %d", len(targets), len(DefaultTargets))
	}
	if targets[0].Name != "YouTube" {
		t.Fatalf("first target = %q, want YouTube", targets[0].Name)
	}
}

func TestLoadTargetsUsesDefaultsWhenConfigIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checks.json")
	if err := os.WriteFile(path, []byte(`{"targets":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	targets, err := LoadTargets(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(targets) != len(DefaultTargets) {
		t.Fatalf("targets = %d, want %d", len(targets), len(DefaultTargets))
	}
}

func TestLoadTargetsNormalizesBareDomains(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checks.json")
	if err := os.WriteFile(path, []byte(`{"targets":[{"name":"Example","url":"example.com"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	targets, err := LoadTargets(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(targets))
	}
	if targets[0].URL != "https://example.com/" {
		t.Fatalf("url = %q, want https://example.com/", targets[0].URL)
	}
}

func TestCheckerStates(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantState  string
	}{
		{name: "up", statusCode: http.StatusNoContent, wantState: StateUp},
		{name: "redirect", statusCode: http.StatusFound, wantState: StateUp},
		{name: "http error", statusCode: http.StatusServiceUnavailable, wantState: StateHTTPError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: tt.statusCode,
						Status:     http.StatusText(tt.statusCode),
						Body:       io.NopCloser(strings.NewReader("ok")),
					}, nil
				}),
			}
			checker := NewWithClient([]Target{{Name: "Test", URL: "https://example.com/"}}, client)
			results := checker.Check(context.Background())

			if len(results) != 1 {
				t.Fatalf("results = %d, want 1", len(results))
			}
			if results[0].State != tt.wantState {
				t.Fatalf("state = %q, want %q", results[0].State, tt.wantState)
			}
			if results[0].StatusCode != tt.statusCode {
				t.Fatalf("statusCode = %d, want %d", results[0].StatusCode, tt.statusCode)
			}
			if results[0].CheckedAt.IsZero() {
				t.Fatal("checkedAt is zero")
			}
		})
	}
}

func TestCheckerReturnsDownOnRequestFailure(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network unavailable")
		}),
	}
	checker := NewWithClient([]Target{{Name: "Broken", URL: "https://broken.example/"}}, client)

	results := checker.Check(context.Background())

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].State != StateDown {
		t.Fatalf("state = %q, want %q", results[0].State, StateDown)
	}
	if results[0].Error == "" {
		t.Fatal("error is empty")
	}
	if results[0].Error != "Не удалось подключиться к сайту" {
		t.Fatalf("error = %q, want connection message", results[0].Error)
	}
}

func TestCheckerReturnsHumanTimeoutMessage(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	}
	checker := NewWithClient([]Target{{Name: "Slow", URL: "https://slow.example/"}}, client)

	results := checker.Check(context.Background())

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].State != StateDown {
		t.Fatalf("state = %q, want %q", results[0].State, StateDown)
	}
	if results[0].Error != "Сайт не ответил за отведенное время" {
		t.Fatalf("error = %q", results[0].Error)
	}
}

func TestCheckerReturnsHumanDNSMessage(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, &net.DNSError{Err: "no such host", Name: "missing.example"}
		}),
	}
	checker := NewWithClient([]Target{{Name: "Missing", URL: "https://missing.example/"}}, client)

	results := checker.Check(context.Background())

	if results[0].Error != "Не удалось найти адрес сайта" {
		t.Fatalf("error = %q", results[0].Error)
	}
}

func TestCheckerLatencyIncludesResponseBodyRead(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Body:       io.NopCloser(strings.NewReader("ok")),
			}, nil
		}),
	}
	checker := NewWithClient([]Target{{Name: "Test", URL: "https://example.com/"}}, client)
	times := []time.Time{
		time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 19, 10, 0, 0, int(75*time.Millisecond), time.UTC),
	}
	checker.now = func() time.Time {
		next := times[0]
		times = times[1:]
		return next
	}

	results := checker.Check(context.Background())

	if results[0].LatencyMs != 75 {
		t.Fatalf("latency = %d, want 75", results[0].LatencyMs)
	}
}
