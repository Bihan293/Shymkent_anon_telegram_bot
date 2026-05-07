package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleInfo(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From.ID != adminID {
		return
	}

	parts := strings.Fields(message.Text)
	if len(parts) != 2 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Использование: /info <номер>")
		bot.Send(msg)
		return
	}

	anonNum, err := strconv.Atoi(parts[1])
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Неверный номер.")
		bot.Send(msg)
		return
	}

	msgInfo, err := GetMessageInfo(anonNum)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Сообщение не найдено.")
		bot.Send(msg)
		return
	}

	banned, _ := IsBanned(msgInfo.UserID)
	banStatus := "нет"
	if banned {
		banStatus = "да"
	}

	todayCount, _ := TodayMessageCount(msgInfo.UserID)

	text := fmt.Sprintf(
		"📋 Анон #%d\n\n👤 Username: @%s\n🆔 User ID: %d\n📅 Дата: %s\n📨 Сообщений сегодня: %d\n🚫 Бан: %s",
		msgInfo.AnonNumber,
		msgInfo.Username,
		msgInfo.UserID,
		msgInfo.CreatedAt.Format("02.01.2006 15:04"),
		todayCount,
		banStatus,
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyMarkup = InfoKeyboard(anonNum)
	bot.Send(msg)
}

// HandleCallback is the single entry-point for ALL callback queries.
// It routes user-facing callbacks (confirm_send / cancel_send) and admin callbacks.
func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	data := callback.Data

	// ── Language selection ────────────────────────────────────────────
	if strings.HasPrefix(data, "lang:") {
		handleLangSelection(bot, callback)
		return
	}

	// ── Inline "send anon" button from welcome message ───────────────
	if data == "start_anon" {
		handleStartAnon(bot, callback)
		return
	}

	// ── User confirm/cancel / subscription check ──────────────────────
	switch data {
	case "confirm_send":
		handleConfirmSend(bot, callback)
		return
	case "cancel_send":
		handleCancelSend(bot, callback)
		return
	case "check_subscription":
		handleCheckSubscription(bot, callback)
		return
	case "confirm_admin_reply":
		if callback.From.ID == adminID {
			handleConfirmAdminReply(bot, callback)
		}
		return
	case "cancel_admin_reply":
		if callback.From.ID == adminID {
			handleCancelAdminReply(bot, callback)
		}
		return
	}

	// ── Admin-only callbacks below ─────────────────────────────────────
	if callback.From.ID != adminID {
		answer := tgbotapi.NewCallback(callback.ID, "Нет доступа")
		bot.Send(answer)
		return
	}

	switch {
	case strings.HasPrefix(data, "ban:"):
		handleBanRequest(bot, callback)

	case strings.HasPrefix(data, "confirm_ban:"):
		handleConfirmBan(bot, callback)

	case strings.HasPrefix(data, "cancel_ban:"):
		handleCancelBan(bot, callback)

	case strings.HasPrefix(data, "unban:"):
		handleUnban(bot, callback)

	case strings.HasPrefix(data, "reply:"):
		handleReplyStart(bot, callback)

	case data == "bcast_confirm":
		handleBroadcastConfirm(bot, callback)

	case data == "bcast_cancel":
		handleBroadcastCancelCb(bot, callback)

	case strings.HasPrefix(data, "chan_select:"):
		handleChannelSelect(bot, callback)

	case strings.HasPrefix(data, "chan_remove:"):
		handleChannelRemove(bot, callback)

	case strings.HasPrefix(data, "reqchan_remove:"):
		HandleRequiredChannelRemoveCallback(bot, callback)
	}
}

// ── Broadcast callback handlers ─────────────────────────

func handleBroadcastConfirm(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID

	state := getState(adminID)
	if state != StateAdminBcastUsersConfirm && state != StateAdminBcastChanConfirm {
		answer := tgbotapi.NewCallback(callback.ID, "Нет активной рассылки")
		bot.Send(answer)
		return
	}

	del := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
	bot.Request(del)

	answer := tgbotapi.NewCallback(callback.ID, "Запускаю...")
	bot.Send(answer)

	ExecuteBroadcast(bot, chatID)
}

func handleBroadcastCancelCb(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID

	del := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
	bot.Request(del)

	CancelBroadcast(bot, chatID)

	answer := tgbotapi.NewCallback(callback.ID, "Отменено")
	bot.Send(answer)
}

