package slack

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	slackapi "github.com/slack-go/slack"
)

var userIDPattern = regexp.MustCompile(`^[UW][A-Z0-9]+$`)
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ResolveUserReference accepts canonical uppercase Slack user IDs, strict
// <@ID> mentions, and explicit @usernames. Username matching is case-insensitive
// and uses only Slack's username field; names and email aliases are not accepted.
func (c *APIClient) ResolveUserReference(ctx context.Context, reference string) (string, error) {
	resolved, err := c.ResolveUserReferences(ctx, []string{reference})
	if err != nil {
		return "", err
	}
	return resolved[0], nil
}

// ResolveUserReferences is the batch form of ResolveUserReference. It fetches
// the workspace user list at most once even when an MPDM or invite has several
// name-based references.
func (c *APIClient) ResolveUserReferences(ctx context.Context, references []string) ([]string, error) {
	if len(references) == 0 {
		return nil, fmt.Errorf("at least one user is required")
	}
	type pendingReference struct {
		input      string
		normalized string
		directID   string
	}
	pending := make([]pendingReference, len(references))
	needsLookup := false
	for i, reference := range references {
		directID, normalized, err := ParseUserReference(reference)
		if err != nil {
			return nil, err
		}
		pending[i] = pendingReference{input: strings.TrimSpace(reference), normalized: normalized, directID: directID}
		needsLookup = needsLookup || directID == ""
	}

	var users []slackapi.User
	if needsLookup {
		var err error
		users, err = c.sdk.GetUsersContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("list users while resolving references: %w", err)
		}
	}

	resolved := make([]string, len(pending))
	for i, reference := range pending {
		if reference.directID != "" {
			resolved[i] = reference.directID
			continue
		}
		var matches []slackapi.User
		for _, user := range users {
			if !user.Deleted && strings.EqualFold(reference.normalized, user.Name) {
				matches = append(matches, user)
			}
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("user not found: %s", reference.input)
		}
		if len(matches) > 1 {
			ids := make([]string, 0, len(matches))
			for _, user := range matches {
				ids = append(ids, user.ID)
			}
			return nil, fmt.Errorf("ambiguous user reference %s matches %s; use a user ID", reference.input, strings.Join(ids, ","))
		}
		resolved[i] = matches[0].ID
	}
	return resolved, nil
}

// ParseUserReference validates a reference without network calls. Exactly one of
// id or username is populated on success. IDs retain their canonical uppercase
// spelling; @usernames are returned without @ and matched case-insensitively.
// Explicit @ syntax always denotes a username, even when it resembles an ID.
func ParseUserReference(reference string) (id, username string, err error) {
	input := strings.TrimSpace(reference)
	if userIDPattern.MatchString(input) {
		return input, "", nil
	}
	if strings.HasPrefix(input, "<@") && strings.HasSuffix(input, ">") {
		candidate := input[2 : len(input)-1]
		if userIDPattern.MatchString(candidate) {
			return candidate, "", nil
		}
	}
	if strings.HasPrefix(input, "@") && usernamePattern.MatchString(input[1:]) {
		return "", input[1:], nil
	}
	return "", "", fmt.Errorf("invalid user reference %q: use a canonical uppercase Slack user ID, <@ID>, or @username; bare names and emails are not supported", input)
}
