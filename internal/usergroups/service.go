package usergroups

import (
	"context"
	"fmt"
	"strings"

	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
)

type OperationsClient interface {
	ListUserGroups(context.Context, appslack.UserGroupListOptions) ([]slackapi.UserGroup, error)
	GetUserGroupMembers(context.Context, string) ([]string, error)
	CreateUserGroup(context.Context, slackapi.UserGroup, bool, bool) (*slackapi.UserGroup, error)
	UpdateUserGroup(context.Context, string, appslack.UserGroupUpdateOptions) (*slackapi.UserGroup, error)
	SetUserGroupEnabled(context.Context, string, bool, bool, string) (*slackapi.UserGroup, error)
	UpdateUserGroupMembers(context.Context, string, []string) (*slackapi.UserGroup, error)
}

type Service struct{ client OperationsClient }

func NewService(client OperationsClient) *Service { return &Service{client: client} }

type ListResult struct {
	OK         bool                 `json:"ok"`
	UserGroups []slackapi.UserGroup `json:"usergroups"`
}

func (s *Service) List(ctx context.Context, opts appslack.UserGroupListOptions) (*ListResult, error) {
	groups, err := s.client.ListUserGroups(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &ListResult{OK: true, UserGroups: groups}, nil
}

type MembersResult struct {
	OK        bool     `json:"ok"`
	UserGroup string   `json:"usergroup"`
	Members   []string `json:"members"`
}

func (s *Service) Members(ctx context.Context, ref string) (*MembersResult, error) {
	id, err := s.ResolveID(ctx, ref)
	if err != nil {
		return nil, err
	}
	members, err := s.client.GetUserGroupMembers(ctx, id)
	if err != nil {
		return nil, err
	}
	return &MembersResult{OK: true, UserGroup: id, Members: members}, nil
}

type MutationResult struct {
	OK        bool               `json:"ok"`
	Action    string             `json:"action"`
	UserGroup slackapi.UserGroup `json:"usergroup"`
}

func (s *Service) Create(ctx context.Context, group slackapi.UserGroup, includeCount, enableSection bool) (*MutationResult, error) {
	if strings.TrimSpace(group.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	created, err := s.client.CreateUserGroup(ctx, group, includeCount, enableSection)
	if err != nil {
		return nil, err
	}
	return &MutationResult{OK: true, Action: "create", UserGroup: *created}, nil
}
func (s *Service) Update(ctx context.Context, ref string, opts appslack.UserGroupUpdateOptions) (*MutationResult, error) {
	id, err := s.ResolveID(ctx, ref)
	if err != nil {
		return nil, err
	}
	updated, err := s.client.UpdateUserGroup(ctx, id, opts)
	if err != nil {
		return nil, err
	}
	return &MutationResult{OK: true, Action: "update", UserGroup: *updated}, nil
}
func (s *Service) SetEnabled(ctx context.Context, ref string, enabled, includeCount bool, teamID string) (*MutationResult, error) {
	id, err := s.ResolveID(ctx, ref)
	if err != nil {
		return nil, err
	}
	group, err := s.client.SetUserGroupEnabled(ctx, id, enabled, includeCount, teamID)
	if err != nil {
		return nil, err
	}
	action := "disable"
	if enabled {
		action = "enable"
	}
	return &MutationResult{OK: true, Action: action, UserGroup: *group}, nil
}
func (s *Service) SetMembers(ctx context.Context, ref string, members []string) (*MutationResult, error) {
	id, err := s.ResolveID(ctx, ref)
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("at least one member is required")
	}
	group, err := s.client.UpdateUserGroupMembers(ctx, id, members)
	if err != nil {
		return nil, err
	}
	return &MutationResult{OK: true, Action: "update-members", UserGroup: *group}, nil
}

// AddMembers appends users to the current membership while preserving the
// existing order and avoiding duplicate IDs.
func (s *Service) AddMembers(ctx context.Context, ref string, members []string) (*MutationResult, error) {
	id, err := s.ResolveID(ctx, ref)
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("at least one member is required")
	}
	current, err := s.client.GetUserGroupMembers(ctx, id)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(current)+len(members))
	merged := make([]string, 0, len(current)+len(members))
	for _, member := range append(current, members...) {
		member = strings.TrimSpace(member)
		if member == "" {
			continue
		}
		if _, ok := seen[member]; ok {
			continue
		}
		seen[member] = struct{}{}
		merged = append(merged, member)
	}
	group, err := s.client.UpdateUserGroupMembers(ctx, id, merged)
	if err != nil {
		return nil, err
	}
	return &MutationResult{OK: true, Action: "add-members", UserGroup: *group}, nil
}

// RemoveMembers removes the requested users from the current membership. A
// group may become empty, so this intentionally does not use SetMembers.
func (s *Service) RemoveMembers(ctx context.Context, ref string, members []string) (*MutationResult, error) {
	id, err := s.ResolveID(ctx, ref)
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("at least one member is required")
	}
	remove := make(map[string]struct{}, len(members))
	for _, member := range members {
		member = strings.TrimSpace(member)
		if member != "" {
			remove[member] = struct{}{}
		}
	}
	if len(remove) == 0 {
		return nil, fmt.Errorf("at least one member is required")
	}
	current, err := s.client.GetUserGroupMembers(ctx, id)
	if err != nil {
		return nil, err
	}
	remaining := make([]string, 0, len(current))
	for _, member := range current {
		if _, ok := remove[strings.TrimSpace(member)]; !ok {
			remaining = append(remaining, member)
		}
	}
	if len(remaining) == 0 {
		return nil, fmt.Errorf("Slack does not allow removing all user group members; disable the group instead")
	}
	group, err := s.client.UpdateUserGroupMembers(ctx, id, remaining)
	if err != nil {
		return nil, err
	}
	return &MutationResult{OK: true, Action: "remove-members", UserGroup: *group}, nil
}

func (s *Service) ResolveID(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("user group is required")
	}
	if strings.HasPrefix(ref, "S") && strings.ToUpper(ref) == ref {
		return ref, nil
	}
	handle := strings.TrimPrefix(ref, "@")
	groups, err := s.client.ListUserGroups(ctx, appslack.UserGroupListOptions{IncludeDisabled: true})
	if err != nil {
		return "", err
	}
	for _, group := range groups {
		if strings.EqualFold(group.Handle, handle) || strings.EqualFold(group.Name, ref) {
			return group.ID, nil
		}
	}
	return "", fmt.Errorf("user group not found: %s", ref)
}

func (r *ListResult) Lines() []string {
	if len(r.UserGroups) == 0 {
		return []string{"No user groups found."}
	}
	lines := []string{fmt.Sprintf("User Groups (%d)", len(r.UserGroups))}
	for _, group := range r.UserGroups {
		lines = append(lines, fmt.Sprintf("%s  @%s  %s (%d members)", group.ID, group.Handle, group.Name, group.UserCount))
	}
	return lines
}
func (r *MembersResult) Lines() []string {
	lines := []string{fmt.Sprintf("Members of %s (%d)", r.UserGroup, len(r.Members))}
	lines = append(lines, r.Members...)
	return lines
}
func (r *MutationResult) Lines() []string {
	return []string{fmt.Sprintf("User group %s: @%s (%s)", r.Action, r.UserGroup.Handle, r.UserGroup.ID)}
}
