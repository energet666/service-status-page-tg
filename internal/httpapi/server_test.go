package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"service-status-page/internal/store"
)

type fakeNotifier struct {
	reports []store.Report
}

func (f *fakeNotifier) NotifyReport(report store.Report) error {
	f.reports = append(f.reports, report)
	return nil
}

func TestGetStatusOnEmptyState(t *testing.T) {
	handler := newTestHandler(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	var body struct {
		Status store.Status `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status.State != store.StatusOK {
		t.Fatalf("state = %q, want %q", body.Status.State, store.StatusOK)
	}
}

func TestStatusEventsStreamInitialAndUpdatedState(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	handler := New(st, nil, filepath.Join(t.TempDir(), "dist"))
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/status/events", nil).WithContext(ctx)
	res := newSSETestWriter()
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(res, req)
	}()

	initial := readSSEStatus(t, res)
	if statusCode := res.statusCode(); statusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", statusCode, http.StatusOK)
	}
	if contentType := res.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("content type = %q, want text/event-stream", contentType)
	}
	if initial.Status.State != store.StatusOK {
		t.Fatalf("initial state = %q, want %q", initial.Status.State, store.StatusOK)
	}

	if _, err := st.SetStatus(store.StatusIncident, "Проверочный инцидент", "test"); err != nil {
		t.Fatal(err)
	}

	updated := readSSEStatus(t, res)
	if updated.Status.State != store.StatusIncident {
		t.Fatalf("updated state = %q, want %q", updated.Status.State, store.StatusIncident)
	}

	cancel()
	<-done
}

func TestCreateReport(t *testing.T) {
	notifier := &fakeNotifier{}
	handler := newTestHandler(t, notifier)
	req := httptest.NewRequest(http.MethodPost, "/api/reports", bytes.NewBufferString(`{"message":"Кнопка не работает","name":"Анна"}`))
	req.RemoteAddr = "192.0.2.10:1234"
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusCreated, res.Body.String())
	}
	if len(notifier.reports) != 1 {
		t.Fatalf("notifier reports = %d, want 1", len(notifier.reports))
	}
}

func TestCreateReportRequiresMessage(t *testing.T) {
	handler := newTestHandler(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/reports", bytes.NewBufferString(`{"message":" "}`))
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestCreateReportRateLimit(t *testing.T) {
	handler := newTestHandler(t, nil)

	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/reports", bytes.NewBufferString(`{"message":"bug"}`))
		req.RemoteAddr = "192.0.2.20:1234"
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if i < 5 && res.Code != http.StatusCreated {
			t.Fatalf("request %d status = %d, want %d", i, res.Code, http.StatusCreated)
		}
		if i == 5 && res.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d status = %d, want %d", i, res.Code, http.StatusTooManyRequests)
		}
	}
}

func newTestHandler(t *testing.T, notifier ReportNotifier) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return New(st, notifier, filepath.Join(t.TempDir(), "dist"))
}

type sseTestWriter struct {
	header http.Header
	ch     chan string

	mu     sync.Mutex
	status int
}

func newSSETestWriter() *sseTestWriter {
	return &sseTestWriter{
		header: make(http.Header),
		ch:     make(chan string, 10),
	}
}

func (w *sseTestWriter) Header() http.Header {
	return w.header
}

func (w *sseTestWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		w.status = status
	}
}

func (w *sseTestWriter) Write(b []byte) (int, error) {
	w.ch <- string(b)
	return len(b), nil
}

func (w *sseTestWriter) Flush() {}

func (w *sseTestWriter) statusCode() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func readSSEStatus(t *testing.T, writer *sseTestWriter) struct {
	Status store.Status `json:"status"`
} {
	t.Helper()

	for {
		var chunk string
		select {
		case chunk = <-writer.ch:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for SSE event")
		}

		dataLine := ""
		for _, line := range strings.Split(chunk, "\n") {
			if strings.HasPrefix(line, "data: ") {
				dataLine = strings.TrimPrefix(line, "data: ")
				break
			}
		}
		if dataLine == "" {
			continue
		}

		var body struct {
			Status store.Status `json:"status"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(dataLine)), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
}
