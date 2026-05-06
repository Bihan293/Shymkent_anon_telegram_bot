package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var db *pgxpool.Pool

func InitDB(databaseURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}

	if err := pool.Ping(ctx); err != nil {
		return err
	}

	db = pool

	return createTables()
}

func createTables() error {
	ctx := context.Background()

	queries := []string{
		`CREATE TABLE IF NOT EXISTS messages (
			id SERIAL PRIMARY KEY,
			anon_number BIGSERIAL UNIQUE,
			user_id BIGINT NOT NULL,
			username TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS bans (
			user_id BIGINT PRIMARY KEY
		)`,
		`CREATE TABLE IF NOT EXISTS limits (
			user_id BIGINT NOT NULL,
			count INT NOT NULL DEFAULT 0,
			date TEXT NOT NULL,
			PRIMARY KEY (user_id, date)
		)`,
		// users table for broadcasting to all who interacted with the bot
		`CREATE TABLE IF NOT EXISTS users (
			user_id BIGINT PRIMARY KEY,
			username TEXT NOT NULL DEFAULT '',
			first_name TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			last_seen TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		// channels saved by admin for broadcasts
		`CREATE TABLE IF NOT EXISTS channels (
			id SERIAL PRIMARY KEY,
			chat_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL DEFAULT '',
			added_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		// channels for mandatory subscription
		`CREATE TABLE IF NOT EXISTS required_channels (
			id SERIAL PRIMARY KEY,
			chat_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL DEFAULT '',
			invite_link TEXT NOT NULL DEFAULT '',
			added_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		// settings storage (key-value)
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(ctx, q); err != nil {
			return err
		}
	}

	return nil
}

// ── User registration ─────────────────────────────────────────────────────

// UpsertUser stores/updates a user in the users table for broadcasting purposes.
func UpsertUser(userID int64, username, firstName string) error {
	ctx := context.Background()
	_, err := db.Exec(ctx,
		`INSERT INTO users (user_id, username, first_name, last_seen)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (user_id) DO UPDATE
		 SET username = EXCLUDED.username,
		     first_name = EXCLUDED.first_name,
		     last_seen = NOW()`,
		userID, username, firstName,
	)
	return err
}

// GetAllUserIDs returns every user_id stored in the users table.
func GetAllUserIDs() ([]int64, error) {
	ctx := context.Background()
	rows, err := db.Query(ctx, `SELECT user_id FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetUsersCount returns total registered users (table-based).
func GetUsersCount() (int, error) {
	ctx := context.Background()
	var c int
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&c)
	return c, err
}

// ── Channels ──────────────────────────────────────────────────────────────

type Channel struct {
	ID      int
	ChatID  string
	Title   string
	AddedAt time.Time
}

// AddChannel inserts or updates a channel for broadcasting.
func AddChannel(chatID, title string) error {
	ctx := context.Background()
	_, err := db.Exec(ctx,
		`INSERT INTO channels (chat_id, title) VALUES ($1, $2)
		 ON CONFLICT (chat_id) DO UPDATE SET title = EXCLUDED.title`,
		chatID, title,
	)
	return err
}

// RemoveChannel deletes a channel by chat_id.
func RemoveChannel(chatID string) error {
	ctx := context.Background()
	_, err := db.Exec(ctx, `DELETE FROM channels WHERE chat_id = $1`, chatID)
	return err
}

// ListChannels returns all stored channels.
func ListChannels() ([]Channel, error) {
	ctx := context.Background()
	rows, err := db.Query(ctx, `SELECT id, chat_id, title, added_at FROM channels ORDER BY added_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Channel
	for rows.Next() {
		var c Channel
		if err := rows.Scan(&c.ID, &c.ChatID, &c.Title, &c.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetChannelByID returns a channel by its db id.
func GetChannelByID(id int) (*Channel, error) {
	ctx := context.Background()
	var c Channel
	err := db.QueryRow(ctx,
		`SELECT id, chat_id, title, added_at FROM channels WHERE id = $1`,
		id,
	).Scan(&c.ID, &c.ChatID, &c.Title, &c.AddedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ── Required (mandatory subscription) channels ────────────────────────────

type RequiredChannel struct {
	ID         int
	ChatID     string
	Title      string
	InviteLink string
	AddedAt    time.Time
}

// AddRequiredChannel inserts or updates a channel that's mandatory for subscription.
func AddRequiredChannel(chatID, title, inviteLink string) error {
	ctx := context.Background()
	_, err := db.Exec(ctx,
		`INSERT INTO required_channels (chat_id, title, invite_link)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (chat_id) DO UPDATE
		 SET title = EXCLUDED.title,
		     invite_link = EXCLUDED.invite_link`,
		chatID, title, inviteLink,
	)
	return err
}

// RemoveRequiredChannel deletes a required channel by chat_id.
func RemoveRequiredChannel(chatID string) error {
	ctx := context.Background()
	_, err := db.Exec(ctx, `DELETE FROM required_channels WHERE chat_id = $1`, chatID)
	return err
}

// ListRequiredChannels returns all stored required-subscription channels.
func ListRequiredChannels() ([]RequiredChannel, error) {
	ctx := context.Background()
	rows, err := db.Query(ctx,
		`SELECT id, chat_id, title, invite_link, added_at
		 FROM required_channels ORDER BY added_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RequiredChannel
	for rows.Next() {
		var c RequiredChannel
		if err := rows.Scan(&c.ID, &c.ChatID, &c.Title, &c.InviteLink, &c.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetRequiredChannelByID returns a required channel by its db id.
func GetRequiredChannelByID(id int) (*RequiredChannel, error) {
	ctx := context.Background()
	var c RequiredChannel
	err := db.QueryRow(ctx,
		`SELECT id, chat_id, title, invite_link, added_at
		 FROM required_channels WHERE id = $1`,
		id,
	).Scan(&c.ID, &c.ChatID, &c.Title, &c.InviteLink, &c.AddedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CountRequiredChannels returns total number of mandatory subscription channels.
func CountRequiredChannels() (int, error) {
	ctx := context.Background()
	var c int
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM required_channels`).Scan(&c)
	return c, err
}

// ── Settings (key-value) ──────────────────────────────────────────────────

// GetSetting returns the value for a key (empty string if not set).
func GetSetting(key string) (string, error) {
	ctx := context.Background()
	var v string
	err := db.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&v)
	if err != nil {
		// not found is not an error here — just return empty
		return "", nil
	}
	return v, nil
}

// SetSetting upserts a key/value pair.
func SetSetting(key, value string) error {
	ctx := context.Background()
	_, err := db.Exec(ctx,
		`INSERT INTO settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value,
	)
	return err
}

// SaveMessage inserts a row and returns the auto-generated anon_number.
func SaveMessage(userID int64, username string) (int, error) {
	ctx := context.Background()

	var anonNum int
	err := db.QueryRow(ctx,
		`INSERT INTO messages (user_id, username) VALUES ($1, $2) RETURNING anon_number`,
		userID, username,
	).Scan(&anonNum)
	if err != nil {
		return 0, err
	}

	return anonNum, nil
}

func GetMessageInfo(anonNumber int) (*Message, error) {
	ctx := context.Background()
	msg := &Message{}

	err := db.QueryRow(ctx,
		`SELECT id, anon_number, user_id, username, created_at FROM messages WHERE anon_number = $1`,
		anonNumber,
	).Scan(&msg.ID, &msg.AnonNumber, &msg.UserID, &msg.Username, &msg.CreatedAt)

	if err != nil {
		return nil, err
	}
	return msg, nil
}

func CheckLimit(userID int64) (int, error) {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	var count int
	err := db.QueryRow(ctx,
		`SELECT count FROM limits WHERE user_id = $1 AND date = $2`,
		userID, today,
	).Scan(&count)

	if err != nil {
		return 0, nil // нет записи — 0 сообщений
	}
	return count, nil
}

func IncreaseLimit(userID int64) error {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	_, err := db.Exec(ctx,
		`INSERT INTO limits (user_id, count, date) VALUES ($1, 1, $2)
		 ON CONFLICT (user_id, date) DO UPDATE SET count = limits.count + 1`,
		userID, today,
	)
	return err
}

func BanUser(userID int64) error {
	ctx := context.Background()
	_, err := db.Exec(ctx,
		`INSERT INTO bans (user_id) VALUES ($1) ON CONFLICT DO NOTHING`,
		userID,
	)
	return err
}

func UnbanUser(userID int64) error {
	ctx := context.Background()
	_, err := db.Exec(ctx, `DELETE FROM bans WHERE user_id = $1`, userID)
	return err
}

func IsBanned(userID int64) (bool, error) {
	ctx := context.Background()
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM bans WHERE user_id = $1)`,
		userID,
	).Scan(&exists)
	return exists, err
}

func TodayMessageCount(userID int64) (int, error) {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	var count int
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages WHERE user_id = $1 AND created_at::date = $2::date`,
		userID, today,
	).Scan(&count)
	return count, err
}

// ── Statistics functions ──────────────────────────────────────────────────

// GetTotalUsers returns the count of unique users who have sent messages.
func GetTotalUsers() (int, error) {
	ctx := context.Background()
	var count int
	err := db.QueryRow(ctx, `SELECT COUNT(DISTINCT user_id) FROM messages`).Scan(&count)
	return count, err
}

// GetTotalMessages returns the total number of messages sent.
func GetTotalMessages() (int, error) {
	ctx := context.Background()
	var count int
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM messages`).Scan(&count)
	return count, err
}

// GetTotalBans returns the number of currently banned users.
func GetTotalBans() (int, error) {
	ctx := context.Background()
	var count int
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM bans`).Scan(&count)
	return count, err
}

// GetTodayMessages returns the count of messages sent today.
func GetTodayMessages() (int, error) {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")
	var count int
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages WHERE created_at::date = $1::date`,
		today,
	).Scan(&count)
	return count, err
}

// GetTodayUsers returns the count of unique users who sent messages today.
func GetTodayUsers() (int, error) {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")
	var count int
	err := db.QueryRow(ctx,
		`SELECT COUNT(DISTINCT user_id) FROM messages WHERE created_at::date = $1::date`,
		today,
	).Scan(&count)
	return count, err
}

// GetWeekMessages returns the count of messages sent in the last 7 days.
func GetWeekMessages() (int, error) {
	ctx := context.Background()
	var count int
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages WHERE created_at >= NOW() - INTERVAL '7 days'`,
	).Scan(&count)
	return count, err
}

// GetTopUser returns the user_id and count of the user who sent the most messages.
func GetTopUser() (int64, int, error) {
	ctx := context.Background()
	var userID int64
	var count int
	err := db.QueryRow(ctx,
		`SELECT user_id, COUNT(*) as cnt FROM messages GROUP BY user_id ORDER BY cnt DESC LIMIT 1`,
	).Scan(&userID, &count)
	if err != nil {
		return 0, 0, err
	}
	return userID, count, nil
}

// GetLastMessageTime returns the time of the most recent message.
func GetLastMessageTime() (*time.Time, error) {
	ctx := context.Background()
	var t time.Time
	err := db.QueryRow(ctx,
		`SELECT created_at FROM messages ORDER BY created_at DESC LIMIT 1`,
	).Scan(&t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
