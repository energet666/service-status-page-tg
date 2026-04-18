package store

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesDefaultState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	if snap.Status.State != StatusOK {
		t.Fatalf("status = %q, want %q", snap.Status.State, StatusOK)
	}
	if snap.Status.Message == "" {
		t.Fatal("default status message is empty")
	}
}

func TestStorePersistsUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetStatus(StatusMaintenance, "Работы с 13:00 до 14:00", "test"); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	snap := reopened.Snapshot()
	if snap.Status.State != StatusMaintenance {
		t.Fatalf("status = %q, want %q", snap.Status.State, StatusMaintenance)
	}
	if len(snap.Announcements) != 1 {
		t.Fatalf("announcements = %d, want 1", len(snap.Announcements))
	}
}

func TestStoreLimitsHistory(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < MaxItems+5; i++ {
		if _, err := st.AddAnnouncement("message", AnnouncementInfo, "test"); err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddReport(Report{Message: "bug"}); err != nil {
			t.Fatal(err)
		}
	}

	snap := st.Snapshot()
	if len(snap.Announcements) != MaxItems {
		t.Fatalf("announcements = %d, want %d", len(snap.Announcements), MaxItems)
	}
	if len(snap.Reports) != MaxItems {
		t.Fatalf("reports = %d, want %d", len(snap.Reports), MaxItems)
	}
}

func TestAddReportAddsUserAnnouncement(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	report, err := st.AddReport(Report{
		Message: "Не открывается страница оплаты",
		Name:    "Анна",
		Contact: "@anna",
	})
	if err != nil {
		t.Fatal(err)
	}

	snap := st.Snapshot()
	if len(snap.Announcements) != 1 {
		t.Fatalf("announcements = %d, want 1", len(snap.Announcements))
	}
	ann := snap.Announcements[0]
	if ann.ID != report.ID {
		t.Fatalf("announcement ID = %q, want report ID %q", ann.ID, report.ID)
	}
	if ann.Kind != AnnouncementUser {
		t.Fatalf("announcement kind = %q, want %q", ann.Kind, AnnouncementUser)
	}
	if ann.Message != report.Message {
		t.Fatalf("announcement message = %q, want %q", ann.Message, report.Message)
	}
	if ann.CreatedBy != report.Name {
		t.Fatalf("announcement createdBy = %q, want %q", ann.CreatedBy, report.Name)
	}
}

func TestSubscribeReceivesUpdates(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	updates, unsubscribe := st.Subscribe()
	defer unsubscribe()

	if _, err := st.SetStatus(StatusIncident, "Проверочный инцидент", "test"); err != nil {
		t.Fatal(err)
	}

	select {
	case <-updates:
	default:
		t.Fatal("subscriber did not receive update")
	}
}
