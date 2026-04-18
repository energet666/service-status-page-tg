package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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