func handleChannelSelect(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID

	parts := strings.SplitN(callback.Data, ":", 2)
	if len(parts) != 2 {
		return
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}
	ch, err := GetChannelByID(id)
	if err != nil {
		answer := tgbotapi.NewCallback(callback.ID, "Канал не найден")
		bot.Send(answer)
		return
	}

	setBroadcastDraft(&BroadcastDraft{
		Target:    BroadcastChannel,
		ChannelID: ch.ChatID,
	})
	setState(adminID, StateAdminBcastChanContent)

	del := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
	bot.Request(del)

	title := ch.Title
	if title == "" {
		title = ch.ChatID
	}
	text := fmt.Sprintf(
		"📢 *Рассылка в канал «%s»*\n"+
			"━━━━━━━━━━━━━━━━━━━━\n\n"+
			"📤 *Отправьте сообщение для публикации:*\n"+
			"• Текст\n"+
			"• Фото (до 8 шт.)\n"+
			"• Видео (до 3 шт.)\n"+
			"• Альбом с подписью\n\n"+
			"💡 _Как только пришлёте контент — сразу появится предпросмотр._\n\n"+
			"❌ Чтобы отменить — нажмите кнопку ниже.", title)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminCancelOnlyKeyboard()
	bot.Send(msg)

	answer := tgbotapi.NewCallback(callback.ID, "Канал выбран")
	bot.Send(answer)
}

func handleChannelRemove(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID

	parts := strings.SplitN(callback.Data, ":", 2)
	if len(parts) != 2 {
		return
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}
	ch, err := GetChannelByID(id)
	if err != nil {
		answer := tgbotapi.NewCallback(callback.ID, "Канал не найден")
		bot.Send(answer)
		return
	}

	if err := RemoveChannel(ch.ChatID); err != nil {
		answer := tgbotapi.NewCallback(callback.ID, "Ошибка удаления")
		bot.Send(answer)
		return
	}

	channels, _ := ListChannels()
	if len(channels) == 0 {
		edit := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID,
			"📭 Список каналов пуст.")
		bot.Send(edit)
	} else {
		var sb strings.Builder
		sb.WriteString("📋 *Список каналов:*\n\n")
		for i, c := range channels {
			title := c.Title
			if title == "" {
				title = "(без названия)"
			}
			sb.WriteString(fmt.Sprintf("%d. *%s*\n   `%s`\n\n", i+1, title, c.ChatID))
		}
		sb.WriteString("Нажмите на канал чтобы удалить его:")

		kb := ChannelRemoveKeyboard(channels)
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, callback.Message.MessageID,
			sb.String(), kb)
		edit.ParseMode = "Markdown"
		bot.Send(edit)
	}

	answer := tgbotapi.NewCallback(callback.ID, "Канал удален")
	bot.Send(answer)
}

// ── Language selection callback ─────────────────────────

// ── Language selection callback ──────────────────────────────────────────────

func handleLangSelection(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	userID := callback.From.ID
	chatID := callback.Message.Chat.ID

	parts := strings.SplitN(callback.Data, ":", 2)
	if len(parts) != 2 {
		return
	}
	lang := parts[1]
	if lang != LangRU && lang != LangKZ {
		return
	}

	setUserLang(userID, lang)

	// Delete the language picker message
	del := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
	bot.Request(del)

	// If user was in CHOOSING_LANGUAGE state (first time), show welcome
	state := getState(userID)
	if state == StateChoosingLanguage {
		setState(userID, StateIdle)

		// Check subscription after language choice
		if !IsSubscribed(bot, userID) {
			sendSubscriptionMessage(bot, chatID, lang)
			answer := tgbotapi.NewCallback(callback.ID, "")
			bot.Send(answer)
			return
		}

		isAdmin := userID == adminID
		sendWelcome(bot, chatID, lang, isAdmin)

		answer := tgbotapi.NewCallback(callback.ID, "")
		bot.Send(answer)
		return
	}

	// Otherwise it's a language change from the menu
	setState(userID, StateIdle)

	// Send confirmation + welcome
	confirmMsg := tgbotapi.NewMessage(chatID, t(lang, "lang_changed"))
	confirmMsg.ParseMode = "Markdown"
	if userID == adminID {
		confirmMsg.ReplyMarkup = AdminKeyboard()
	} else {
		confirmMsg.ReplyMarkup = UserKeyboard(lang)
	}
	bot.Send(confirmMsg)

	// Show welcome with inline
	welcomeMsg := tgbotapi.NewMessage(chatID, t(lang, "welcome"))
	welcomeMsg.ParseMode = "Markdown"
	welcomeMsg.ReplyMarkup = WelcomeInlineKeyboard(lang)
	bot.Send(welcomeMsg)

	answer := tgbotapi.NewCallback(callback.ID, "")
	bot.Send(answer)
}

