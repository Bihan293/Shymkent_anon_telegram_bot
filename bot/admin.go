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

func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	if callback.From.ID != adminID {
		callbackAnswer := tgbotapi.NewCallback(callback.ID, "Нет доступа")
		bot.Send(callbackAnswer)
		return
	}

	data := callback.Data

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

	text := fmt.Sprintf("Забанить автора сообщения #%d?", anonNum)
	edit := tgbotapi.NewEditMessageText(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		text,
	)
	keyboard := ConfirmBanKeyboard(anonNum)
	edit.ReplyMarkup = &keyboard
	bot.Send(edit)

	answer := tgbotapi.NewCallback(callback.ID, "")
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

	text := fmt.Sprintf("🔒 Пользователь забанен (Анон #%d)", anonNum)
	edit := tgbotapi.NewEditMessageText(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		text,
	)
	keyboard := UnbanKeyboard(anonNum)
	edit.ReplyMarkup = &keyboard
	bot.Send(edit)

	answer := tgbotapi.NewCallback(callback.ID, "Забанен")
	bot.Send(answer)
}

func handleCancelBan(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	anonNum := parseAnonNumber(callback.Data)
	if anonNum == 0 {
		return
	}

	text := fmt.Sprintf("Анон #%d — бан отменён", anonNum)
	edit := tgbotapi.NewEditMessageText(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		text,
	)
	keyboard := BanKeyboard(anonNum)
	edit.ReplyMarkup = &keyboard
	bot.Send(edit)

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

	text := fmt.Sprintf("✅ Пользователь разбанен (Анон #%d)", anonNum)
	edit := tgbotapi.NewEditMessageText(
		callback.Message.Chat.ID,
		callback.Message.MessageID,
		text,
	)
	keyboard := BanKeyboard(anonNum)
	edit.ReplyMarkup = &keyboard
	bot.Send(edit)

	answer := tgbotapi.NewCallback(callback.ID, "Разбанен")
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
