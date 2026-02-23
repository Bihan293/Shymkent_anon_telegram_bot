package main

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Состояния пользователей: ожидание сообщения
var waitingForMessage = make(map[int64]bool)

func HandleStart(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	text := "Анонимные сообщения администратору.\nОсновной канал: https://t.me/shymkent_anon"

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyMarkup = UserKeyboard()

	bot.Send(msg)
}

func HandleCreateMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID

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

	waitingForMessage[userID] = true
}

func HandleUserMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID

	if !waitingForMessage[userID] {
		return
	}

	banned, _ := IsBanned(userID)
	if banned {
		msg := tgbotapi.NewMessage(message.Chat.ID, "⛔ Вы заблокированы.")
		delete(waitingForMessage, userID)
		bot.Send(msg)
		return
	}

	count, _ := CheckLimit(userID)
	if count >= 3 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Вы исчерпали лимит сообщений на сегодня (3/3).")
		delete(waitingForMessage, userID)
		bot.Send(msg)
		return
	}

	username := message.From.UserName
	anonNum, err := SaveMessage(userID, username)
	if err != nil {
		log.Printf("SaveMessage error: %v", err)
		return
	}

	if err := IncreaseLimit(userID); err != nil {
		log.Printf("IncreaseLimit error: %v", err)
	}

	delete(waitingForMessage, userID)

	// Отправка подтверждения пользователю
	confirm := tgbotapi.NewMessage(message.Chat.ID, "Отправлено ✓")
	bot.Send(confirm)

	// Пересылка контента админу
	sendToAdmin(bot, message, anonNum)
}

func sendToAdmin(bot *tgbotapi.BotAPI, message *tgbotapi.Message, anonNum int) {
	header := fmt.Sprintf("Анон #%d", anonNum)
	keyboard := BanKeyboard(anonNum)

	if message.Photo != nil {
		photo := message.Photo[len(message.Photo)-1]
		msg := tgbotapi.NewPhoto(adminID, tgbotapi.FileID(photo.FileID))
		caption := header
		if message.Caption != "" {
			caption = fmt.Sprintf("%s\n\n%s", header, message.Caption)
		}
		msg.Caption = caption
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
		return
	}

	if message.Video != nil {
		msg := tgbotapi.NewVideo(adminID, tgbotapi.FileID(message.Video.FileID))
		caption := header
		if message.Caption != "" {
			caption = fmt.Sprintf("%s\n\n%s", header, message.Caption)
		}
		msg.Caption = caption
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
		return
	}

	if message.Text != "" {
		text := fmt.Sprintf("%s\n\n%s", header, message.Text)
		msg := tgbotapi.NewMessage(adminID, text)
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
		return
	}

	// Другие типы — просто текст
	msg := tgbotapi.NewMessage(adminID, header)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}
