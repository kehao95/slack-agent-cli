package cmd

import "testing"

func TestConversationCommandsRegistered(t *testing.T) {
	commands := []string{
		"list", "info", "create", "archive", "unarchive", "rename", "topic",
		"purpose", "members", "invite", "kick", "open", "close", "mark", "join", "leave",
	}
	for _, name := range commands {
		if command, _, err := conversationsCmd.Find([]string{name}); err != nil || command == conversationsCmd {
			t.Fatalf("conversations %s not registered: %v", name, err)
		}
	}
}
