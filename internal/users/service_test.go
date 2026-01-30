package users

import (
	"context"
	"errors"
	"testing"

	slackapi "github.com/slack-go/slack"
)

// Note: mockUserClient is already defined in resolver_test.go

func TestService_List(t *testing.T) {
	tests := []struct {
		name        string
		params      ListParams
		mockUsers   []slackapi.User
		mockErr     error
		wantErr     bool
		wantCount   int
		includeBots bool
	}{
		{
			name: "list users without bots",
			params: ListParams{
				Limit:       100,
				IncludeBots: false,
			},
			mockUsers: []slackapi.User{
				{ID: "U1", Name: "alice", RealName: "Alice Smith", IsBot: false},
				{ID: "U2", Name: "botuser", RealName: "Bot User", IsBot: true},
				{ID: "U3", Name: "bob", RealName: "Bob Jones", IsBot: false},
			},
			mockErr:     nil,
			wantErr:     false,
			wantCount:   2, // Only non-bot users
			includeBots: false,
		},
		{
			name: "list users with bots",
			params: ListParams{
				Limit:       100,
				IncludeBots: true,
			},
			mockUsers: []slackapi.User{
				{ID: "U1", Name: "alice", RealName: "Alice Smith", IsBot: false},
				{ID: "U2", Name: "botuser", RealName: "Bot User", IsBot: true},
				{ID: "U3", Name: "bob", RealName: "Bob Jones", IsBot: false},
			},
			mockErr:     nil,
			wantErr:     false,
			wantCount:   3, // All users
			includeBots: true,
		},
		{
			name: "list with pagination",
			params: ListParams{
				Limit:       50,
				Cursor:      "cursor123",
				IncludeBots: false,
			},
			mockUsers: []slackapi.User{
				{ID: "U4", Name: "charlie", RealName: "Charlie Brown", IsBot: false},
			},
			mockErr:     nil,
			wantErr:     false,
			wantCount:   1,
			includeBots: false,
		},
		{
			name: "api error",
			params: ListParams{
				Limit:       100,
				IncludeBots: false,
			},
			mockUsers:   nil,
			mockErr:     errors.New("slack api error"),
			wantErr:     true,
			wantCount:   0,
			includeBots: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserClient{
				allUsers:     tt.mockUsers,
				listUsersErr: tt.mockErr,
			}

			service := NewService(mock)
			result, err := service.List(context.Background(), tt.params)

			if (err != nil) != tt.wantErr {
				t.Errorf("List() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error case
			}

			if len(result.Users) != tt.wantCount {
				t.Errorf("List() got %d users, want %d", len(result.Users), tt.wantCount)
			}

			// Note: mock always returns empty cursor
			if !result.OK {
				t.Errorf("List() OK = false, want true")
			}
		})
	}
}

func TestService_GetInfo(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		mockUser  *slackapi.User
		mockErr   error
		wantErr   bool
		wantID    string
		wantName  string
		wantEmail string
	}{
		{
			name:   "get user info success",
			userID: "U123",
			mockUser: &slackapi.User{
				ID:       "U123",
				Name:     "alice",
				RealName: "Alice Smith",
				Profile: slackapi.UserProfile{
					Email:       "alice@example.com",
					DisplayName: "alice.smith",
					Title:       "Engineer",
				},
			},
			mockErr:   nil,
			wantErr:   false,
			wantID:    "U123",
			wantName:  "alice",
			wantEmail: "alice@example.com",
		},
		{
			name:     "api error",
			userID:   "U999",
			mockUser: nil,
			mockErr:  errors.New("user not found"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserClient{
				singleUser: tt.mockUser,
				err:        tt.mockErr,
			}

			service := NewService(mock)
			result, err := service.GetInfo(context.Background(), tt.userID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error case
			}

			if result.User.ID != tt.wantID {
				t.Errorf("GetInfo() ID = %v, want %v", result.User.ID, tt.wantID)
			}

			if result.User.Name != tt.wantName {
				t.Errorf("GetInfo() Name = %v, want %v", result.User.Name, tt.wantName)
			}

			if result.User.Email != tt.wantEmail {
				t.Errorf("GetInfo() Email = %v, want %v", result.User.Email, tt.wantEmail)
			}

			if !result.OK {
				t.Errorf("GetInfo() OK = false, want true")
			}
		})
	}
}