// ── Inline start_anon callback ──────────────────────────────────────────────

func handleStartAnon(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	userID := callback.From.ID
	chatID := callback.Message.Chat.ID
	lang := getUserLang(userID)

	// Build a fake message to reuse HandleCreateMessage logic
	fakeMsg := &tgbotapi.Message{
		From: callback.From,
		Chat: callback.Message.Chat,
	}

	// Check subscription
	if !IsSubscribed(bot, userID) {
		sendSubscriptionMessage(bot, chatID, lang)
		answer := tgbotapi.NewCallback(callback.ID, "")
		bot.Send(answer)
		return
	}

	answer := tgbotapi.NewCallback(callback.ID, "")
	bot.Send(answer)

	HandleCreateMessage(bot, fakeMsg)
}

// ── Ban/Unban handlers ──────────────────────────────────────────────────────

func handleBanRequest(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	anonNum := parseAnonNumber(callback.Data)
	if anonNum == 0 {
		return
	}

	// Only update the keyboard — never touch message text/caption/media
	keyboard := ConfirmBanKeyboard(anonNum)
	editMarkup := tgbotapi.NewEditMessageReplyMarkup(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		keyboard,
	)
	bot.Send(editMarkup)

	answer := tgbotapi.NewCallback(callback.ID, fmt.Sprintf("Забанить автора #%d?", anonNum))
	bot.Send(answer)
}

func handleConfirmBan(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	anonNum := parseAnonNumber(callback.Data)
	if anonNum == 0 {
		return
	}

	msgInfo, err := GetMessageInfo(anonNum)
	if err != nil {
		answer := tgbotapi.NewCallback(callback.ID, "Сообщение не найдено")
		bot.Send(answer)
		return
	}

	if err := BanUser(msgInfo.UserID); err != nil {
		log.Printf("BanUser error: %v", err)
		answer := tgbotapi.NewCallback(callback.ID, "Ошибка бана")
		bot.Send(answer)
		return
	}

	// Only update the keyboard — never touch message text/caption/media
	keyboard := UnbanKeyboard(anonNum)
	editMarkup := tgbotapi.NewEditMessageReplyMarkup(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		keyboard,
	)
	bot.Send(editMarkup)

	// Notify the banned user
	banNotice := tgbotapi.NewMessage(msgInfo.UserID, "⛔ Вы были заблокированы администратором. Вы больше не можете отправлять анонимные сообщения.")
	bot.Send(banNotice)

	answer := tgbotapi.NewCallback(callback.ID, fmt.Sprintf("🔒 Забанен (Анон #%d)", anonNum))
	bot.Send(answer)
}

func handleCancelBan(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	anonNum := parseAnonNumber(callback.Data)
	if anonNum == 0 {
		return
	}

	// Only update the keyboard — never touch message text/caption/media
	keyboard := BanKeyboard(anonNum)
	editMarkup := tgbotapi.NewEditMessageReplyMarkup(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		keyboard,
	)
	bot.Send(editMarkup)

	answer := tgbotapi.NewCallback(callback.ID, "Отменено")
	bot.Send(answer)
}

func handleUnban(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	anonNum := parseAnonNumber(callback.Data)
	if anonNum == 0 {
		return
	}

	msgInfo, err := GetMessageInfo(anonNum)
	if err != nil {
		answer := tgbotapi.NewCallback(callback.ID, "Сообщение не найдено")
		bot.Send(answer)
		return
	}

	if err := UnbanUser(msgInfo.UserID); err != nil {
		log.Printf("UnbanUser error: %v", err)
		answer := tgbotapi.NewCallback(callback.ID, "Ошибка разбана")
		bot.Send(answer)
		return
	}

	// Only update the keyboard — never touch message text/caption/media
	keyboard := BanKeyboard(anonNum)
	editMarkup := tgbotapi.NewEditMessageReplyMarkup(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		keyboard,
	)
	bot.Send(editMarkup)

	// Notify the unbanned user
	unbanNotice := tgbotapi.NewMessage(msgInfo.UserID, "✅ Вы были разблокированы. Теперь вы снова можете отправлять анонимные сообщения.")
	bot.Send(unbanNotice)

	answer := tgbotapi.NewCallback(callback.ID, fmt.Sprintf("✅ Разбанен (Анон #%d)", anonNum))
	bot.Send(answer)
}

