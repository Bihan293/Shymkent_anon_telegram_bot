package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ── State Machine ──────────────────────────────────────────────────────────
var (
	userStates   = make(map[int64]string)
	userDrafts   = make(map[int64]*DraftMessage)
	userCooldown = make(map[int64]time.Time) // last successful send time
	mu           sync.Mutex
)

const cooldownDuration = 5 * time.Minute

func getState(userID int64) string {
	mu.Lock()
	defer mu.Unlock()
	s, ok := userStates[userID]
	if !ok {
		return StateIdle
	}
	return s
}

func setState(userID int64, state string) {
	mu.Lock()
	defer mu.Unlock()
	userStates[userID] = state
}

func setDraft(userID int64, d *DraftMessage) {
	mu.Lock()
	defer mu.Unlock()
	userDrafts[userID] = d
}

func getDraft(userID int64) *DraftMessage {
	mu.Lock()
	defer mu.Unlock()
	return userDrafts[userID]
}

func deleteDraft(userID int64) {
	mu.Lock()
	defer mu.Unlock()
	delete(userDrafts, userID)
}

// ── Media-group buffer ─────────────────────────────────────────────────────
var (
	mediaBuffer = make(map[string][]tgbotapi.Message)
	mediaTimers = make(map[string]*time.Timer)
	mediaMu     sync.Mutex
)

// ── Validation helpers ─────────────────────────────────────────────────────

func validateDraftLimits(draft *DraftMessage) string {
	if len(draft.PhotoIDs) > MaxPhotos {
		return fmt.Sprintf("⚠️ Слишком много фото. Максимум: %d. Вы отправили: %d.", MaxPhotos, len(draft.PhotoIDs))
	}
	if len(draft.VideoIDs) > MaxVideos {
		return fmt.Sprintf("⚠️ Слишком много видео. Максимум: %d. Вы отправили: %d.", MaxVideos, len(draft.VideoIDs))
	}
	textLen := len([]rune(draft.Text))
	if textLen > MaxTextLength {
		return fmt.Sprintf("⚠️ Слишком длинный текст. Максимум: %d символов. У вас: %d.", MaxTextLength, textLen)
	}
	return ""
}

// ── Subscription check ─────────────────────────────────────────────────────

func IsSubscribed(bot *tgbotapi.BotAPI, userID int64) bool {
	chatCfg := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			SuperGroupUsername: ChannelUsername,
			UserID:             userID,
		},
	}

	member, err := bot.GetChatMember(chatCfg)
	if err != nil {
		log.Printf("GetChatMember error for user %d: %v", userID, err)
		return false
	}

	switch member.Status {
	case "creator", "administrator", "member":
		return true
	default:
		return false
	}
}

func sendSubscriptionMessage(bot *tgbotapi.BotAPI, chatID int64) {
	text := "❗ Чтобы пользоваться ботом, подпишитесь на наш официальный канал:\n\n📢 " + ChannelLink
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = SubscriptionKeyboard()
	bot.Send(msg)
}

// ── Handlers ───────────────────────────────────────────────────────────────

func HandleStart(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	setState(message.From.ID, StateIdle)
	deleteDraft(message.From.ID)

	// Check channel subscription
	if !IsSubscribed(bot, message.From.ID) {
		sendSubscriptionMessage(bot, message.Chat.ID)
		return
	}

	text := "Анонимные сообщения администратору.\nОсновной канал: https://t.me/shymkent_anon"
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyMarkup = UserKeyboard()
	bot.Send(msg)
}

func HandleCreateMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID

	// Check channel subscription
	if !IsSubscribed(bot, userID) {
		sendSubscriptionMessage(bot, message.Chat.ID)
		return
	}

	banned, err := IsBanned(userID)
	if err != nil {
		log.Printf("IsBanned error: %v", err)
		return
	}
	if banned {
		msg := tgbotapi.NewMessage(message.Chat.ID, "⛔ Вы заблокированы.")
		bot.Send(msg)
		return
	}

	// Check 5-minute cooldown
	if remaining := getCooldownRemaining(userID); remaining > 0 {
		minutes := int(remaining.Minutes())
		seconds := int(remaining.Seconds()) % 60
		text := fmt.Sprintf("⏳ Подождите %d мин. %d сек. перед отправкой следующего сообщения.", minutes, seconds)
		msg := tgbotapi.NewMessage(message.Chat.ID, text)
		bot.Send(msg)
		return
	}

	count, err := CheckLimit(userID)
	if err != nil {
		log.Printf("CheckLimit error: %v", err)
		return
	}
	if count >= 3 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы исчерпали лимит сообщений на сегодня (3/3).")
		bot.Send(msg)
		return
	}

	remaining := 3 - count
	text := fmt.Sprintf("У вас осталось %d/3 сообщений сегодня.\nНапишите ваше сообщение. Можно прикрепить фото и видео.", remaining)
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	bot.Send(msg)

	setState(userID, StateWaitingContent)
}

// HandleUserMessage processes incoming content when user is in WAITING_CONTENT.
func HandleUserMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID

	if getState(userID) != StateWaitingContent {
		return
	}

	// Check channel subscription
	if !IsSubscribed(bot, userID) {
		setState(userID, StateIdle)
		sendSubscriptionMessage(bot, message.Chat.ID)
		return
	}

	// Check ban again
	banned, _ := IsBanned(userID)
	if banned {
		msg := tgbotapi.NewMessage(message.Chat.ID, "⛔ Вы заблокированы.")
		setState(userID, StateIdle)
		bot.Send(msg)
		return
	}

	// Check limit again
	count, _ := CheckLimit(userID)
	if count >= 3 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы исчерпали лимит сообщений на сегодня (3/3).")
		setState(userID, StateIdle)
		bot.Send(msg)
		return
	}

	// ── Media-group (album) ────────────────────────────────────────────
	if message.MediaGroupID != "" {
		handleMediaGroup(bot, message)
		return
	}

	// ── Single message (text / single photo / single video) ────────────
	draft := &DraftMessage{}

	if message.Photo != nil {
		best := message.Photo[len(message.Photo)-1]
		draft.PhotoIDs = append(draft.PhotoIDs, best.FileID)
	}
	if message.Video != nil {
		draft.VideoIDs = append(draft.VideoIDs, message.Video.FileID)
	}

	// Caption or plain text
	if message.Caption != "" {
		draft.Text = message.Caption
	} else if message.Text != "" {
		draft.Text = message.Text
	}

	// Must have at least something
	if draft.Text == "" && len(draft.PhotoIDs) == 0 && len(draft.VideoIDs) == 0 {
		return
	}

	// Validate content limits
	if errMsg := validateDraftLimits(draft); errMsg != "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, errMsg)
		bot.Send(msg)
		return
	}

	setDraft(userID, draft)
	setState(userID, StateWaitingConfirm)
	sendPreview(bot, message.Chat.ID, draft)
}

// ── Media-group logic ──────────────────────────────────────────────────────

func handleMediaGroup(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	groupID := message.MediaGroupID
	userID := message.From.ID
	chatID := message.Chat.ID

	mediaMu.Lock()
	mediaBuffer[groupID] = append(mediaBuffer[groupID], *message)

	// Reset or start the timer for this group
	if t, ok := mediaTimers[groupID]; ok {
		t.Stop()
	}
	mediaTimers[groupID] = time.AfterFunc(1*time.Second, func() {
		mediaMu.Lock()
		messages := mediaBuffer[groupID]
		delete(mediaBuffer, groupID)
		delete(mediaTimers, groupID)
		mediaMu.Unlock()

		draft := buildDraftFromAlbum(messages)

		// Validate content limits
		if errMsg := validateDraftLimits(draft); errMsg != "" {
			msg := tgbotapi.NewMessage(chatID, errMsg)
			bot.Send(msg)
			return
		}

		setDraft(userID, draft)
		setState(userID, StateWaitingConfirm)
		sendPreview(bot, chatID, draft)
	})
	mediaMu.Unlock()
}

