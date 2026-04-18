package bot

import (
	"testing"
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
			name:           "ok",
			command:        "/ok",
			defaultMessage: defaultOKStatusMessage,
		},
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
