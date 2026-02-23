package main

import "time"

// User states
const (
	StateIdle           = "IDLE"
	StateWaitingContent = "WAITING_CONTENT"
	StateWaitingConfirm = "WAITING_CONFIRM"
)

type Message struct {
	ID         int
	AnonNumber int
	UserID     int64
	Username   string
	CreatedAt  time.Time
}

type UserLimit struct {
	UserID int64
	Count  int
	Date   string
}

// DraftMessage holds user content before confirm/cancel
type DraftMessage struct {
	Text     string
	PhotoIDs []string
	VideoIDs []string
}
