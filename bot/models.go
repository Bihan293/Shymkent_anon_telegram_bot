package main

import "time"

// User states
const (
	StateIdle           = "IDLE"
	StateWaitingContent = "WAITING_CONTENT"
	StateWaitingConfirm = "WAITING_CONFIRM"

	// Admin states for replying to anonymous users
	StateAdminReplyContent = "ADMIN_REPLY_CONTENT"
	StateAdminReplyConfirm = "ADMIN_REPLY_CONFIRM"
)

// Message content limits
const (
	MaxPhotos     = 8
	MaxVideos     = 3
	MaxTextLength = 2000
)

// Channel for mandatory subscription check
const (
	ChannelUsername = "@shymkent_anon"
	ChannelLink    = "https://t.me/shymkent_anon"
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

// AdminReplyDraft holds admin's reply content before sending to the user
type AdminReplyDraft struct {
	TargetUserID int64
	AnonNumber   int
	Text         string
	PhotoIDs     []string
	VideoIDs     []string
}
