package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ── State Machine ──────────────────────────────────────────────────────────
var (
	userStates      = make(map[int64]string)
	userDrafts      = make(map[int64]*DraftMessage)
	userCooldown    = make(map[int64]time.Time) // last successful send time
	adminReplyDraft *AdminReplyDraft             // single admin reply draft
	adminMenu       = "main"                     // tracks current admin menu: "main", "panel", "broadcast", "reqsubs"
	mu              sync.Mutex
)

func setAdminMenu(menu string) {
	mu.Lock()
	defer mu.Unlock()
	adminMenu = menu
}

func getAdminMenu() string {
	mu.Lock()
	defer mu.Unlock()
	return adminMenu
}

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

func setAdminReplyDraft(d *AdminReplyDraft) {
	mu.Lock()
	defer mu.Unlock()
	adminReplyDraft = d
}

func getAdminReplyDraft() *AdminReplyDraft {
	mu.Lock()
	defer mu.Unlock()
	return adminReplyDraft
}

func deleteAdminReplyDraft() {
	mu.Lock()
	defer mu.Unlock()
	adminReplyDraft = nil
}

// ── Media-group buffer ─────────────────────────────────────────────────────
var (
	mediaBuffer = make(map[string][]tgbotapi.Message)
	mediaTimers = make(map[string]*time.Timer)
	mediaMu     sync.Mutex
)

// ── Validation helpers ─────────────────────────────────────────────────────

func validateDraftLimits(draft *DraftMessage, lang string) string {
	if len(draft.PhotoIDs) > MaxPhotos {
		return fmt.Sprintf(t(lang, "too_many_photos"), MaxPhotos, len(draft.PhotoIDs))
	}
	if len(draft.VideoIDs) > MaxVideos {
		return fmt.Sprintf(t(lang, "too_many_videos"), MaxVideos, len(draft.VideoIDs))
	}
	textLen := len([]rune(draft.Text))
	if textLen > MaxTextLength {
		return fmt.Sprintf(t(lang, "text_too_long"), MaxTextLength, textLen)
	}
	return ""
}

// ── Subscription check ─────────────────────────────────────────────────────

// IsSubscribed checks subscription on every required channel stored in DB.
// If there are no required channels — returns true (no subscription needed).
// Admin is always considered subscribed (so admin panel works regardless).
func IsSubscribed(bot *tgbotapi.BotAPI, userID int64) bool {
	if userID == adminID {
		return true
	}

	channels, err := ListRequiredChannels()
	if err != nil {
		log.Printf("ListRequiredChannels error: %v", err)
		// On DB error: don't block users
		return true
	}
	if len(channels) == 0 {
		return true
	}

	for _, ch := range channels {
		if !isMemberOfChannel(bot, ch.ChatID, userID) {
			return false
		}
	}
	return true
}

// isMemberOfChannel checks whether userID is in the channel identified by chatRef.
// chatRef can be a numeric chat_id (e.g. -1001234567890) or @username.
func isMemberOfChannel(bot *tgbotapi.BotAPI, chatRef string, userID int64) bool {
	var cfg tgbotapi.GetChatMemberConfig
	if id, err := strconv.ParseInt(chatRef, 10, 64); err == nil {
		cfg = tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: id,
				UserID: userID,
			},
		}
	} else {
		uname := chatRef
		if !strings.HasPrefix(uname, "@") {
			uname = "@" + uname
		}
		cfg = tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				SuperGroupUsername: uname,
				UserID:             userID,
			},
		}
	}

	member, err := bot.GetChatMember(cfg)
	if err != nil {
		log.Printf("GetChatMember error for user %d in %s: %v", userID, chatRef, err)
		// If we can't verify (bot not admin etc.) — treat as not subscribed
		return false
	}

	switch member.Status {
	case "creator", "administrator", "member":
		return true
	default:
		return false
	}
}

// EnforceSubscription returns true when the user is subscribed (or no channels configured).
// Otherwise it sends them the subscription prompt and returns false.
func EnforceSubscription(bot *tgbotapi.BotAPI, chatID, userID int64, lang string) bool {
	if IsSubscribed(bot, userID) {
		return true
	}
	sendSubscriptionMessage(bot, chatID, lang)
	return false
}

