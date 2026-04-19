package store

import (
	"errors"
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

func TestDeleteLatestAnnouncementRollsBackStatus(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetStatus(StatusMaintenance, "Работы с 13:00", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetStatus(StatusIncident, "Платежи недоступны", "test"); err != nil {
		t.Fatal(err)
	}

	ann, statusChanged, err := st.DeleteLatestAnnouncement()
	if err != nil {
		t.Fatal(err)
	}
	if !statusChanged {
		t.Fatal("statusChanged = false, want true")
	}
	if ann.Message != "Платежи недоступны" {
		t.Fatalf("deleted announcement message = %q", ann.Message)
	}

	snap := st.Snapshot()
	if snap.Status.State != StatusMaintenance {
		t.Fatalf("status = %q, want %q", snap.Status.State, StatusMaintenance)
	}
	if snap.Status.Message != "Работы с 13:00" {
		t.Fatalf("status message = %q", snap.Status.Message)
	}
	if len(snap.Announcements) != 1 {
		t.Fatalf("announcements = %d, want 1", len(snap.Announcements))
	}
}

func TestDeleteLatestAnnouncementKeepsStatusForPlainAnnouncement(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetStatus(StatusIncident, "Платежи недоступны", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAnnouncement("Администратор проверяет очередь", AnnouncementInfo, "test"); err != nil {
		t.Fatal(err)
	}

	_, statusChanged, err := st.DeleteLatestAnnouncement()
	if err != nil {
		t.Fatal(err)
	}
	if statusChanged {
		t.Fatal("statusChanged = true, want false")
	}

	snap := st.Snapshot()
	if snap.Status.State != StatusIncident {
		t.Fatalf("status = %q, want %q", snap.Status.State, StatusIncident)
	}
	if snap.Status.Message != "Платежи недоступны" {
		t.Fatalf("status message = %q", snap.Status.Message)
	}
	if len(snap.Announcements) != 1 {
		t.Fatalf("announcements = %d, want 1", len(snap.Announcements))
	}
}

func TestDeleteLatestAnnouncementFallsBackToDefaultStatus(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetStatus(StatusIncident, "Платежи недоступны", "test"); err != nil {
		t.Fatal(err)
	}

	_, statusChanged, err := st.DeleteLatestAnnouncement()
	if err != nil {
		t.Fatal(err)
	}
	if !statusChanged {
		t.Fatal("statusChanged = false, want true")
	}

	snap := st.Snapshot()
	if snap.Status.State != StatusOK {
		t.Fatalf("status = %q, want %q", snap.Status.State, StatusOK)
	}
	if len(snap.Announcements) != 0 {
		t.Fatalf("announcements = %d, want 0", len(snap.Announcements))
	}
}

func TestDeleteLatestAnnouncementRequiresAnnouncement(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = st.DeleteLatestAnnouncement()
	if !errors.Is(err, ErrNoAnnouncements) {
		t.Fatalf("err = %v, want %v", err, ErrNoAnnouncements)
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
