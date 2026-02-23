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
	}

	for _, q := range queries {
		if _, err := db.Exec(ctx, q); err != nil {
			return err
		}
	}

	return nil
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