func sendSubscriptionMessage(bot *tgbotapi.BotAPI, chatID int64, lang string) {
	channels, _ := ListRequiredChannels()
	if len(channels) == 0 {
		// Nothing configured — nothing to show.
		return
	}

	// Custom admin-defined message has priority over default i18n one.
	text, _ := GetSetting(SettingSubscribeMessage)
	if strings.TrimSpace(text) == "" {
		text = t(lang, "subscribe_required")
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = SubscriptionKeyboard(lang, channels)
	bot.Send(msg)
}

// ── sendWelcome sends the main welcome message with inline+reply keyboards ──

func sendWelcome(bot *tgbotapi.BotAPI, chatID int64, lang string, isAdmin bool) {
	text := t(lang, "welcome")
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = WelcomeInlineKeyboard(lang)
	bot.Send(msg)

	// Also set the reply keyboard
	if isAdmin {
		kbMsg := tgbotapi.NewMessage(chatID, "👇")
		kbMsg.ReplyMarkup = AdminKeyboard()
		bot.Send(kbMsg)
	} else {
		kbMsg := tgbotapi.NewMessage(chatID, "👇")
		kbMsg.ReplyMarkup = UserKeyboard(lang)
		bot.Send(kbMsg)
	}
}

// ── sendWelcomeCompact sends the welcome text + reply keyboard in one message

func sendWelcomeCompact(bot *tgbotapi.BotAPI, chatID int64, lang string, isAdmin bool) {
	text := t(lang, "welcome")
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if isAdmin {
		msg.ReplyMarkup = AdminKeyboard()
	} else {
		msg.ReplyMarkup = UserKeyboard(lang)
	}
	bot.Send(msg)
}

// ── Handlers ───────────────────────────────────────────────────────────────

func HandleStart(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID
	setState(userID, StateIdle)
	deleteDraft(userID)

	// Check channel subscription
	if !IsSubscribed(bot, userID) {
		lang := getUserLang(userID)
		sendSubscriptionMessage(bot, message.Chat.ID, lang)
		return
	}

	// If language is not chosen yet, ask user to pick one
	lang := getUserLang(userID)
	if lang == "" {
		setState(userID, StateChoosingLanguage)
		msg := tgbotapi.NewMessage(message.Chat.ID, "🌐 Выберите язык / Тілді таңдаңыз:")
		msg.ReplyMarkup = LanguageKeyboard()
		// Remove reply keyboard while choosing language
		bot.Send(msg)
		return
	}

	isAdmin := userID == adminID
	sendWelcome(bot, message.Chat.ID, lang, isAdmin)
}

func HandleCreateMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID
	lang := getUserLang(userID)

	// Check channel subscription
	if !IsSubscribed(bot, userID) {
		sendSubscriptionMessage(bot, message.Chat.ID, lang)
		return
	}

	banned, err := IsBanned(userID)
	if err != nil {
		log.Printf("IsBanned error: %v", err)
		return
	}
	if banned {
		msg := tgbotapi.NewMessage(message.Chat.ID, t(lang, "banned"))
		bot.Send(msg)
		return
	}

	// Check 5-minute cooldown
	if remaining := getCooldownRemaining(userID); remaining > 0 {
		minutes := int(remaining.Minutes())
		seconds := int(remaining.Seconds()) % 60
		text := fmt.Sprintf(t(lang, "cooldown"), minutes, seconds)
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
		msg := tgbotapi.NewMessage(message.Chat.ID, t(lang, "limit_reached"))
		bot.Send(msg)
		return
	}

	remaining := 3 - count
	text := fmt.Sprintf(t(lang, "remaining"), remaining)
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = "Markdown"
	// Show cancel keyboard (hide main menu)
	msg.ReplyMarkup = CancelKeyboard(lang)
	bot.Send(msg)

	setState(userID, StateWaitingContent)
}

// HandleHelp sends detailed help/instructions.
func HandleHelp(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID
	lang := getUserLang(userID)

	text := t(lang, "help")
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = "Markdown"
	if userID == adminID {
		msg.ReplyMarkup = AdminKeyboard()
	} else {
		msg.ReplyMarkup = UserKeyboard(lang)
	}
	bot.Send(msg)
}

// HandleChangeLang shows the language picker inline keyboard.
func HandleChangeLang(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID
	lang := getUserLang(userID)

	msg := tgbotapi.NewMessage(message.Chat.ID, t(lang, "choose_lang"))
	msg.ReplyMarkup = LanguageKeyboard()
	bot.Send(msg)
}

