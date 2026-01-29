package users

import (
	"context"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
)

// Service provides user-related operations.
type Service struct {
	client UserClient
}

// NewService creates a new users service.
func NewService(client UserClient) *Service {
	return &Service{client: client}
}

// ListParams controls user listing behavior.
type ListParams struct {
	Limit       int
	Cursor      string
	IncludeBots bool
}

// ListResult contains the result of a users list operation.
type ListResult struct {
	OK         bool       `json:"ok"`
	Users      []UserInfo `json:"users"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

// UserInfo contains a subset of user information.
type UserInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RealName    string `json:"real_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
	Title       string `json:"title,omitempty"`
	IsBot       bool   `json:"is_bot"`
	IsDeleted   bool   `json:"is_deleted"`
}

// UserInfoResult contains the result of a user info lookup.
type UserInfoResult struct {
	OK   bool     `json:"ok"`
	User UserInfo `json:"user"`
}

// List fetches users with pagination.
func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.Limit <= 0 {
		params.Limit = 100
	}

	users, nextCursor, err := s.client.ListUsers(ctx, params.Cursor, params.Limit)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	// Filter out bots if requested
	var filtered []UserInfo
	for _, u := range users {
		if !params.IncludeBots && u.IsBot {
			continue
		}
		filtered = append(filtered, toUserInfo(&u))
	}

	return &ListResult{
		OK:         true,
		Users:      filtered,
		NextCursor: nextCursor,
	}, nil
}

// GetInfo fetches information for a specific user.
func (s *Service) GetInfo(ctx context.Context, userID string) (*UserInfoResult, error) {
	user, err := s.client.GetUserInfo(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user info: %w", err)
	}

	return &UserInfoResult{
		OK:   true,
		User: toUserInfo(user),
	}, nil
}

// Lines implements the output.Printable interface for ListResult.
func (r *ListResult) Lines() []string {
	if len(r.Users) == 0 {
		return []string{"No users found."}
	}

	title := fmt.Sprintf("Workspace Members (%d)", len(r.Users))
	lines := []string{title, strings.Repeat("-", len(title))}

	for _, u := range r.Users {
		name := u.Name
		if name == "" {
			name = u.ID
		}
		displayName := u.DisplayName
		if displayName == "" {
			displayName = u.RealName
		}
		if displayName == "" {
			displayName = u.Name
		}

		line := fmt.Sprintf("@%s (%s)", name, u.ID)
		if displayName != "" && displayName != name {
			line += fmt.Sprintf(" - %s", displayName)
		}
		if u.IsBot {
			line += " [bot]"
		}
		if u.IsDeleted {
			line += " [deleted]"
		}

		lines = append(lines, line)
	}

	if r.NextCursor != "" {
		lines = append(lines, "", fmt.Sprintf("Next cursor: %s", r.NextCursor))
	}

	return lines
}

// Lines implements the output.Printable interface for UserInfoResult.
func (r *UserInfoResult) Lines() []string {
	u := r.User
	name := u.Name
	if name == "" {
		name = u.ID
	}

	title := fmt.Sprintf("User: @%s", name)
	lines := []string{title, strings.Repeat("-", len(title))}

	lines = append(lines, fmt.Sprintf("ID: %s", u.ID))

	displayName := u.DisplayName
	if displayName == "" {
		displayName = u.RealName
	}
	if displayName != "" {
		lines = append(lines, fmt.Sprintf("Name: %s", displayName))
	}

	if u.Email != "" {
		lines = append(lines, fmt.Sprintf("Email: %s", u.Email))
	}

	if u.Title != "" {
		lines = append(lines, fmt.Sprintf("Title: %s", u.Title))
	}

	if u.IsBot {
		lines = append(lines, "Type: Bot")
	}

	if u.IsDeleted {
		lines = append(lines, "Status: Deleted")
	} else {
		lines = append(lines, "Status: Active")
	}

	return lines
}

// toUserInfo converts a slack-go User to our UserInfo struct.
func toUserInfo(u *slackapi.User) UserInfo {
	return UserInfo{
		ID:          u.ID,
		Name:        u.Name,
		RealName:    u.RealName,
		DisplayName: u.Profile.DisplayName,
		Email:       u.Profile.Email,
		Title:       u.Profile.Title,
		IsBot:       u.IsBot,
		IsDeleted:   u.Deleted,
	}
}
