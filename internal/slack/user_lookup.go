package slack

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	slackapi "github.com/slack-go/slack"
)

var userIDPattern = regexp.MustCompile(`^[UW][A-Z0-9]+$`)

// ResolveUserReference resolves Slack user IDs, <@U...> mentions, @handles,
// display names, real names, and email addresses. It intentionally returns an
// ambiguity error rather than choosing an arbitrary account.
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
		input, normalized, directID, err := normalizeUserReference(reference)
		if err != nil {
			return nil, err
		}
		pending[i] = pendingReference{input: input, normalized: normalized, directID: directID}
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
			if !user.Deleted && equalUserReference(reference.normalized, user) {
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

func normalizeUserReference(reference string) (input, normalized, directID string, err error) {
	input = strings.TrimSpace(reference)
	if input == "" {
		return "", "", "", fmt.Errorf("user is required")
	}
	normalized = strings.TrimSuffix(strings.TrimPrefix(input, "<@"), ">")
	if userIDPattern.MatchString(strings.ToUpper(normalized)) {
		return input, normalized, strings.ToUpper(normalized), nil
	}
	normalized = strings.TrimSpace(strings.TrimPrefix(input, "@"))
	if normalized == "" {
		return "", "", "", fmt.Errorf("user is required")
	}
	if userIDPattern.MatchString(strings.ToUpper(normalized)) {
		return input, normalized, strings.ToUpper(normalized), nil
	}
	return input, normalized, "", nil
}

func equalUserReference(input string, user slackapi.User) bool {
	candidates := []string{
		user.Name,
		user.RealName,
		user.Profile.DisplayName,
		user.Profile.RealName,
		user.Profile.Email,
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" && strings.EqualFold(input, strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}