// HandleAdminReplyMessage processes content from admin when building a reply.
func HandleAdminReplyMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From.ID != adminID {
		return
	}

	state := getState(adminID)
	if state != StateAdminReplyContent {
		return
	}

	draft := getAdminReplyDraft()
	if draft == nil {
		setState(adminID, StateIdle)
		return
	}

	// Handle media group for admin reply
	if message.MediaGroupID != "" {
		handleAdminReplyMediaGroup(bot, message)
		return
	}

	// Collect content
	if message.Photo != nil {
		best := message.Photo[len(message.Photo)-1]
		draft.PhotoIDs = append(draft.PhotoIDs, best.FileID)
	}
	if message.Video != nil {
		draft.VideoIDs = append(draft.VideoIDs, message.Video.FileID)
	}

	if message.Caption != "" {
		draft.Text = message.Caption
	} else if message.Text != "" {
		draft.Text = message.Text
	}

	if draft.Text == "" && len(draft.PhotoIDs) == 0 && len(draft.VideoIDs) == 0 {
		return
	}

	setAdminReplyDraft(draft)
	setState(adminID, StateAdminReplyConfirm)
	sendAdminReplyPreview(bot, message.Chat.ID, draft)
}

// ── Admin reply media-group ───────────────────────────────────────────────

var (
	adminMediaBuffer = make(map[string][]tgbotapi.Message)
	adminMediaTimers = make(map[string]*time.Timer)
	adminMediaMu     sync.Mutex
)

func handleAdminReplyMediaGroup(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	groupID := message.MediaGroupID
	chatID := message.Chat.ID

	adminMediaMu.Lock()
	adminMediaBuffer[groupID] = append(adminMediaBuffer[groupID], *message)

	if t, ok := adminMediaTimers[groupID]; ok {
		t.Stop()
	}
	adminMediaTimers[groupID] = time.AfterFunc(1*time.Second, func() {
		adminMediaMu.Lock()
		messages := adminMediaBuffer[groupID]
		delete(adminMediaBuffer, groupID)
		delete(adminMediaTimers, groupID)
		adminMediaMu.Unlock()

		draft := getAdminReplyDraft()
		if draft == nil {
			return
		}

		for _, m := range messages {
			if m.Photo != nil {
				best := m.Photo[len(m.Photo)-1]
				draft.PhotoIDs = append(draft.PhotoIDs, best.FileID)
			}
			if m.Video != nil {
				draft.VideoIDs = append(draft.VideoIDs, m.Video.FileID)
			}
			if draft.Text == "" && m.Caption != "" {
				draft.Text = m.Caption
			}
		}

		setAdminReplyDraft(draft)
		setState(adminID, StateAdminReplyConfirm)
		sendAdminReplyPreview(bot, chatID, draft)
	})
	adminMediaMu.Unlock()
}

// ── Admin reply preview ───────────────────────────────────────────────────

