package bot

import (
	"path/filepath"
	"testing"

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
