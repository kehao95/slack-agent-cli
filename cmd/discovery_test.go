package cmd

import (
	"testing"

	usersops "github.com/kehao95/slack-agent-cli/internal/users"
)

func TestUserMatchesQueryUsernamePrefixIsExact(t *testing.T) {
	user := usersops.UserInfo{
		UserID:   "U123",
		Username: "@alice",
		Name:     "alice",
		Email:    "alice@example.com",
	}

	if !userMatchesQuery(user, "@alice") {
		t.Fatal("@alice should match the exact username")
	}
	if userMatchesQuery(user, "@ali") {
		t.Fatal("@ali must not perform partial username matching")
	}
	if userMatchesQuery(user, "@alice@example.com") {
		t.Fatal("@alice@example.com must not be treated as an email query")
	}
}

func TestUserMatchesQueryEmailIsExact(t *testing.T) {
	user := usersops.UserInfo{UserID: "U123", Username: "@alice", Email: "alice@example.com"}
	if !userMatchesQuery(user, "alice@example.com") {
		t.Fatal("email query should match exactly")
	}
	if userMatchesQuery(user, "alice@example") {
		t.Fatal("partial email query must not match")
	}
}
