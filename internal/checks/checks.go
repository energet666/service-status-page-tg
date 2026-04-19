package checks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	StateUp        = "up"
	StateHTTPError = "http_error"
	StateDown      = "down"
)

var DefaultTargets = []Target{
	{Name: "YouTube", URL: "https://www.youtube.com/"},
	{Name: "Instagram", URL: "https://www.instagram.com/"},
}

type Target struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Result struct {
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	State      string    `json:"state"`
	LatencyMs  int64     `json:"latencyMs"`
	StatusCode int       `json:"statusCode,omitempty"`
	CheckedAt  time.Time `json:"checkedAt"`
	Error      string    `json:"error"`
}

type Checker struct {
	targets []Target
	client  *http.Client
	now     func() time.Time
}

type configFile struct {
	Targets []Target `json:"targets"`
}

func New(path string) (*Checker, error) {
	targets, err := LoadTargets(path)
	if err != nil {
		return nil, err
	}
	return NewWithClient(targets, &http.Client{Timeout: 5 * time.Second}), nil
}

func NewWithClient(targets []Target, client *http.Client) *Checker {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &Checker{
		targets: normalizeTargets(targets),
		client:  client,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func LoadTargets(path string) ([]Target, error) {
	if strings.TrimSpace(path) == "" {
		return cloneTargets(DefaultTargets), nil
	}

	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cloneTargets(DefaultTargets), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read checks config: %w", err)
	}

	var cfg configFile
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("decode checks config: %w", err)
	}
	if len(cfg.Targets) == 0 {
		return cloneTargets(DefaultTargets), nil
	}

	return normalizeTargets(cfg.Targets), nil
}

func (c *Checker) Check(ctx context.Context) []Result {
	results := make([]Result, len(c.targets))
	var wg sync.WaitGroup
	for i, target := range c.targets {
		wg.Add(1)
		go func(i int, target Target) {
			defer wg.Done()
			results[i] = c.checkOne(ctx, target)
		}(i, target)
	}
	wg.Wait()
	return results
}

func (c *Checker) checkOne(ctx context.Context, target Target) Result {
	start := c.now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		return Result{
			Name:      target.Name,
			URL:       target.URL,
			State:     StateDown,
			CheckedAt: start,
			Error:     err.Error(),
		}
	}

	res, err := c.client.Do(req)
	result := Result{
		Name:  target.Name,
		URL:   target.URL,
		State: StateDown,
	}
	if err != nil {
		checkedAt := c.now()
		result.CheckedAt = checkedAt
		result.LatencyMs = latencyMs(start, checkedAt)
		result.Error = err.Error()
		return result
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	checkedAt := c.now()
	result.CheckedAt = checkedAt
	result.LatencyMs = latencyMs(start, checkedAt)

	result.StatusCode = res.StatusCode
	if res.StatusCode >= 200 && res.StatusCode < 400 {
		result.State = StateUp
		return result
	}

	result.State = StateHTTPError
	result.Error = res.Status
	return result
}

func normalizeTargets(targets []Target) []Target {
	if len(targets) == 0 {
		return cloneTargets(DefaultTargets)
	}

	out := make([]Target, 0, len(targets))
	for _, target := range targets {
		target.Name = strings.TrimSpace(target.Name)
		target.URL = normalizeURL(target.URL)
		if target.URL == "" {
			continue
		}
		if target.Name == "" {
			target.Name = target.URL
		}
		out = append(out, target)
	}
	if len(out) == 0 {
		return cloneTargets(DefaultTargets)
	}
	return out
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

func latencyMs(start, end time.Time) int64 {
	latency := end.Sub(start).Milliseconds()
	if latency < 0 {
		return 0
	}
	return latency
}

func cloneTargets(targets []Target) []Target {
	out := make([]Target, len(targets))
	copy(out, targets)
	return out
}
