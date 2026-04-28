// Package eventstore provides a local SQLite-backed Slack event log.
package eventstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Event is the normalized event shape stored by the daemon and returned by queries.
type Event struct {
	Cursor           int64           `json:"cursor,omitempty"`
	ReceivedAt       time.Time       `json:"received_at,omitempty"`
	Kind             string          `json:"kind"`
	EnvelopeID       string          `json:"envelope_id,omitempty"`
	EventID          string          `json:"event_id,omitempty"`
	EventTime        int             `json:"event_time,omitempty"`
	Type             string          `json:"type"`
	Subtype          string          `json:"subtype,omitempty"`
	Channel          string          `json:"channel,omitempty"`
	ChannelID        string          `json:"channel_id,omitempty"`
	ConversationType string          `json:"conversation_type,omitempty"`
	User             string          `json:"user,omitempty"`
	UserID           string          `json:"user_id,omitempty"`
	BotID            string          `json:"bot_id,omitempty"`
	ItemUser         string          `json:"item_user,omitempty"`
	ItemUserID       string          `json:"item_user_id,omitempty"`
	Reaction         string          `json:"reaction,omitempty"`
	TS               string          `json:"ts,omitempty"`
	ThreadTS         string          `json:"thread_ts,omitempty"`
	Text             string          `json:"text,omitempty"`
	IsThreadReply    bool            `json:"is_thread_reply,omitempty"`
	IsThreadRoot     bool            `json:"is_thread_root,omitempty"`
	IsSelf           bool            `json:"is_self,omitempty"`
	Raw              json.RawMessage `json:"raw,omitempty"`
}

// ClaimedEvent describes an event currently leased by a worker.
type ClaimedEvent struct {
	Event
	ClaimedAt  time.Time `json:"claimed_at,omitempty"`
	LeaseUntil time.Time `json:"lease_until,omitempty"`
}

// SelfIdentity describes the active Slack principal used for self filtering.
type SelfIdentity struct {
	Role   string
	UserID string
	BotID  string
}

// Matches reports whether an event actor belongs to this identity.
func (s SelfIdentity) Matches(userID, botID string) bool {
	userID = strings.TrimSpace(userID)
	botID = strings.TrimSpace(botID)
	selfUserID := strings.TrimSpace(s.UserID)
	selfBotID := strings.TrimSpace(s.BotID)

	switch strings.ToLower(strings.TrimSpace(s.Role)) {
	case "user":
		return userID != "" && selfUserID != "" && userID == selfUserID
	case "bot":
		return (botID != "" && selfBotID != "" && botID == selfBotID) ||
			(userID != "" && selfUserID != "" && userID == selfUserID)
	default:
		return (userID != "" && selfUserID != "" && userID == selfUserID) ||
			(botID != "" && selfBotID != "" && botID == selfBotID)
	}
}

// Filter describes event query constraints.
type Filter struct {
	Type              string
	MessageKind       string
	MentionUserID     string
	ChannelID         string
	ConversationTypes map[string]struct{}
	ThreadTS          string
	UserID            string
	ThreadsOnly       bool
	ExcludeSelf       bool
	SelfIdentity      SelfIdentity
	SinceCursor       int64
	SinceReceivedAt   time.Time
	Limit             int
}

// Store wraps an event SQLite database.
type Store struct {
	db   *sql.DB
	path string
}

// DefaultPath returns the default event DB path adjacent to the slk config directory.
func DefaultPath(configPath, teamID string) (string, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return "", errors.New("team id is required")
	}
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determine home directory: %w", err)
		}
		configPath = filepath.Join(home, ".config", "slack-cli", "config.json")
	}
	return filepath.Join(filepath.Dir(configPath), "events", teamID, "events.db"), nil
}