func sendAdminReplyPreview(bot *tgbotapi.BotAPI, chatID int64, draft *AdminReplyDraft) {
	header := fmt.Sprintf("📨 Ответ для Анон #%d — предпросмотр:", draft.AnonNumber)
	keyboard := ConfirmAdminReplyKeyboard()

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

		mg := tgbotapi.NewMediaGroup(chatID, mediaGroup)
		bot.Send(mg)

		btnMsg := tgbotapi.NewMessage(chatID, "Отправить это сообщение пользователю?")
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

// ── Statistics handler ────────────────────────────────────────────────────

func HandleStatistics(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From.ID != adminID {
		return
	}

	totalUsers, _ := GetTotalUsers()
	totalMessages, _ := GetTotalMessages()
	totalBans, _ := GetTotalBans()
	todayMessages, _ := GetTodayMessages()
	todayUsers, _ := GetTodayUsers()
	weekMessages, _ := GetWeekMessages()

	text := fmt.Sprintf(
		"📊 *Статистика бота*\n\n"+
			"👥 Всего пользователей: *%d*\n"+
			"📨 Всего сообщений: *%d*\n"+
			"🚫 Забанено: *%d*\n\n"+
			"📅 *Сегодня:*\n"+
			"   📨 Сообщений: *%d*\n"+
			"   👥 Активных пользователей: *%d*\n\n"+
			"📆 *За неделю:*\n"+
			"   📨 Сообщений: *%d*\n",
		totalUsers,
		totalMessages,
		totalBans,
		todayMessages,
		todayUsers,
		weekMessages,
	)

	topUserID, topCount, err := GetTopUser()
	if err == nil && topUserID != 0 {
		text += fmt.Sprintf("\n🏆 *Топ отправитель:*\n   🆔 %d — %d сообщений\n", topUserID, topCount)
	}

	lastTime, err := GetLastMessageTime()
	if err == nil && lastTime != nil {
		text += fmt.Sprintf("\n🕐 Последнее сообщение: %s", lastTime.Format("02.01.2006 15:04"))
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

// HandleUserMessage processes incoming content when user is in WAITING_CONTENT.
func HandleUserMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID
	lang := getUserLang(userID)

	// If admin is in reply mode, handle that instead
	if userID == adminID {
		state := getState(adminID)
		if state == StateAdminReplyContent || state == StateAdminReplyConfirm {
			HandleAdminReplyMessage(bot, message)
			return
		}
	}

	if getState(userID) != StateWaitingContent {
		return
	}

	// Check channel subscription
	if !IsSubscribed(bot, userID) {
		setState(userID, StateIdle)
		sendSubscriptionMessage(bot, message.Chat.ID, lang)
		return
	}

	// Check ban again
	banned, _ := IsBanned(userID)
	if banned {
		msg := tgbotapi.NewMessage(message.Chat.ID, t(lang, "banned"))
		setState(userID, StateIdle)
		bot.Send(msg)
		return
	}

	// Check limit again
	count, _ := CheckLimit(userID)
	if count >= 3 {
		msg := tgbotapi.NewMessage(message.Chat.ID, t(lang, "limit_reached"))
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
	if errMsg := validateDraftLimits(draft, lang); errMsg != "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, errMsg)
		bot.Send(msg)
		return
	}

	setDraft(userID, draft)
	setState(userID, StateWaitingConfirm)
	sendPreview(bot, message.Chat.ID, draft, lang)
}

// ── Media-group logic ──────────────────────────────────────────────────────

func handleMediaGroup(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	groupID := message.MediaGroupID
	userID := message.From.ID
	chatID := message.Chat.ID
	lang := getUserLang(userID)

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
		if errMsg := validateDraftLimits(draft, lang); errMsg != "" {
			msg := tgbotapi.NewMessage(chatID, errMsg)
			bot.Send(msg)
			return
		}

		setDraft(userID, draft)
		setState(userID, StateWaitingConfirm)
		sendPreview(bot, chatID, draft, lang)
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

func sendPreview(bot *tgbotapi.BotAPI, chatID int64, draft *DraftMessage, lang string) {
	header := t(lang, "preview_header")
	keyboard := ConfirmSendKeyboard(lang)

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
		btnMsg := tgbotapi.NewMessage(chatID, t(lang, "confirm_question"))
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
	lang := getUserLang(userID)

	draft := getDraft(userID)
	if draft == nil || getState(userID) != StateWaitingConfirm {
		answer := tgbotapi.NewCallback(callback.ID, t(lang, "nothing_to_send"))
		bot.Send(answer)
		return
	}

	username := callback.From.UserName
	anonNum, err := SaveMessage(userID, username)
	if err != nil {
		log.Printf("SaveMessage error: %v", err)
		answer := tgbotapi.NewCallback(callback.ID, t(lang, "error_try_again"))
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

	// Delete the preview message
	del := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
	bot.Request(del)

	// Send success + show welcome again
	isAdmin := userID == adminID
	okMsg := tgbotapi.NewMessage(chatID, t(lang, "sent_ok"))
	if isAdmin {
		okMsg.ReplyMarkup = AdminKeyboard()
	} else {
		okMsg.ReplyMarkup = UserKeyboard(lang)
	}
	bot.Send(okMsg)

	// Send welcome with inline button
	welcomeMsg := tgbotapi.NewMessage(chatID, t(lang, "welcome"))
	welcomeMsg.ParseMode = "Markdown"
	welcomeMsg.ReplyMarkup = WelcomeInlineKeyboard(lang)
	bot.Send(welcomeMsg)

	answer := tgbotapi.NewCallback(callback.ID, t(lang, "sent_callback"))
	bot.Send(answer)
}

func handleCancelSend(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	userID := callback.From.ID
	chatID := callback.Message.Chat.ID
	lang := getUserLang(userID)

	deleteDraft(userID)
	setState(userID, StateIdle)

	// Delete the preview message
	del := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
	bot.Request(del)

	// Send cancellation + show welcome again
	isAdmin := userID == adminID
	cancelMsg := tgbotapi.NewMessage(chatID, t(lang, "cancelled"))
	if isAdmin {
		cancelMsg.ReplyMarkup = AdminKeyboard()
	} else {
		cancelMsg.ReplyMarkup = UserKeyboard(lang)
	}
	bot.Send(cancelMsg)

	// Send welcome with inline button
	welcomeMsg := tgbotapi.NewMessage(chatID, t(lang, "welcome"))
	welcomeMsg.ParseMode = "Markdown"
	welcomeMsg.ReplyMarkup = WelcomeInlineKeyboard(lang)
	bot.Send(welcomeMsg)

	answer := tgbotapi.NewCallback(callback.ID, t(lang, "cancelled_cb"))
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
