package cmd

import "testing"

func TestMessagesListIncludeMetadataFlagDefaultsOff(t *testing.T) {
	flag := messagesListCmd.Flags().Lookup("include-metadata")
	if flag == nil {
		t.Fatal("messages list is missing --include-metadata")
	}
	if flag.DefValue != "false" {
		t.Fatalf("--include-metadata default = %q, want false", flag.DefValue)
	}
}