// Open opens or creates an event store.
func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("event store path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create event store dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db, path: path}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Path returns the backing SQLite path.
func (s *Store) Path() string {
	return s.path
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) init() error {
	stmts := []string{
		`PRAGMA busy_timeout=5000`,
		`PRAGMA journal_mode=WAL`,
		`CREATE TABLE IF NOT EXISTS events (
			cursor INTEGER PRIMARY KEY AUTOINCREMENT,
			received_at TEXT NOT NULL,
			kind TEXT,
			envelope_id TEXT,
			event_id TEXT,
			event_time INTEGER,
			type TEXT NOT NULL,
			subtype TEXT,
			channel TEXT,
			channel_id TEXT,
			conversation_type TEXT,
			user TEXT,
			user_id TEXT,
			bot_id TEXT,
			item_user TEXT,
			item_user_id TEXT,
			reaction TEXT,
			ts TEXT,
			thread_ts TEXT,
			text TEXT,
			is_thread_reply INTEGER NOT NULL DEFAULT 0,
			is_thread_root INTEGER NOT NULL DEFAULT 0,
			is_self INTEGER NOT NULL DEFAULT 0,
			claim_count INTEGER NOT NULL DEFAULT 0,
			claimed_at TEXT,
			claim_expires_at TEXT,
			acked_at TEXT,
			raw_json BLOB,
			event_json BLOB NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_events_event_id ON events(event_id) WHERE event_id != ''`,
		`CREATE INDEX IF NOT EXISTS idx_events_received_at ON events(received_at)`,
		`CREATE INDEX IF NOT EXISTS idx_events_channel ON events(channel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_events_thread ON events(thread_ts, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_events_type ON events(type, subtype)`,
		`CREATE INDEX IF NOT EXISTS idx_events_self ON events(is_self)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init event store: %w", err)
		}
	}
	if err := s.ensureColumn("bot_id", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("claim_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("claimed_at", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("claim_expires_at", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("acked_at", "TEXT"); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_queue ON events(acked_at, claim_expires_at, cursor)`); err != nil {
		return fmt.Errorf("init event store: %w", err)
	}
	return nil
}

func (s *Store) ensureColumn(name, definition string) error {
	rows, err := s.db.Query(`PRAGMA table_info(events)`)
	if err != nil {
		return fmt.Errorf("inspect event store schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			columnName string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return fmt.Errorf("scan event store schema: %w", err)
		}
		if columnName == name {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect event store schema: %w", err)
	}
	if _, err := s.db.Exec(`ALTER TABLE events ADD COLUMN ` + name + ` ` + definition); err != nil {
		return fmt.Errorf("migrate event store schema: %w", err)
	}
	return nil
}

// Insert appends an event and returns its local cursor.
func (s *Store) Insert(ctx context.Context, event Event) (int64, error) {
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = time.Now().UTC()
	} else {
		event.ReceivedAt = event.ReceivedAt.UTC()
	}
	event.Cursor = 0
	payload, err := json.Marshal(event)
	if err != nil {
		return 0, fmt.Errorf("marshal event: %w", err)
	}

	res, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO events (
		received_at, kind, envelope_id, event_id, event_time, type, subtype,
		channel, channel_id, conversation_type, user, user_id, bot_id, item_user, item_user_id,
		reaction, ts, thread_ts, text, is_thread_reply, is_thread_root, is_self, raw_json, event_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ReceivedAt.Format(time.RFC3339Nano),
		event.Kind,
		event.EnvelopeID,
		event.EventID,
		event.EventTime,
		event.Type,
		event.Subtype,
		event.Channel,
		event.ChannelID,
		event.ConversationType,
		event.User,
		event.UserID,
		event.BotID,
		event.ItemUser,
		event.ItemUserID,
		event.Reaction,
		event.TS,
		event.ThreadTS,
		event.Text,
		boolInt(event.IsThreadReply),
		boolInt(event.IsThreadRoot),
		boolInt(event.IsSelf),
		[]byte(event.Raw),
		payload,
	)
	if err != nil {
		return 0, fmt.Errorf("insert event: %w", err)
	}
	cursor, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read inserted cursor: %w", err)
	}
	return cursor, nil
}

