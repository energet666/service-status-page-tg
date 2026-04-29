package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"service-status-page/internal/checks"
	"service-status-page/internal/store"
)

type ReportNotifier interface {
	NotifyReport(store.Report) error
}

type AvailabilityChecker interface {
	Check(context.Context) []checks.Result
}

type publicStatus struct {
	State            store.StatusState `json:"state"`
	Message          string            `json:"message"`
	UpdatedAt        time.Time         `json:"updatedAt"`
	Source           string            `json:"source"`
	ChecksTotal      int               `json:"checksTotal"`
	ChecksFailed     int               `json:"checksFailed"`
	AnnouncementKind string            `json:"announcementKind,omitempty"`
}

type Server struct {
	store     *store.Store
	notifier  ReportNotifier
	checker   AvailabilityChecker
	spaDir    string
	limiter   *rateLimiter
	startedAt time.Time
	mux       *http.ServeMux
	done      chan struct{}
	doneOnce  sync.Once
}

func New(st *store.Store, notifier ReportNotifier, checker AvailabilityChecker, spaDir string) *Server {
	s := &Server{
		store:     st,
		notifier:  notifier,
		checker:   checker,
		spaDir:    spaDir,
		limiter:   newRateLimiter(5, 10*time.Minute),
		startedAt: time.Now().UTC(),
		done:      make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/status/events", s.handleStatusEvents)
	mux.HandleFunc("GET /api/checks", s.handleChecks)
	mux.HandleFunc("POST /api/reports", s.handleCreateReport)
	mux.HandleFunc("/", s.handleSPA)
	s.mux = mux
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) Shutdown() {
	s.doneOnce.Do(func() {
		close(s.done)
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.statusResponse(r.Context()))
}

func (s *Server) handleChecks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.checksResponse(r.Context()))
}

