package bot

import (
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

func TestParseStatusCommand(t *testing.T) {
	state, message, err := ParseStatusCommand("maintenance Работы с 13:00 до 14:00")
	if err != nil {
		t.Fatal(err)
	}
	if state != store.StatusMaintenance {
		t.Fatalf("state = %q, want %q", state, store.StatusMaintenance)
	}
	if message != "Работы с 13:00 до 14:00" {
		t.Fatalf("message = %q", message)
	}
}

func TestParseStatusCommandRejectsUnknownStatus(t *testing.T) {
	if _, _, err := ParseStatusCommand("bad text"); err == nil {
		t.Fatal("expected error")
	}
}
