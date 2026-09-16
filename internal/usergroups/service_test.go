package usergroups

import (
	"context"
	"testing"

	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
)

type mockOperationsClient struct {
	groups         []slackapi.UserGroup
	members        []string
	updatedMembers []string
}

func (m *mockOperationsClient) ListUserGroups(context.Context, appslack.UserGroupListOptions) ([]slackapi.UserGroup, error) {
	return m.groups, nil
}
func (m *mockOperationsClient) GetUserGroupMembers(context.Context, string) ([]string, error) {
	return m.members, nil
}
func (m *mockOperationsClient) CreateUserGroup(_ context.Context, group slackapi.UserGroup, _, _ bool) (*slackapi.UserGroup, error) {
	group.ID = "S2"
	return &group, nil
}
func (m *mockOperationsClient) UpdateUserGroup(_ context.Context, id string, _ appslack.UserGroupUpdateOptions) (*slackapi.UserGroup, error) {
	return &slackapi.UserGroup{ID: id}, nil
}
func (m *mockOperationsClient) SetUserGroupEnabled(_ context.Context, id string, _ bool, _ bool, _ string) (*slackapi.UserGroup, error) {
	return &slackapi.UserGroup{ID: id}, nil
}
func (m *mockOperationsClient) UpdateUserGroupMembers(_ context.Context, id string, members []string) (*slackapi.UserGroup, error) {
	m.updatedMembers = members
	return &slackapi.UserGroup{ID: id, Users: members}, nil
}

func TestServiceResolvesHandleAndManagesMembers(t *testing.T) {
	mock := &mockOperationsClient{groups: []slackapi.UserGroup{{ID: "S1", Handle: "engineering", Name: "Engineering"}}, members: []string{"U1"}}
	service := NewService(mock)
	result, err := service.Members(context.Background(), "@engineering")
	if err != nil || result.UserGroup != "S1" || len(result.Members) != 1 {
		t.Fatalf("unexpected members: %#v err=%v", result, err)
	}
	updated, err := service.SetMembers(context.Background(), "@engineering", []string{"U1", "U2"})
	if err != nil || updated.UserGroup.ID != "S1" || len(mock.updatedMembers) != 2 {
		t.Fatalf("unexpected update: %#v err=%v", updated, err)
	}
}

func TestServiceRejectsUnknownGroup(t *testing.T) {
	_, err := NewService(&mockOperationsClient{}).ResolveID(context.Background(), "@missing")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestServiceMemberDeltasPreserveExistingMembership(t *testing.T) {
	mock := &mockOperationsClient{
		groups:  []slackapi.UserGroup{{ID: "S1", Handle: "engineering", Name: "Engineering"}},
		members: []string{"U1", "U2"},
	}
	service := NewService(mock)

	if _, err := service.AddMembers(context.Background(), "@engineering", []string{"U2", "U3"}); err != nil {
		t.Fatalf("AddMembers: %v", err)
	}
	if got, want := mock.updatedMembers, []string{"U1", "U2", "U3"}; !sameStrings(got, want) {
		t.Fatalf("added members = %#v, want %#v", got, want)
	}

	mock.members = mock.updatedMembers
	if _, err := service.RemoveMembers(context.Background(), "S1", []string{"U1"}); err != nil {
		t.Fatalf("RemoveMembers: %v", err)
	}
	if got, want := mock.updatedMembers, []string{"U2", "U3"}; !sameStrings(got, want) {
		t.Fatalf("removed members = %#v, want %#v", got, want)
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
