package checks

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

type AvailabilityNotifier interface {
	NotifyAvailabilityProblems([]Result) error
	NotifyAvailabilityRecovered([]Result) error
}

type Monitor struct {
	checker interface {
		Check(context.Context) []Result
	}
	notifier AvailabilityNotifier
	interval time.Duration

	lastProblemKey string
}

func NewMonitor(checker interface {
	Check(context.Context) []Result
}, notifier AvailabilityNotifier, interval time.Duration) *Monitor {
	return &Monitor{
		checker:  checker,
		notifier: notifier,
		interval: interval,
	}
}

func (m *Monitor) Run(ctx context.Context) {
	if m == nil || m.checker == nil || m.notifier == nil || m.interval <= 0 {
		return
	}

	m.CheckNow(ctx)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.CheckNow(ctx)
		}
	}
}

func (m *Monitor) CheckNow(ctx context.Context) {
	results := m.checker.Check(ctx)
	key := problemKey(results)
	if key == "" {
		if m.lastProblemKey != "" {
			if err := m.notifier.NotifyAvailabilityRecovered(results); err != nil {
				log.Printf("failed to send availability recovery alert: %v", err)
			}
		}
		m.lastProblemKey = ""
		return
	}
	if key == m.lastProblemKey {
		return
	}

	m.lastProblemKey = key
	if err := m.notifier.NotifyAvailabilityProblems(results); err != nil {
		log.Printf("failed to send availability alert: %v", err)
	}
}

func problemKey(results []Result) string {
	var parts []string
	for _, result := range results {
		if result.State == StateUp {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%s", result.Name, result.URL, result.State, result.StatusCode, result.Error))
	}
	return strings.Join(parts, "\x01")
}