func buildDraftFromAlbum(messages []tgbotapi.Message) *DraftMessage {
	draft := &DraftMessage{}
	for _, m := range messages {
		if m.Photo != nil {
			best := m.Photo[len(m.Photo)-1]
			draft.PhotoIDs = append(draft.PhotoIDs, best.FileID)
		}
		if m.Video != nil {
			draft.VideoIDs = append(draft.VideoIDs, m.Video.FileID)
		}
		// Take caption from the first message that has one
		if draft.Text == "" && m.Caption != "" {
			draft.Text = m.Caption
		}
	}
	return draft
}

// ── Preview ────────────────────────────────────────────────────────────────

func sendPreview(bot *tgbotapi.BotAPI, chatID int64, draft *DraftMessage) {
	header := "Анон предпросмотр:"
	keyboard := ConfirmSendKeyboard()

	totalMedia := len(draft.PhotoIDs) + len(draft.VideoIDs)

	// Album (multiple media)
	if totalMedia > 1 {
		var mediaGroup []interface{}
		first := true
		for _, pid := range draft.PhotoIDs {
			item := tgbotapi.NewInputMediaPhoto(tgbotapi.FileID(pid))
			if first {
				caption := header
				if draft.Text != "" {
					caption = fmt.Sprintf("%s\n\n%s", header, draft.Text)
				}
				item.Caption = caption
				first = false
			}
			mediaGroup = append(mediaGroup, item)
		}
		for _, vid := range draft.VideoIDs {
			item := tgbotapi.NewInputMediaVideo(tgbotapi.FileID(vid))
			if first {
				caption := header
				if draft.Text != "" {
					caption = fmt.Sprintf("%s\n\n%s", header, draft.Text)
				}
				item.Caption = caption
				first = false
			}
			mediaGroup = append(mediaGroup, item)
		}

		mg := tgbotapi.NewMediaGroup(chatID, mediaGroup)
		bot.Send(mg)

		// Send inline keyboard as a separate text message
		btnMsg := tgbotapi.NewMessage(chatID, "Отправить это сообщение?")
		btnMsg.ReplyMarkup = keyboard
		bot.Send(btnMsg)
		return
	}

	// Single photo
	if len(draft.PhotoIDs) == 1 {
		caption := header
		if draft.Text != "" {
			caption = fmt.Sprintf("%s\n\n%s", header, draft.Text)
		}
		ph := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(draft.PhotoIDs[0]))
		ph.Caption = caption
		ph.ReplyMarkup = keyboard
		bot.Send(ph)
		return
	}

	// Single video
	if len(draft.VideoIDs) == 1 {
		caption := header
		if draft.Text != "" {
			caption = fmt.Sprintf("%s\n\n%s", header, draft.Text)
		}
		v := tgbotapi.NewVideo(chatID, tgbotapi.FileID(draft.VideoIDs[0]))
		v.Caption = caption
		v.ReplyMarkup = keyboard
		bot.Send(v)
		return
	}

	// Text only
	text := fmt.Sprintf("%s\n\n%s", header, draft.Text)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// ── Confirm / Cancel callbacks (called from admin.go dispatcher) ───────────

func handleConfirmSend(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	userID := callback.From.ID
	chatID := callback.Message.Chat.ID

	draft := getDraft(userID)
	if draft == nil || getState(userID) != StateWaitingConfirm {
		answer := tgbotapi.NewCallback(callback.ID, "Нечего отправлять")
		bot.Send(answer)
		return
	}

	username := callback.From.UserName
	anonNum, err := SaveMessage(userID, username)
	if err != nil {
		log.Printf("SaveMessage error: %v", err)
		answer := tgbotapi.NewCallback(callback.ID, "Ошибка, попробуйте снова")
		bot.Send(answer)
		return
	}

	if err := IncreaseLimit(userID); err != nil {
		log.Printf("IncreaseLimit error: %v", err)
	}

	// Send to admin
	sendDraftToAdmin(bot, draft, anonNum)

	// Clean up
	deleteDraft(userID)
	setState(userID, StateIdle)
	setCooldown(userID)

	// Delete the preview message and send a fresh confirmation
	// (EditMessageText fails on photo/video messages — Telegram API limitation)
	del := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
	bot.Request(del)

	confirmMsg := tgbotapi.NewMessage(chatID, "✅ Сообщение отправлено!")
	confirmMsg.ReplyMarkup = UserKeyboard()
	bot.Send(confirmMsg)

	answer := tgbotapi.NewCallback(callback.ID, "Отправлено ✓")
	bot.Send(answer)
}