// ── Admin Reply to Anonymous User ─────────────────────────────────────────

func handleReplyStart(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	anonNum := parseAnonNumber(callback.Data)
	if anonNum == 0 {
		return
	}

	msgInfo, err := GetMessageInfo(anonNum)
	if err != nil {
		answer := tgbotapi.NewCallback(callback.ID, "Сообщение не найдено")
		bot.Send(answer)
		return
	}

	// Set admin into reply mode
	draft := &AdminReplyDraft{
		TargetUserID: msgInfo.UserID,
		AnonNumber:   anonNum,
	}
	setAdminReplyDraft(draft)
	setState(adminID, StateAdminReplyContent)

	text := fmt.Sprintf("💬 Ответ для Анон #%d\n\nОтправьте сообщение (текст, фото, видео) которое хотите переслать этому пользователю.\n\nНапишите /cancel чтобы отменить.", anonNum)
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, text)
	bot.Send(msg)

	answer := tgbotapi.NewCallback(callback.ID, fmt.Sprintf("Ответ для #%d", anonNum))
	bot.Send(answer)
}

func handleConfirmAdminReply(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID

	draft := getAdminReplyDraft()
	if draft == nil || getState(adminID) != StateAdminReplyConfirm {
		answer := tgbotapi.NewCallback(callback.ID, "Нечего отправлять")
		bot.Send(answer)
		return
	}

	// Send the reply to the user
	sendReplyToUser(bot, draft)

	// Clean up
	deleteAdminReplyDraft()
	setState(adminID, StateIdle)

	// Delete preview message
	del := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
	bot.Request(del)

	confirmMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Сообщение отправлено пользователю (Анон #%d)!", draft.AnonNumber))
	bot.Send(confirmMsg)

	answer := tgbotapi.NewCallback(callback.ID, "Отправлено ✓")
	bot.Send(answer)
}

func handleCancelAdminReply(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID

	deleteAdminReplyDraft()
	setState(adminID, StateIdle)

	// Delete preview message
	del := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
	bot.Request(del)

	cancelMsg := tgbotapi.NewMessage(chatID, "❌ Ответ отменён.")
	bot.Send(cancelMsg)

	answer := tgbotapi.NewCallback(callback.ID, "Отменено")
	bot.Send(answer)
}

// sendReplyToUser sends the admin's composed reply to the anonymous user.
func sendReplyToUser(bot *tgbotapi.BotAPI, draft *AdminReplyDraft) {
	header := "📩 Сообщение от администратора:"
	targetID := draft.TargetUserID
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

		mg := tgbotapi.NewMediaGroup(targetID, mediaGroup)
		bot.Send(mg)
		return
	}

	// Single photo
	if len(draft.PhotoIDs) == 1 {
		caption := header
		if draft.Text != "" {
			caption = fmt.Sprintf("%s\n\n%s", header, draft.Text)
		}
		ph := tgbotapi.NewPhoto(targetID, tgbotapi.FileID(draft.PhotoIDs[0]))
		ph.Caption = caption
		bot.Send(ph)
		return
	}

	// Single video
	if len(draft.VideoIDs) == 1 {
		caption := header
		if draft.Text != "" {
			caption = fmt.Sprintf("%s\n\n%s", header, draft.Text)
		}
		v := tgbotapi.NewVideo(targetID, tgbotapi.FileID(draft.VideoIDs[0]))
		v.Caption = caption
		bot.Send(v)
		return
	}

	// Text only
	text := fmt.Sprintf("%s\n\n%s", header, draft.Text)
	msg := tgbotapi.NewMessage(targetID, text)
	bot.Send(msg)
}

// ── Helpers ───────────────────────────────────────────────────────────────

func parseAnonNumber(data string) int {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}
	return n
}

func handleCheckSubscription(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	userID := callback.From.ID
	chatID := callback.Message.Chat.ID
	lang := getUserLang(userID)

	if IsSubscribed(bot, userID) {
		// Delete subscription message
		del := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
		bot.Request(del)

		isAdmin := userID == adminID
		sendWelcome(bot, chatID, lang, isAdmin)

		setState(userID, StateIdle)

		answer := tgbotapi.NewCallback(callback.ID, t(lang, "sub_confirmed"))
		bot.Send(answer)
	} else {
		answer := tgbotapi.NewCallback(callback.ID, t(lang, "sub_not_confirmed"))
		bot.Send(answer)
	}
}
