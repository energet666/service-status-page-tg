package bot

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"service-status-page/internal/checks"
	"service-status-page/internal/store"
)

func TestIsAdmin(t *testing.T) {
	admins := map[int64]struct{}{1: {}}
	if !IsAdmin(1, admins) {
		t.Fatal("admin was rejected")
	}
	if IsAdmin(2, admins) {
		t.Fatal("non-admin was accepted")
	}
}

func TestParseStatusMessage(t *testing.T) {
	message, err := ParseStatusMessage("Работы с 13:00 до 14:00", "/maintenance")
	if err != nil {
		t.Fatal(err)
	}
	if message != "Работы с 13:00 до 14:00" {
		t.Fatalf("message = %q", message)
	}
}

func TestParseStatusMessageUsesDefaultText(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		defaultMessage string
	}{
		{
			name:           "maintenance",
			command:        "/maintenance",
			defaultMessage: defaultMaintenanceStatusMessage,
		},
		{
			name:           "incident",
			command:        "/incident",
			defaultMessage: defaultIncidentStatusMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, err := ParseStatusMessage(" ", tt.command, tt.defaultMessage)
			if err != nil {
				t.Fatal(err)
			}
			if message != tt.defaultMessage {
				t.Fatalf("message = %q, want %q", message, tt.defaultMessage)
			}
		})
	}
}

func TestPublishStatusAnnouncementDoesNotChangeStoredStatus(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	b := &Bot{store: st}

	ann, err := b.publishStatusAnnouncement(store.AnnouncementMaintenance, "Работы с 13:00", "admin")
	if err != nil {
		t.Fatal(err)
	}

	snap := st.Snapshot()
	if snap.Status.State != store.StatusOK {
		t.Fatalf("status = %q, want %q", snap.Status.State, store.StatusOK)
	}
	if len(snap.Announcements) != 1 {
		t.Fatalf("announcements = %d, want 1", len(snap.Announcements))
	}
	if snap.Announcements[0].ID != ann.ID {
		t.Fatalf("announcement ID = %q, want %q", snap.Announcements[0].ID, ann.ID)
	}
	if snap.Announcements[0].Kind != store.AnnouncementMaintenance {
		t.Fatalf("announcement kind = %q, want %q", snap.Announcements[0].Kind, store.AnnouncementMaintenance)
	}
}

func TestHelpTextIncludesPinnedInfoCommands(t *testing.T) {
	text := helpText()
	for _, want := range []string{
		"/info текст постоянного блока",
		"/clearinfo",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("help text %q does not contain %q", text, want)
		}
	}
}

func TestFormatAvailabilityProblems(t *testing.T) {
	text := formatAvailabilityProblems([]checks.Result{
		{
			Name:       "Example",
			URL:        "https://example.com/",
			State:      checks.StateHTTPError,
			StatusCode: 503,
			Error:      "Service Unavailable",
			LatencyMs:  120,
			CheckedAt:  time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC),
		},
		{
			Name:  "OK",
			URL:   "https://ok.example/",
			State: checks.StateUp,
		},
	})

	for _, want := range []string{
		"Проблемы с доступностью сайтов",
		"Example",
		"https://example.com/",
		"Состояние: HTTP-ошибка",
		"HTTP: 503",
		"Ошибка: Service Unavailable",
		"Задержка: 120 мс",
		"Проверено: 19.04 10:00 UTC",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("message %q does not contain %q", text, want)
		}
	}
	if strings.Contains(text, "OK") {
		t.Fatalf("message %q contains healthy target", text)
	}
}

func TestFormatAvailabilityRecovered(t *testing.T) {
	text := formatAvailabilityRecovered([]checks.Result{
		{
			Name:      "Example",
			URL:       "https://example.com/",
			State:     checks.StateUp,
			CheckedAt: time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC),
		},
		{
			Name:      "OK",
			URL:       "https://ok.example/",
			State:     checks.StateUp,
			CheckedAt: time.Date(2026, 4, 19, 10, 1, 0, 0, time.UTC),
		},
	})

	for _, want := range []string{
		"Доступность сайтов восстановлена",
		"Все проверки успешны: 2",
		"Проверено: 19.04 10:01 UTC",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("message %q does not contain %q", text, want)
		}
	}
}
