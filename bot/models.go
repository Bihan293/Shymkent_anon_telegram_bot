package main

import "time"

// User states
const (
	StateIdle             = "IDLE"
	StateWaitingContent   = "WAITING_CONTENT"
	StateWaitingConfirm   = "WAITING_CONFIRM"
	StateChoosingLanguage = "CHOOSING_LANGUAGE"

	// Admin states for replying to anonymous users
	StateAdminReplyContent = "ADMIN_REPLY_CONTENT"
	StateAdminReplyConfirm = "ADMIN_REPLY_CONFIRM"

	// Admin broadcast states (to bot users)
	StateAdminBcastUsersContent = "ADMIN_BCAST_USERS_CONTENT"
	StateAdminBcastUsersButtons = "ADMIN_BCAST_USERS_BUTTONS"
	StateAdminBcastUsersConfirm = "ADMIN_BCAST_USERS_CONFIRM"

	// Admin broadcast states (to channel)
	StateAdminBcastChanTarget  = "ADMIN_BCAST_CHAN_TARGET"
	StateAdminBcastChanContent = "ADMIN_BCAST_CHAN_CONTENT"
	StateAdminBcastChanButtons = "ADMIN_BCAST_CHAN_BUTTONS"
	StateAdminBcastChanConfirm = "ADMIN_BCAST_CHAN_CONFIRM"

	// Admin states for managing mandatory subscription channels
	StateAdminReqChanAdd     = "ADMIN_REQCHAN_ADD"
	StateAdminReqChanEditMsg = "ADMIN_REQCHAN_EDITMSG"
)

// Message content limits
const (
	MaxPhotos     = 8
	MaxVideos     = 3
	MaxTextLength = 2000
)

// Settings keys (stored in DB).
const (
	SettingSubscribeMessage = "subscribe_message" // custom welcome/prompt text shown when user must subscribe
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

// InlineButton represents one inline url button used in broadcasts.
type InlineButton struct {
	Text string
	URL  string
}

// BroadcastTarget defines the target type of a broadcast.
type BroadcastTarget string

const (
	BroadcastUsers   BroadcastTarget = "USERS"
	BroadcastChannel BroadcastTarget = "CHANNEL"
)

// BroadcastDraft holds an admin's broadcast composition before sending.
type BroadcastDraft struct {
	Target    BroadcastTarget
	ChannelID string // for channel broadcast: @username or -100... id
	Text      string
	PhotoIDs  []string
	VideoIDs  []string
	Buttons   []InlineButton
}