func TestListResult_Lines(t *testing.T) {
	tests := []struct {
		name      string
		result    *ListResult
		wantLines int
		wantEmpty bool
	}{
		{
			name: "multiple users",
			result: &ListResult{
				OK: true,
				Users: []UserInfo{
					{ID: "U1", Name: "alice", DisplayName: "Alice Smith", IsBot: false},
					{ID: "U2", Name: "bot", DisplayName: "Bot", IsBot: true},
				},
			},
			wantLines: 4, // title, separator, 2 users
			wantEmpty: false,
		},
		{
			name: "empty users",
			result: &ListResult{
				OK:    true,
				Users: []UserInfo{},
			},
			wantLines: 1, // "No users found."
			wantEmpty: true,
		},
		{
			name: "with cursor",
			result: &ListResult{
				OK: true,
				Users: []UserInfo{
					{ID: "U1", Name: "alice", DisplayName: "Alice Smith"},
				},
				NextCursor: "cursor123",
			},
			wantLines: 5, // title, separator, 1 user, blank line, cursor
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := tt.result.Lines()

			if len(lines) != tt.wantLines {
				t.Errorf("Lines() got %d lines, want %d", len(lines), tt.wantLines)
			}

			if tt.wantEmpty && lines[0] != "No users found." {
				t.Errorf("Lines() expected 'No users found.' for empty result")
			}
		})
	}
}

func TestUserInfoResult_Lines(t *testing.T) {
	result := &UserInfoResult{
		OK: true,
		User: UserInfo{
			ID:          "U123",
			Name:        "alice",
			DisplayName: "Alice Smith",
			Email:       "alice@example.com",
			Title:       "Engineer",
			IsBot:       false,
			IsDeleted:   false,
		},
	}

	lines := result.Lines()

	if len(lines) < 5 {
		t.Errorf("Lines() got %d lines, want at least 5", len(lines))
	}

	// Check that key fields are present
	found := make(map[string]bool)
	for _, line := range lines {
		if line == "ID: U123" {
			found["id"] = true
		}
		if line == "Email: alice@example.com" {
			found["email"] = true
		}
		if line == "Title: Engineer" {
			found["title"] = true
		}
		if line == "Status: Active" {
			found["status"] = true
		}
	}

	for field, ok := range found {
		if !ok {
			t.Errorf("Lines() missing expected field: %s", field)
		}
	}
}

func TestService_GetPresence(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		mockPresence *slackapi.UserPresence
		mockErr      error
		wantErr      bool
		wantPresence string
		wantOnline   bool
	}{
		{
			name:   "active user",
			userID: "U123",
			mockPresence: &slackapi.UserPresence{
				Presence:        "active",
				Online:          true,
				AutoAway:        false,
				ManualAway:      false,
				ConnectionCount: 2,
				LastActivity:    slackapi.JSONTime(1234567890),
			},
			mockErr:      nil,
			wantErr:      false,
			wantPresence: "active",
			wantOnline:   true,
		},
		{
			name:   "away user",
			userID: "U456",
			mockPresence: &slackapi.UserPresence{
				Presence:        "away",
				Online:          false,
				AutoAway:        true,
				ManualAway:      false,
				ConnectionCount: 0,
				LastActivity:    slackapi.JSONTime(1234567890),
			},
			mockErr:      nil,
			wantErr:      false,
			wantPresence: "away",
			wantOnline:   false,
		},
		{
			name:         "api error",
			userID:       "U999",
			mockPresence: nil,
			mockErr:      errors.New("presence api error"),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockUserClient{
				presence:    tt.mockPresence,
				presenceErr: tt.mockErr,
			}

			service := NewService(mock)
			result, err := service.GetPresence(context.Background(), tt.userID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetPresence() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error case
			}

			if result.Presence != tt.wantPresence {
				t.Errorf("GetPresence() presence = %v, want %v", result.Presence, tt.wantPresence)
			}

			if result.Online != tt.wantOnline {
				t.Errorf("GetPresence() online = %v, want %v", result.Online, tt.wantOnline)
			}

			if !result.OK {
				t.Errorf("GetPresence() OK = false, want true")
			}
		})
	}
}

func TestPresenceResult_Lines(t *testing.T) {
	tests := []struct {
		name       string
		result     *PresenceResult
		wantStatus string
		wantOnline string
	}{
		{
			name: "active user",
			result: &PresenceResult{
				OK:              true,
				Presence:        "active",
				Online:          true,
				AutoAway:        false,
				ManualAway:      false,
				ConnectionCount: 2,
				LastActivity:    1234567890,
			},
			wantStatus: "Status: 🟢 Active",
			wantOnline: "Online: Yes",
		},
		{
			name: "away user",
			result: &PresenceResult{
				OK:              true,
				Presence:        "away",
				Online:          false,
				AutoAway:        true,
				ManualAway:      false,
				ConnectionCount: 0,
				LastActivity:    1234567890,
			},
			wantStatus: "Status: ⚪ Away",
			wantOnline: "Online: No",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := tt.result.Lines()

			if len(lines) < 3 {
				t.Errorf("Lines() got %d lines, want at least 3", len(lines))
			}

			foundStatus := false
			foundOnline := false
			for _, line := range lines {
				if line == tt.wantStatus {
					foundStatus = true
				}
				if line == tt.wantOnline {
					foundOnline = true
				}
			}

			if !foundStatus {
				t.Errorf("Lines() missing expected status: %s", tt.wantStatus)
			}
			if !foundOnline {
				t.Errorf("Lines() missing expected online: %s", tt.wantOnline)
			}
		})
	}
}