func handleCancelSend(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	userID := callback.From.ID
	chatID := callback.Message.Chat.ID

	deleteDraft(userID)
	setState(userID, StateIdle)

	// Delete the preview message and send a fresh cancellation
	// (EditMessageText fails on photo/video messages — Telegram API limitation)
	del := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
	bot.Request(del)

	cancelMsg := tgbotapi.NewMessage(chatID, "❌ Сообщение отменено.")
	cancelMsg.ReplyMarkup = UserKeyboard()
	bot.Send(cancelMsg)

	answer := tgbotapi.NewCallback(callback.ID, "Отменено")
	bot.Send(answer)
}

// ── Cooldown helpers ──────────────────────────────────────────────────────

func setCooldown(userID int64) {
	mu.Lock()
	defer mu.Unlock()
	userCooldown[userID] = time.Now()
}

func getCooldownRemaining(userID int64) time.Duration {
	mu.Lock()
	defer mu.Unlock()
	lastSend, ok := userCooldown[userID]
	if !ok {
		return 0
	}
	elapsed := time.Since(lastSend)
	if elapsed >= cooldownDuration {
		return 0
	}
	return cooldownDuration - elapsed
}

// ── Send draft to admin ────────────────────────────────────────────────────

func sendDraftToAdmin(bot *tgbotapi.BotAPI, draft *DraftMessage, anonNum int) {
	header := fmt.Sprintf("Анон #%d", anonNum)
	keyboard := BanKeyboard(anonNum)

	totalMedia := len(draft.PhotoIDs) + len(draft.VideoIDs)

	// Album
	if totalMedia > 1 {
		var mediaGroup []interface{}
		first := true
		for _, pid := range draft.PhotoIDs {
			item := tgbotapi.NewInputMediaPhoto(tgbotapi.FileID(pid))
			if first {
				caption := header
				if draft.Text != "" {
					caption = fmt.Sprintf("%s\n\n%s", header, draft.Text)
				}
				item.Caption = caption
				first = false
			}
			mediaGroup = append(mediaGroup, item)
		}
		for _, vid := range draft.VideoIDs {
			item := tgbotapi.NewInputMediaVideo(tgbotapi.FileID(vid))
			if first {
				caption := header
				if draft.Text != "" {
					caption = fmt.Sprintf("%s\n\n%s", header, draft.Text)
				}
				item.Caption = caption
				first = false
			}
			mediaGroup = append(mediaGroup, item)
		}

		mg := tgbotapi.NewMediaGroup(adminID, mediaGroup)
		bot.Send(mg)

		// Ban keyboard as separate message
		btnMsg := tgbotapi.NewMessage(adminID, header)
		btnMsg.ReplyMarkup = keyboard
		bot.Send(btnMsg)
		return
	}

	// Single photo
	if len(draft.PhotoIDs) == 1 {
		caption := header
		if draft.Text != "" {
			caption = fmt.Sprintf("%s\n\n%s", header, draft.Text)
		}
		ph := tgbotapi.NewPhoto(adminID, tgbotapi.FileID(draft.PhotoIDs[0]))
		ph.Caption = caption
		ph.ReplyMarkup = keyboard
		bot.Send(ph)
		return
	}

	// Single video
	if len(draft.VideoIDs) == 1 {
		caption := header
		if draft.Text != "" {
			caption = fmt.Sprintf("%s\n\n%s", header, draft.Text)
		}
		v := tgbotapi.NewVideo(adminID, tgbotapi.FileID(draft.VideoIDs[0]))
		v.Caption = caption
		v.ReplyMarkup = keyboard
		bot.Send(v)
		return
	}

	// Text only
	text := fmt.Sprintf("%s\n\n%s", header, draft.Text)
	msg := tgbotapi.NewMessage(adminID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}
