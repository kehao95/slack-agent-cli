package cmd

import (
	"testing"
	"time"
)

func TestNormalizeScheduleTime(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "unix", input: "1700000060", want: "1700000060"},
		{name: "RFC3339", input: "2023-11-14T22:14:20Z", want: "1700000060"},
		{name: "relative", input: "1m", want: "1700000060"},
		{name: "past", input: "1699999999", wantErr: true},
		{name: "invalid", input: "tomorrow", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeScheduleTime(tt.input, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeScheduleTime() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("normalizeScheduleTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtendedMessageCommandsRegistered(t *testing.T) {
	for _, name := range []string{"permalink", "ephemeral", "schedule", "scheduled", "stream"} {
		if _, _, err := messagesCmd.Find([]string{name}); err != nil {
			t.Fatalf("messages %s not registered: %v", name, err)
		}
	}
}
