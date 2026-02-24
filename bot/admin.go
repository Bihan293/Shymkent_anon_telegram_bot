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
	}
}

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

	if IsSubscribed(bot, userID) {
		// Delete subscription message
		del := tgbotapi.NewDeleteMessage(chatID, callback.Message.MessageID)
		bot.Request(del)

		// Send welcome message with user keyboard
		text := "Анонимные сообщения администратору.\nОсновной канал: https://t.me/shymkent_anon"
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ReplyMarkup = UserKeyboard()
		bot.Send(msg)

		setState(userID, StateIdle)

		answer := tgbotapi.NewCallback(callback.ID, "✅ Подписка подтверждена!")
		bot.Send(answer)
	} else {
		answer := tgbotapi.NewCallback(callback.ID, "❌ Вы не подписаны на канал!")
		bot.Send(answer)
	}
}