// Claim atomically selects one matching event and marks it as leased until now+lease.
func (s *Store) Claim(ctx context.Context, filter Filter, lease time.Duration) (ClaimedEvent, bool, error) {
	var claimed ClaimedEvent
	if lease <= 0 {
		return claimed, false, fmt.Errorf("lease must be greater than zero")
	}

	now := time.Now().UTC()
	leaseUntil := now.Add(lease)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return claimed, false, fmt.Errorf("begin claim transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	where, args := buildClaimWhere(filter, now)
	selectStmt := `SELECT cursor, received_at, is_self, event_json FROM events ` + where + ` ORDER BY cursor ASC LIMIT 1`
	var event Event
	row := tx.QueryRowContext(ctx, selectStmt, args...)
	if err := scanEventInto(&event, row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return claimed, false, nil
		}
		return claimed, false, fmt.Errorf("claim select: %w", err)
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE events
		 SET claimed_at = ?, claim_expires_at = ?, claim_count = COALESCE(claim_count, 0) + 1
		 WHERE cursor = ?
		 AND acked_at IS NULL
		 AND (claim_expires_at IS NULL OR claim_expires_at <= ?)`,
		now.Format(time.RFC3339Nano),
		leaseUntil.Format(time.RFC3339Nano),
		event.Cursor,
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return claimed, false, fmt.Errorf("claim update: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return claimed, false, fmt.Errorf("claim rows affected: %w", err)
	}
	if rows != 1 {
		return claimed, false, nil
	}

	if err := tx.Commit(); err != nil {
		return claimed, false, fmt.Errorf("commit claim: %w", err)
	}

	claimed = ClaimedEvent{
		Event:      event,
		ClaimedAt:  now,
		LeaseUntil: leaseUntil,
	}
	return claimed, true, nil
}

// Ack marks a claimed or unclaimed event as processed.
func (s *Store) Ack(ctx context.Context, cursor int64) (bool, error) {
	if cursor <= 0 {
		return false, fmt.Errorf("cursor must be positive")
	}

	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `UPDATE events SET acked_at = ? WHERE cursor = ? AND acked_at IS NULL`, now.Format(time.RFC3339Nano), cursor)
	if err != nil {
		return false, fmt.Errorf("ack event: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read ack rows: %w", err)
	}
	if rows == 1 {
		return true, nil
	}

	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM events WHERE cursor = ?)`, cursor).Scan(&exists); err != nil {
		return false, fmt.Errorf("check event exists: %w", err)
	}
	if !exists {
		return false, fmt.Errorf("event %d not found", cursor)
	}
	return false, nil
}

// Query returns matching events ordered by local cursor.
func (s *Store) Query(ctx context.Context, filter Filter) ([]Event, error) {
	where, args := buildWhere(filter)
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, `SELECT cursor, received_at, is_self, event_json FROM events `+where+` ORDER BY cursor ASC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan events: %w", err)
	}
	return events, nil
}

// LatestCursor returns the largest stored cursor, or 0 when the store is empty.
func (s *Store) LatestCursor(ctx context.Context) (int64, error) {
	var cursor sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(cursor) FROM events`).Scan(&cursor); err != nil {
		return 0, fmt.Errorf("latest cursor: %w", err)
	}
	if !cursor.Valid {
		return 0, nil
	}
	return cursor.Int64, nil
}

// Count returns the number of retained events.
func (s *Store) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count events: %w", err)
	}
	return count, nil
}

