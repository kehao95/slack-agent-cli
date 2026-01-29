package cache

import (
	"context"
	"fmt"
	"io"
	"time"

	slackapi "github.com/slack-go/slack"
)

// PopulateConfig controls cache population behavior.
type PopulateConfig struct {
	// PageSize is the number of items per API call (default 200).
	PageSize int
	// PageDelay is the delay between pages to avoid rate limits (default 1s).
	PageDelay time.Duration
	// FetchAll continues fetching until complete (default false = one page).
	FetchAll bool
	// Output for progress messages (can be nil for silent operation).
	Output io.Writer
}

// DefaultPopulateConfig returns sensible defaults.
func DefaultPopulateConfig() PopulateConfig {
	return PopulateConfig{
		PageSize:  200,
		PageDelay: time.Second,
		FetchAll:  false,
		Output:    nil,
	}
}

// PopulateResult describes the outcome of a populate operation.
type PopulateResult struct {
	Count      int
	Complete   bool
	NextCursor string
	Pages      int
}

// ChannelFetcher defines the interface for fetching channels.
type ChannelFetcher interface {
	ListChannels(ctx context.Context, cursor string, limit int) ([]slackapi.Channel, string, error)
}

// UserFetcher defines the interface for fetching users.
type UserFetcher interface {
	ListUsers(ctx context.Context, cursor string, limit int) ([]slackapi.User, string, error)
}

// PopulateChannels incrementally populates the channel cache.
// It resumes from any existing partial cache and saves progress after each page.
func (s *Store) PopulateChannels(ctx context.Context, fetcher ChannelFetcher, cfg PopulateConfig) (PopulateResult, error) {
	if cfg.PageSize == 0 {
		cfg.PageSize = 200
	}
	if cfg.PageDelay == 0 {
		cfg.PageDelay = time.Second
	}

	// Check for complete cache first
	var complete []slackapi.Channel
	if found, _ := s.Load(CacheKeyChannels, &complete); found {
		return PopulateResult{
			Count:    len(complete),
			Complete: true,
		}, nil
	}

	// Load partial progress
	var channels []slackapi.Channel
	state, found, err := s.LoadPartial(CacheKeyChannels, &channels)
	if err != nil {
		return PopulateResult{}, err
	}

	cursor := ""
	if found && !state.Complete {
		cursor = state.NextCursor
		s.log(cfg.Output, "Resuming from %d channels (cursor: %s...)\n", len(channels), truncate(cursor, 20))
	} else if !found {
		channels = make([]slackapi.Channel, 0)
	}

	pages := 0
	for {
		select {
		case <-ctx.Done():
			// Save progress before exiting
			_ = s.SavePartial(CacheKeyChannels, channels, cursor, false, len(channels))
			return PopulateResult{
				Count:      len(channels),
				Complete:   false,
				NextCursor: cursor,
				Pages:      pages,
			}, ctx.Err()
		default:
		}

		page, nextCursor, err := fetcher.ListChannels(ctx, cursor, cfg.PageSize)
		if err != nil {
			// Save progress before returning error
			_ = s.SavePartial(CacheKeyChannels, channels, cursor, false, len(channels))
			return PopulateResult{
				Count:      len(channels),
				Complete:   false,
				NextCursor: cursor,
				Pages:      pages,
			}, fmt.Errorf("fetch channels: %w", err)
		}

		channels = append(channels, page...)
		pages++
		s.log(cfg.Output, "Fetched page %d: %d channels total\n", pages, len(channels))

		// Save progress after each page
		if nextCursor == "" {
			// Complete - promote to main cache
			if err := s.PromotePartial(CacheKeyChannels, channels); err != nil {
				return PopulateResult{}, err
			}
			return PopulateResult{
				Count:    len(channels),
				Complete: true,
				Pages:    pages,
			}, nil
		}

		// Save partial progress
		if err := s.SavePartial(CacheKeyChannels, channels, nextCursor, false, len(channels)); err != nil {
			return PopulateResult{}, err
		}

		if !cfg.FetchAll {
			// Single page mode - return with cursor for next call
			return PopulateResult{
				Count:      len(channels),
				Complete:   false,
				NextCursor: nextCursor,
				Pages:      pages,
			}, nil
		}

		cursor = nextCursor

		// Rate limit delay
		select {
		case <-ctx.Done():
			return PopulateResult{
				Count:      len(channels),
				Complete:   false,
				NextCursor: nextCursor,
				Pages:      pages,
			}, ctx.Err()
		case <-time.After(cfg.PageDelay):
		}
	}
}

// CacheKeyChannels is the cache key for channels.
const CacheKeyChannels = "channels"

// CacheKeyUsers is the cache key for users.
const CacheKeyUsers = "users"

// PopulateUsers incrementally populates the user cache.
func (s *Store) PopulateUsers(ctx context.Context, fetcher UserFetcher, cfg PopulateConfig) (PopulateResult, error) {
	if cfg.PageSize == 0 {
		cfg.PageSize = 200
	}
	if cfg.PageDelay == 0 {
		cfg.PageDelay = time.Second
	}

	// Check for complete cache first
	var complete []slackapi.User
	if found, _ := s.Load(CacheKeyUsers, &complete); found {
		return PopulateResult{
			Count:    len(complete),
			Complete: true,
		}, nil
	}

	// Load partial progress
	var users []slackapi.User
	state, found, err := s.LoadPartial(CacheKeyUsers, &users)
	if err != nil {
		return PopulateResult{}, err
	}

	cursor := ""
	if found && !state.Complete {
		cursor = state.NextCursor
		s.log(cfg.Output, "Resuming from %d users (cursor: %s...)\n", len(users), truncate(cursor, 20))
	} else if !found {
		users = make([]slackapi.User, 0)
	}

	pages := 0
	for {
		select {
		case <-ctx.Done():
			_ = s.SavePartial(CacheKeyUsers, users, cursor, false, len(users))
			return PopulateResult{
				Count:      len(users),
				Complete:   false,
				NextCursor: cursor,
				Pages:      pages,
			}, ctx.Err()
		default:
		}

		page, nextCursor, err := fetcher.ListUsers(ctx, cursor, cfg.PageSize)
		if err != nil {
			_ = s.SavePartial(CacheKeyUsers, users, cursor, false, len(users))
			return PopulateResult{
				Count:      len(users),
				Complete:   false,
				NextCursor: cursor,
				Pages:      pages,
			}, fmt.Errorf("fetch users: %w", err)
		}

		users = append(users, page...)
		pages++
		s.log(cfg.Output, "Fetched page %d: %d users total\n", pages, len(users))

		if nextCursor == "" {
			if err := s.PromotePartial(CacheKeyUsers, users); err != nil {
				return PopulateResult{}, err
			}
			return PopulateResult{
				Count:    len(users),
				Complete: true,
				Pages:    pages,
			}, nil
		}

		if err := s.SavePartial(CacheKeyUsers, users, nextCursor, false, len(users)); err != nil {
			return PopulateResult{}, err
		}

		if !cfg.FetchAll {
			return PopulateResult{
				Count:      len(users),
				Complete:   false,
				NextCursor: nextCursor,
				Pages:      pages,
			}, nil
		}

		cursor = nextCursor

		select {
		case <-ctx.Done():
			return PopulateResult{
				Count:      len(users),
				Complete:   false,
				NextCursor: nextCursor,
				Pages:      pages,
			}, ctx.Err()
		case <-time.After(cfg.PageDelay):
		}
	}
}

func (s *Store) log(w io.Writer, format string, args ...interface{}) {
	if w != nil {
		fmt.Fprintf(w, format, args...)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