func (s *Server) handleStatusEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	updates, unsubscribe := s.store.Subscribe()
	defer unsubscribe()

	if err := writeStatusEvent(w, s.statusResponse(r.Context())); err != nil {
		return
	}
	flusher.Flush()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		case <-updates:
			if err := writeStatusEvent(w, s.statusResponse(r.Context())); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.limiter.Allow(ip, time.Now()) {
		writeError(w, http.StatusTooManyRequests, "Слишком много сообщений. Попробуйте позже.")
		return
	}

	var input struct {
		Message string `json:"message"`
		Name    string `json:"name"`
		Contact string `json:"contact"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Некорректный JSON")
		return
	}

	input.Message = strings.TrimSpace(input.Message)
	input.Name = strings.TrimSpace(input.Name)
	input.Contact = strings.TrimSpace(input.Contact)
	if input.Message == "" {
		writeError(w, http.StatusBadRequest, "Опишите проблему")
		return
	}
	if isReservedReportName(input.Name) {
		writeError(w, http.StatusBadRequest, "Выберите другое имя")
		return
	}

	report, err := s.store.AddReport(store.Report{
		Message:   input.Message,
		Name:      input.Name,
		Contact:   input.Contact,
		IPHash:    hashIP(ip),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось сохранить сообщение")
		return
	}

	if s.notifier != nil {
		if err := s.notifier.NotifyReport(report); err == nil {
			_ = s.store.MarkReportSent(report.ID)
			report.SentToTelegram = true
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"report": report,
	})
}

func isReservedReportName(name string) bool {
	for _, reserved := range []string{
		"admin",
		"админ",
		"administrator",
		"администратор",
		"moderator",
		"модератор",
		"support",
		"поддержка",
		"служба поддержки",
	} {
		if strings.EqualFold(name, reserved) {
			return true
		}
	}
	return false
}

func (s *Server) statusResponse(ctx context.Context) map[string]any {
	snapshot := s.store.Snapshot()
	targets := s.checkTargets(ctx)
	generatedAt := time.Now().UTC()
	return map[string]any{
		"status":             s.publicStatus(snapshot, targets, generatedAt),
		"pinnedInfo":         snapshot.PinnedInfo,
		"announcements":      snapshot.Announcements,
		"activeAnnouncement": activeAnnouncement(snapshot.Announcements),
		"checks": map[string]any{
			"targets": targets,
			"meta": map[string]any{
				"generatedAt": generatedAt,
			},
		},
		"meta": map[string]any{
			"startedAt":   s.startedAt,
			"generatedAt": generatedAt,
		},
	}
}

func (s *Server) checksResponse(ctx context.Context) map[string]any {
	return map[string]any{
		"targets": s.checkTargets(ctx),
		"meta": map[string]any{
			"generatedAt": time.Now().UTC(),
		},
	}
}

func (s *Server) checkTargets(ctx context.Context) []checks.Result {
	if s.checker == nil {
		return []checks.Result{}
	}
	return s.checker.Check(ctx)
}

func (s *Server) publicStatus(snapshot store.State, targets []checks.Result, now time.Time) publicStatus {
	if len(targets) > 0 {
		failed := failedCheckCount(targets)
		if failed > 0 {
			return publicStatus{
				State:        store.StatusIncident,
				Message:      fmt.Sprintf("Есть проблема: недоступно %d из %d проверяемых адресов", failed, len(targets)),
				UpdatedAt:    latestCheckTime(targets, now),
				Source:       "checks",
				ChecksTotal:  len(targets),
				ChecksFailed: failed,
			}
		}
		return publicStatus{
			State:        store.StatusOK,
			Message:      "Сервис работает штатно",
			UpdatedAt:    latestCheckTime(targets, now),
			Source:       "checks",
			ChecksTotal:  len(targets),
			ChecksFailed: 0,
		}
	}

	if ann, ok := latestAdminAnnouncement(snapshot.Announcements); ok {
		return publicStatus{
			State:            store.StatusOK,
			Message:          ann.Message,
			UpdatedAt:        ann.CreatedAt,
			Source:           "announcement",
			AnnouncementKind: string(ann.Kind),
		}
	}

	return publicStatus{
		State:     store.StatusOK,
		Message:   "Сервис работает штатно",
		UpdatedAt: snapshot.Status.UpdatedAt,
		Source:    "default",
	}
}

func failedCheckCount(targets []checks.Result) int {
	failed := 0
	for _, target := range targets {
		if target.State != checks.StateUp {
			failed++
		}
	}
	return failed
}

func latestCheckTime(targets []checks.Result, fallback time.Time) time.Time {
	latest := time.Time{}
	for _, target := range targets {
		if target.CheckedAt.After(latest) {
			latest = target.CheckedAt
		}
	}
	if latest.IsZero() {
		return fallback
	}
	return latest
}

func latestAdminAnnouncement(announcements []store.Announcement) (store.Announcement, bool) {
	for _, ann := range announcements {
		if ann.Kind == store.AnnouncementUser {
			continue
		}
		if ann.Kind == store.AnnouncementCleared || ann.Kind == store.AnnouncementResolved {
			return store.Announcement{}, false
		}
		if ann.Kind == store.AnnouncementInfo || ann.Kind == store.AnnouncementMaintenance || ann.Kind == store.AnnouncementIncident {
			return ann, true
		}
	}
	return store.Announcement{}, false
}

func activeAnnouncement(announcements []store.Announcement) any {
	ann, ok := latestAdminAnnouncement(announcements)
	if !ok {
		return nil
	}
	return ann
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "Not found")
		return
	}

	cleanPath := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if cleanPath == "." {
		cleanPath = "index.html"
	}

	fullPath := filepath.Join(s.spaDir, cleanPath)
	if !strings.HasPrefix(fullPath, filepath.Clean(s.spaDir)+string(os.PathSeparator)) && filepath.Clean(fullPath) != filepath.Clean(s.spaDir) {
		writeError(w, http.StatusBadRequest, "Bad path")
		return
	}

	if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, fullPath)
		return
	}

	indexPath := filepath.Join(s.spaDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		writeError(w, http.StatusServiceUnavailable, "Frontend build is missing. Run npm run build in web/.")
		return
	}
	http.ServeFile(w, r, indexPath)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeStatusEvent(w http.ResponseWriter, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: status\ndata: %s\n\n", data)
	return err
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func clientIP(r *http.Request) string {
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			value = strings.TrimSpace(strings.Split(value, ",")[0])
		}
		if value != "" {
			return value
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func hashIP(ip string) string {
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:])
}

func reportSummary(report store.Report) string {
	parts := []string{fmt.Sprintf("Баг-репорт #%s", report.ID), report.Message}
	if report.Name != "" {
		parts = append(parts, "Имя: "+report.Name)
	}
	if report.Contact != "" {
		parts = append(parts, "Контакт: "+report.Contact)
	}
	return strings.Join(parts, "\n\n")
}
