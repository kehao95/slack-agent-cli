package users

import (
	"context"
	"fmt"
	"strings"
	"time"

	slackapi "github.com/slack-go/slack"
)

// Service provides user-related operations.
type Service struct {
	client UserClient
}

type operationsClient interface {
	UserClient
	GetUserByEmail(context.Context, string) (*slackapi.User, error)
	GetUserProfile(context.Context, string, bool) (*slackapi.UserProfile, error)
	SetUserStatus(context.Context, string, string, string, int64) error
	ListUserConversations(context.Context, slackapi.GetConversationsForUserParameters) ([]slackapi.Channel, string, error)
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
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	// ID and Name retain their native Slack meanings for compatibility.
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

// PresenceResult contains the result of a user presence lookup.
type PresenceResult struct {
	OK              bool   `json:"ok"`
	Presence        string `json:"presence"`         // "active" or "away"
	Online          bool   `json:"online"`           // true if user is online
	AutoAway        bool   `json:"auto_away"`        // true if away status is automatic
	ManualAway      bool   `json:"manual_away"`      // true if user manually set away
	ConnectionCount int    `json:"connection_count"` // number of active connections
	LastActivity    int64  `json:"last_activity"`    // unix timestamp of last activity
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

// GetPresence fetches the presence status of a specific user.
func (s *Service) GetPresence(ctx context.Context, userID string) (*PresenceResult, error) {
	presence, err := s.client.GetUserPresence(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user presence: %w", err)
	}

	return &PresenceResult{
		OK:              true,
		Presence:        presence.Presence,
		Online:          presence.Online,
		AutoAway:        presence.AutoAway,
		ManualAway:      presence.ManualAway,
		ConnectionCount: presence.ConnectionCount,
		LastActivity:    int64(presence.LastActivity),
	}, nil
}

// LookupByEmail resolves a workspace member without listing the whole workspace.
func (s *Service) LookupByEmail(ctx context.Context, email string) (*UserInfoResult, error) {
	client, err := s.operations()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(email) == "" {
		return nil, fmt.Errorf("email is required")
	}
	user, err := client.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return &UserInfoResult{OK: true, User: toUserInfo(user)}, nil
}

type ProfileResult struct {
	OK      bool                 `json:"ok"`
	UserID  string               `json:"user_id,omitempty"`
	Profile slackapi.UserProfile `json:"profile"`
}

func (s *Service) GetProfile(ctx context.Context, userID string, includeLabels bool) (*ProfileResult, error) {
	client, err := s.operations()
	if err != nil {
		return nil, err
	}
	profile, err := client.GetUserProfile(ctx, userID, includeLabels)
	if err != nil {
		return nil, err
	}
	return &ProfileResult{OK: true, UserID: userID, Profile: *profile}, nil
}

type StatusResult struct {
	OK         bool   `json:"ok"`
	UserID     string `json:"user_id,omitempty"`
	Text       string `json:"text"`
	Emoji      string `json:"emoji"`
	Expiration int    `json:"expiration"`
}

func (s *Service) GetStatus(ctx context.Context, userID string) (*StatusResult, error) {
	profile, err := s.GetProfile(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	return &StatusResult{OK: true, UserID: userID, Text: profile.Profile.StatusText, Emoji: profile.Profile.StatusEmoji, Expiration: profile.Profile.StatusExpiration}, nil
}

func (s *Service) SetStatus(ctx context.Context, userID, text, emoji string, expiration int64) (*StatusResult, error) {
	client, err := s.operations()
	if err != nil {
		return nil, err
	}
	if err := client.SetUserStatus(ctx, userID, text, emoji, expiration); err != nil {
		return nil, err
	}
	return &StatusResult{OK: true, UserID: userID, Text: text, Emoji: emoji, Expiration: int(expiration)}, nil
}

type ConversationsResult struct {
	OK            bool               `json:"ok"`
	UserID        string             `json:"user_id,omitempty"`
	Conversations []slackapi.Channel `json:"conversations"`
	NextCursor    string             `json:"next_cursor,omitempty"`
}

func (s *Service) ListConversations(ctx context.Context, userID string, types []string, limit int, cursor string, excludeArchived bool) (*ConversationsResult, error) {
	client, err := s.operations()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	channels, next, err := client.ListUserConversations(ctx, slackapi.GetConversationsForUserParameters{UserID: userID, Types: types, Limit: limit, Cursor: cursor, ExcludeArchived: excludeArchived})
	if err != nil {
		return nil, err
	}
	return &ConversationsResult{OK: true, UserID: userID, Conversations: channels, NextCursor: next}, nil
}

func (s *Service) operations() (operationsClient, error) {
	client, ok := s.client.(operationsClient)
	if !ok {
		return nil, fmt.Errorf("user operations are not supported by this client")
	}
	return client, nil
}

// Lines implements the output.Printable interface for ListResult.
func (r *ListResult) Lines() []string {
	if len(r.Users) == 0 {
		return []string{"No users found."}
	}

	title := fmt.Sprintf("Workspace Members (%d)", len(r.Users))
	lines := []string{title, strings.Repeat("-", len(title))}

	for _, u := range r.Users {
		name := userHandleOrID(u)
		displayName := u.DisplayName
		if displayName == "" {
			displayName = u.RealName
		}
		if displayName == "" {
			displayName = u.Name
		}

		line := fmt.Sprintf("%s (%s)", name, u.ID)
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
	name := userHandleOrID(u)

	title := fmt.Sprintf("User: %s", name)
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

// Lines implements the output.Printable interface for PresenceResult.
func (r *PresenceResult) Lines() []string {
	title := "User Presence"
	lines := []string{title, strings.Repeat("-", len(title))}

	// Add presence status with icon
	presenceIcon := "🟢"
	if r.Presence == "away" {
		presenceIcon = "⚪"
	}
	lines = append(lines, fmt.Sprintf("Status: %s %s", presenceIcon, strings.Title(r.Presence)))

	if r.Online {
		lines = append(lines, "Online: Yes")
	} else {
		lines = append(lines, "Online: No")
	}

	if r.AutoAway {
		lines = append(lines, "Away Type: Automatic (idle)")
	} else if r.ManualAway {
		lines = append(lines, "Away Type: Manual")
	}

	if r.ConnectionCount > 0 {
		lines = append(lines, fmt.Sprintf("Active Connections: %d", r.ConnectionCount))
	}

	if r.LastActivity > 0 {
		lastActivity := time.Unix(r.LastActivity, 0)
		lines = append(lines, fmt.Sprintf("Last Activity: %s", lastActivity.Format(time.RFC3339)))
	}

	return lines
}

func (r *ProfileResult) Lines() []string {
	user := r.UserID
	if user == "" {
		user = "current"
	}
	return []string{"User Profile", fmt.Sprintf("User: %s", user), fmt.Sprintf("Display name: %s", r.Profile.DisplayName), fmt.Sprintf("Real name: %s", r.Profile.RealName), fmt.Sprintf("Title: %s", r.Profile.Title), fmt.Sprintf("Email: %s", r.Profile.Email)}
}

func (r *StatusResult) Lines() []string {
	user := r.UserID
	if user == "" {
		user = "current"
	}
	return []string{"User Status", fmt.Sprintf("User: %s", user), fmt.Sprintf("Text: %s", r.Text), fmt.Sprintf("Emoji: %s", r.Emoji), fmt.Sprintf("Expiration: %d", r.Expiration)}
}

func (r *ConversationsResult) Lines() []string {
	lines := []string{fmt.Sprintf("User Conversations (%d)", len(r.Conversations))}
	for _, channel := range r.Conversations {
		lines = append(lines, fmt.Sprintf("%s  #%s", channel.ID, channel.Name))
	}
	if r.NextCursor != "" {
		lines = append(lines, "", "Next cursor: "+r.NextCursor)
	}
	return lines
}

// toUserInfo converts a slack-go User to our UserInfo struct.
func toUserInfo(u *slackapi.User) UserInfo {
	username := ""
	if u.Name != "" {
		username = "@" + u.Name
	}
	return UserInfo{
		UserID:      u.ID,
		Username:    username,
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

func userHandleOrID(u UserInfo) string {
	if u.Username != "" {
		return u.Username
	}
	if u.Name != "" {
		return "@" + u.Name
	}
	return u.ID
}
