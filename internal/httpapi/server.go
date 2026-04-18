package httpapi

import (
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

	"service-status-page/internal/store"
)

type ReportNotifier interface {
	NotifyReport(store.Report) error
}

type Server struct {
	store     *store.Store
	notifier  ReportNotifier
	spaDir    string
	limiter   *rateLimiter
	startedAt time.Time
	mux       *http.ServeMux
	done      chan struct{}
	doneOnce  sync.Once
}

func New(st *store.Store, notifier ReportNotifier, spaDir string) *Server {
	s := &Server{
		store:     st,
		notifier:  notifier,
		spaDir:    spaDir,
		limiter:   newRateLimiter(5, 10*time.Minute),
		startedAt: time.Now().UTC(),
		done:      make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/status/events", s.handleStatusEvents)
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
	writeJSON(w, http.StatusOK, s.statusResponse())
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

	if err := writeStatusEvent(w, s.statusResponse()); err != nil {
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
			if err := writeStatusEvent(w, s.statusResponse()); err != nil {
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

func (s *Server) statusResponse() map[string]any {
	snapshot := s.store.Snapshot()
	return map[string]any{
		"status":        snapshot.Status,
		"announcements": snapshot.Announcements,
		"meta": map[string]any{
			"startedAt":   s.startedAt,
			"generatedAt": time.Now().UTC(),
		},
	}
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
