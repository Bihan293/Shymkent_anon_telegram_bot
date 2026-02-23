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
	banNotice := tgbotapi.NewMessage(msgInfo.UserID, "⛔ Вы были заблокированы администратором.")
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

	answer := tgbotapi.NewCallback(callback.ID, fmt.Sprintf("✅ Разбанен (Анон #%d)", anonNum))
	bot.Send(answer)
}

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