// PruneOlderThan deletes events older than cutoff.
func (s *Store) PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE received_at < ?`, cutoff.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("prune events: %w", err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read pruned count: %w", err)
	}
	return count, nil
}

func buildWhere(filter Filter) (string, []interface{}) {
	clauses := []string{"1=1"}
	args := []interface{}{}
	if filter.SinceCursor > 0 {
		clauses = append(clauses, "cursor > ?")
		args = append(args, filter.SinceCursor)
	}
	if !filter.SinceReceivedAt.IsZero() {
		clauses = append(clauses, "received_at >= ?")
		args = append(args, filter.SinceReceivedAt.UTC().Format(time.RFC3339Nano))
	}
	if filter.ChannelID != "" {
		clauses = append(clauses, "channel_id = ?")
		args = append(args, filter.ChannelID)
	}
	if filter.Type != "" {
		clauses = append(clauses, "type = ?")
		args = append(args, filter.Type)
	}
	switch filter.MessageKind {
	case "root":
		clauses = append(clauses, "type = 'message' AND is_thread_reply = 0 AND subtype NOT IN ('message_replied', 'thread_broadcast')")
	case "reply":
		clauses = append(clauses, "type = 'message' AND (is_thread_reply = 1 OR subtype = 'thread_broadcast')")
	case "thread":
		clauses = append(clauses, "type = 'message' AND (is_thread_reply = 1 OR is_thread_root = 1 OR subtype IN ('message_replied', 'thread_broadcast'))")
	}
	if filter.MentionUserID != "" {
		clauses = append(clauses, "type = 'message' AND text LIKE ?")
		args = append(args, "%<@"+filter.MentionUserID+">%")
	}
	if len(filter.ConversationTypes) > 0 {
		placeholders := make([]string, 0, len(filter.ConversationTypes))
		for conversationType := range filter.ConversationTypes {
			placeholders = append(placeholders, "?")
			args = append(args, conversationType)
		}
		clauses = append(clauses, "conversation_type IN ("+strings.Join(placeholders, ",")+")")
	}
	if filter.ThreadTS != "" {
		clauses = append(clauses, "(thread_ts = ? OR ts = ?)")
		args = append(args, filter.ThreadTS, filter.ThreadTS)
	}
	if filter.UserID != "" {
		clauses = append(clauses, "user_id = ?")
		args = append(args, filter.UserID)
	}
	if filter.ThreadsOnly {
		clauses = append(clauses, "(is_thread_reply = 1 OR is_thread_root = 1 OR subtype IN ('message_replied', 'thread_broadcast'))")
	}
	if filter.ExcludeSelf {
		if clause, clauseArgs, ok := buildExcludeSelfClause(filter.SelfIdentity); ok {
			clauses = append(clauses, clause)
			args = append(args, clauseArgs...)
		} else {
			clauses = append(clauses, "is_self = 0")
		}
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func buildExcludeSelfClause(identity SelfIdentity) (string, []interface{}, bool) {
	selfUserID := strings.TrimSpace(identity.UserID)
	selfBotID := strings.TrimSpace(identity.BotID)

	switch strings.ToLower(strings.TrimSpace(identity.Role)) {
	case "user":
		if selfUserID == "" {
			return "", nil, false
		}
		return "COALESCE(user_id, '') != ?", []interface{}{selfUserID}, true
	case "bot":
		switch {
		case selfBotID != "" && selfUserID != "":
			return "NOT (COALESCE(bot_id, '') = ? OR (COALESCE(bot_id, '') = '' AND COALESCE(user_id, '') = ?))", []interface{}{selfBotID, selfUserID}, true
		case selfBotID != "":
			return "COALESCE(bot_id, '') != ?", []interface{}{selfBotID}, true
		case selfUserID != "":
			return "COALESCE(user_id, '') != ?", []interface{}{selfUserID}, true
		default:
			return "", nil, false
		}
	default:
		switch {
		case selfUserID != "" && selfBotID != "":
			return "NOT (COALESCE(user_id, '') = ? OR COALESCE(bot_id, '') = ?)", []interface{}{selfUserID, selfBotID}, true
		case selfUserID != "":
			return "COALESCE(user_id, '') != ?", []interface{}{selfUserID}, true
		case selfBotID != "":
			return "COALESCE(bot_id, '') != ?", []interface{}{selfBotID}, true
		default:
			return "", nil, false
		}
	}
}

func buildClaimWhere(filter Filter, now time.Time) (string, []interface{}) {
	clauses, args := buildWhere(filter)
	clauses += " AND acked_at IS NULL AND (claim_expires_at IS NULL OR claim_expires_at <= ?)"
	args = append(args, now.Format(time.RFC3339Nano))
	return clauses, args
}

func scanEvent(scanner interface {
	Scan(dest ...interface{}) error
}) (Event, error) {
	var event Event
	if err := scanEventInto(&event, scanner); err != nil {
		return Event{}, fmt.Errorf("scan event row: %w", err)
	}
	return event, nil
}

func scanEventInto(event *Event, scanner interface {
	Scan(dest ...interface{}) error
}) error {
	var (
		cursor        int64
		receivedAtRaw string
		isSelf        int
		payload       []byte
	)
	if err := scanner.Scan(&cursor, &receivedAtRaw, &isSelf, &payload); err != nil {
		return fmt.Errorf("scan event row: %w", err)
	}
	var parsed Event
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return fmt.Errorf("unmarshal stored event: %w", err)
	}
	receivedAt, err := time.Parse(time.RFC3339Nano, receivedAtRaw)
	if err != nil {
		return fmt.Errorf("parse received_at: %w", err)
	}
	parsed.Cursor = cursor
	parsed.ReceivedAt = receivedAt
	parsed.IsSelf = isSelf == 1
	*event = parsed
	return nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
