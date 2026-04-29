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

	"service-status-page/internal/checks"
	"service-status-page/internal/store"
)

type fakeNotifier struct {
	reports []store.Report
}

func (f *fakeNotifier) NotifyReport(report store.Report) error {
	f.reports = append(f.reports, report)
	return nil
}

type fakeChecker struct {
	results []checks.Result
}

func (f fakeChecker) Check(context.Context) []checks.Result {
	return f.results
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

func TestGetStatusIncludesPinnedInfo(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetPinnedInfo("Инструкция для клиентов", "admin"); err != nil {
		t.Fatal(err)
	}

	handler := New(st, nil, nil, filepath.Join(t.TempDir(), "dist"))
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	var body struct {
		PinnedInfo *store.PinnedInfo `json:"pinnedInfo"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.PinnedInfo == nil {
		t.Fatal("pinnedInfo = nil, want value")
	}
	if body.PinnedInfo.Message != "Инструкция для клиентов" {
		t.Fatalf("pinnedInfo message = %q", body.PinnedInfo.Message)
	}
}

func TestGetStatusUsesChecksWhenAllTargetsAreUp(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAnnouncement("Администратор проверяет сервис", store.AnnouncementIncident, "admin"); err != nil {
		t.Fatal(err)
	}
	handler := New(st, nil, fakeChecker{results: []checks.Result{
		{Name: "Example", URL: "https://example.com/", State: checks.StateUp, CheckedAt: time.Now().UTC()},
	}}, filepath.Join(t.TempDir(), "dist"))
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	var body struct {
		Status             publicStatus        `json:"status"`
		ActiveAnnouncement *store.Announcement `json:"activeAnnouncement"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status.State != store.StatusOK {
		t.Fatalf("state = %q, want %q", body.Status.State, store.StatusOK)
	}
	if body.Status.Source != "checks" {
		t.Fatalf("source = %q, want checks", body.Status.Source)
	}
	if body.Status.ChecksTotal != 1 || body.Status.ChecksFailed != 0 {
		t.Fatalf("check counts = %d/%d, want 1/0", body.Status.ChecksTotal, body.Status.ChecksFailed)
	}
	if body.ActiveAnnouncement == nil {
		t.Fatal("activeAnnouncement is nil, want admin announcement")
	}
	if body.ActiveAnnouncement.Kind != store.AnnouncementIncident {
		t.Fatalf("active announcement kind = %q, want %q", body.ActiveAnnouncement.Kind, store.AnnouncementIncident)
	}
}

func TestGetStatusUsesIncidentWhenAnyTargetIsDown(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	handler := New(st, nil, fakeChecker{results: []checks.Result{
		{Name: "Up", URL: "https://up.example/", State: checks.StateUp, CheckedAt: time.Now().UTC()},
		{Name: "Down", URL: "https://down.example/", State: checks.StateDown, CheckedAt: time.Now().UTC(), Error: "timeout"},
	}}, filepath.Join(t.TempDir(), "dist"))
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	var body struct {
		Status             publicStatus        `json:"status"`
		ActiveAnnouncement *store.Announcement `json:"activeAnnouncement"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status.State != store.StatusIncident {
		t.Fatalf("state = %q, want %q", body.Status.State, store.StatusIncident)
	}
	if body.Status.ChecksTotal != 2 || body.Status.ChecksFailed != 1 {
		t.Fatalf("check counts = %d/%d, want 2/1", body.Status.ChecksTotal, body.Status.ChecksFailed)
	}
}

func TestGetStatusUsesIncidentWhenAnyTargetHasHTTPError(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	handler := New(st, nil, fakeChecker{results: []checks.Result{
		{Name: "Broken", URL: "https://broken.example/", State: checks.StateHTTPError, StatusCode: http.StatusServiceUnavailable, CheckedAt: time.Now().UTC()},
	}}, filepath.Join(t.TempDir(), "dist"))
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	var body struct {
		Status             publicStatus        `json:"status"`
		ActiveAnnouncement *store.Announcement `json:"activeAnnouncement"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status.State != store.StatusIncident {
		t.Fatalf("state = %q, want %q", body.Status.State, store.StatusIncident)
	}
	if body.Status.ChecksFailed != 1 {
		t.Fatalf("checksFailed = %d, want 1", body.Status.ChecksFailed)
	}
}

func TestGetStatusFallsBackToLatestAdminAnnouncementWithoutChecks(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAnnouncement("Администратор проверяет сервис", store.AnnouncementInfo, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAnnouncement("Пользовательское сообщение", store.AnnouncementUser, "user"); err != nil {
		t.Fatal(err)
	}
	handler := New(st, nil, nil, filepath.Join(t.TempDir(), "dist"))
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	var body struct {
		Status             publicStatus        `json:"status"`
		ActiveAnnouncement *store.Announcement `json:"activeAnnouncement"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status.Source != "announcement" {
		t.Fatalf("source = %q, want announcement", body.Status.Source)
	}
	if body.Status.State != store.StatusOK {
		t.Fatalf("state = %q, want neutral %q", body.Status.State, store.StatusOK)
	}
	if body.Status.Message != "Администратор проверяет сервис" {
		t.Fatalf("message = %q", body.Status.Message)
	}
	if body.ActiveAnnouncement == nil {
		t.Fatal("activeAnnouncement is nil, want admin announcement")
	}
	if body.ActiveAnnouncement.Message != "Администратор проверяет сервис" {
		t.Fatalf("active announcement message = %q", body.ActiveAnnouncement.Message)
	}
}

func TestGetStatusClearedAnnouncementRemovesActiveAnnouncement(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAnnouncement("Ведутся работы", store.AnnouncementMaintenance, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAnnouncement("Объявление снято", store.AnnouncementCleared, "admin"); err != nil {
		t.Fatal(err)
	}
	handler := New(st, nil, nil, filepath.Join(t.TempDir(), "dist"))
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	var body struct {
		ActiveAnnouncement *store.Announcement  `json:"activeAnnouncement"`
		Announcements      []store.Announcement `json:"announcements"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.ActiveAnnouncement != nil {
		t.Fatalf("activeAnnouncement = %#v, want nil", body.ActiveAnnouncement)
	}
	if len(body.Announcements) == 0 {
		t.Fatal("announcements is empty")
	}
	if body.Announcements[0].Kind != store.AnnouncementCleared {
		t.Fatalf("latest announcement kind = %q, want %q", body.Announcements[0].Kind, store.AnnouncementCleared)
	}
}

func TestGetStatusFallsBackToDefaultWithoutChecksOrAnnouncements(t *testing.T) {
	handler := newTestHandler(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	var body struct {
		Status publicStatus `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status.Source != "default" {
		t.Fatalf("source = %q, want default", body.Status.Source)
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

	handler := New(st, nil, nil, filepath.Join(t.TempDir(), "dist"))
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
	if updated.Status.Source != "announcement" {
		t.Fatalf("updated source = %q, want announcement", updated.Status.Source)
	}
	if updated.Status.State != store.StatusOK {
		t.Fatalf("updated state = %q, want neutral %q", updated.Status.State, store.StatusOK)
	}
	if updated.Status.Message != "Проверочный инцидент" {
		t.Fatalf("updated message = %q", updated.Status.Message)
	}

	cancel()
	<-done
}

func TestStatusEventsStopOnServerShutdown(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	handler := New(st, nil, nil, filepath.Join(t.TempDir(), "dist"))
	req := httptest.NewRequest(http.MethodGet, "/api/status/events", nil)
	res := newSSETestWriter()
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(res, req)
	}()

	readSSEStatus(t, res)
	handler.Shutdown()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SSE handler to stop")
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

func TestCreateReportAddsUserAnnouncementToStatus(t *testing.T) {
	handler := newTestHandler(t, nil)
	createReq := httptest.NewRequest(http.MethodPost, "/api/reports", bytes.NewBufferString(`{"message":"Кнопка не работает","name":"Анна","contact":"@anna"}`))
	createReq.RemoteAddr = "192.0.2.10:1234"
	createRes := httptest.NewRecorder()
	handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d: %s", createRes.Code, http.StatusCreated, createRes.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	statusRes := httptest.NewRecorder()
	handler.ServeHTTP(statusRes, statusReq)
	if statusRes.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", statusRes.Code, http.StatusOK)
	}

	var body struct {
		Announcements []store.Announcement `json:"announcements"`
	}
	if err := json.NewDecoder(statusRes.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Announcements) != 1 {
		t.Fatalf("announcements = %d, want 1", len(body.Announcements))
	}
	ann := body.Announcements[0]
	if ann.Kind != store.AnnouncementUser {
		t.Fatalf("announcement kind = %q, want %q", ann.Kind, store.AnnouncementUser)
	}
	if ann.Message != "Кнопка не работает" {
		t.Fatalf("announcement message = %q", ann.Message)
	}
	if ann.CreatedBy != "Анна" {
		t.Fatalf("announcement createdBy = %q, want Анна", ann.CreatedBy)
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

func TestCreateReportRejectsReservedAdminNames(t *testing.T) {
	for _, name := range []string{"admin", "Admin", "ADMIN", "админ", "Админ", "АДМИН"} {
		t.Run(name, func(t *testing.T) {
			handler := newTestHandler(t, nil)
			req := httptest.NewRequest(http.MethodPost, "/api/reports", bytes.NewBufferString(`{"message":"bug","name":"`+name+`"}`))
			res := httptest.NewRecorder()

			handler.ServeHTTP(res, req)

			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
			}
		})
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

func TestGetChecks(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	checkedAt := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	handler := New(st, nil, fakeChecker{results: []checks.Result{
		{
			Name:       "Example",
			URL:        "https://example.com/",
			State:      checks.StateUp,
			LatencyMs:  42,
			StatusCode: http.StatusOK,
			CheckedAt:  checkedAt,
		},
	}}, filepath.Join(t.TempDir(), "dist"))
	req := httptest.NewRequest(http.MethodGet, "/api/checks", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusOK, res.Body.String())
	}
	var body struct {
		Targets []checks.Result `json:"targets"`
		Meta    struct {
			GeneratedAt time.Time `json:"generatedAt"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(body.Targets))
	}
	if body.Targets[0].Name != "Example" {
		t.Fatalf("target name = %q, want Example", body.Targets[0].Name)
	}
	if body.Targets[0].LatencyMs != 42 {
		t.Fatalf("latency = %d, want 42", body.Targets[0].LatencyMs)
	}
	if body.Meta.GeneratedAt.IsZero() {
		t.Fatal("generatedAt is zero")
	}
}

func newTestHandler(t *testing.T, notifier ReportNotifier) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return New(st, notifier, nil, filepath.Join(t.TempDir(), "dist"))
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
	Status publicStatus `json:"status"`
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
			Status publicStatus `json:"status"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(dataLine)), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
}
