package cmd

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestParseStatusExpiration(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	command.Flags().Duration("expires-in", 0, "")
	command.Flags().String("expires-at", "", "")
	if err := command.Flags().Set("expires-at", "1893456000"); err != nil {
		t.Fatal(err)
	}
	got, err := parseStatusExpiration(command)
	if err != nil || got != 1893456000 {
		t.Fatalf("got %d err=%v", got, err)
	}

	command = &cobra.Command{Use: "test"}
	command.Flags().Duration("expires-in", 0, "")
	command.Flags().String("expires-at", "", "")
	if err := command.Flags().Set("expires-at", "2030-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	got, err = parseStatusExpiration(command)
	want := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	if err != nil || got != want {
		t.Fatalf("got %d want %d err=%v", got, want, err)
	}
}

func TestSplitNonEmpty(t *testing.T) {
	got := splitNonEmpty(" public_channel, ,im ")
	if len(got) != 2 || got[0] != "public_channel" || got[1] != "im" {
		t.Fatalf("unexpected split: %#v", got)
	}
}
