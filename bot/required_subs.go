package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// OpenRequiredSubsMenu shows the mandatory-subscription management menu.
func OpenRequiredSubsMenu(bot *tgbotapi.BotAPI, chatID int64) {
	setState(adminID, StateIdle)

	channels, _ := ListRequiredChannels()
	custom, _ := GetSetting(SettingSubscribeMessage)

	var sb strings.Builder
	sb.WriteString("🔔 *Обязательная подписка*\n\n")
	if len(channels) == 0 {
		sb.WriteString("📭 Сейчас обязательной подписки *НЕТ*. Пользователи могут пользоваться ботом свободно.\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("📡 Каналов в списке: *%d*\n\n", len(channels)))
		for i, c := range channels {
			title := c.Title
			if title == "" {
				title = "(без названия)"
			}
			sb.WriteString(fmt.Sprintf("%d. *%s*\n   `%s`\n", i+1, title, c.ChatID))
			if c.InviteLink != "" {
				sb.WriteString(fmt.Sprintf("   🔗 %s\n", c.InviteLink))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("✏️ *Текст приветствия:*\n")
	if strings.TrimSpace(custom) == "" {
		sb.WriteString("_(используется текст по умолчанию)_\n\n")
	} else {
		sb.WriteString(custom + "\n\n")
	}

	sb.WriteString("Используйте кнопки ниже для управления.")

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminRequiredSubsKeyboard()
	bot.Send(msg)
}

// PromptAddRequiredChannel asks the admin to send a channel @username, id, or forward.
func PromptAddRequiredChannel(bot *tgbotapi.BotAPI, chatID int64) {
	setState(adminID, StateAdminReqChanAdd)
	text := "➕ *Добавление канала обязательной подписки*\n\n" +
		"Перешлите любое сообщение из канала, ИЛИ отправьте:\n" +
		"• `@username_канала`\n" +
		"• ID канала (например `-1001234567890`)\n" +
		"• Прямую ссылку: `https://t.me/имя_канала` или `https://t.me/+invite_hash`\n\n" +
		"⚠️ Бот должен быть *администратором* канала, иначе он не сможет проверить подписку пользователей.\n\n" +
		"Чтобы отменить — нажмите ❌ Отменить."
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminCancelOnlyKeyboard()
	bot.Send(msg)
}

// HandleAddRequiredChannelInput processes admin input when adding a required channel.
func HandleAddRequiredChannelInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID

	var chatRef string
	var title string
	var inviteLink string

	// 1. forwarded from channel
	if message.ForwardFromChat != nil && message.ForwardFromChat.IsChannel() {
		chatRef = strconv.FormatInt(message.ForwardFromChat.ID, 10)
		title = message.ForwardFromChat.Title
		if message.ForwardFromChat.UserName != "" {
			inviteLink = "https://t.me/" + message.ForwardFromChat.UserName
		}
	} else {
		raw := strings.TrimSpace(message.Text)
		if raw == "" {
			bot.Send(tgbotapi.NewMessage(chatID,
				"⚠️ Перешлите сообщение из канала, отправьте @username, id или ссылку."))
			return
		}

		// If user sent a t.me link — extract @username or +invite hash
		if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "t.me/") {
			// store the link for later
			inviteLink = raw
			// pick out the part after t.me/
			idx := strings.Index(raw, "t.me/")
			if idx == -1 {
				bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Не похоже на ссылку Telegram. Используйте https://t.me/имя или https://t.me/+invite."))
				return
			}
			tail := strings.TrimRight(raw[idx+5:], "/")
			if tail == "" {
				bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Пустая ссылка."))
				return
			}
			if strings.HasPrefix(tail, "+") || strings.HasPrefix(tail, "joinchat/") {
				// Private invite link — we cannot resolve chat id without bot being inside.
				bot.Send(tgbotapi.NewMessage(chatID,
					"❌ Это приватная ссылка. Для приватных каналов:\n"+
						"1) Добавьте бота в канал как админа\n"+
						"2) Перешлите сюда любой пост из канала\n\n"+
						"После этого канал будет добавлен и ссылка-приглашение сохранится."))
				return
			}
			// public username form
			chatRef = "@" + tail
		} else {
			chatRef = raw
		}
	}

	// Resolve chat info
	var chatCfg tgbotapi.ChatInfoConfig
	if id, err := strconv.ParseInt(chatRef, 10, 64); err == nil {
		chatCfg = tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: id}}
	} else {
		uname := chatRef
		if !strings.HasPrefix(uname, "@") {
			uname = "@" + uname
		}
		chatCfg = tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{SuperGroupUsername: uname}}
	}

	chatInfo, err := bot.GetChat(chatCfg)
	if err != nil {
		log.Printf("GetChat (required) error for %s: %v", chatRef, err)
		bot.Send(tgbotapi.NewMessage(chatID,
			"❌ Не удалось получить информацию о канале.\n"+
				"Убедитесь, что:\n"+
				"• Канал существует (для приватного — перешлите пост из него)\n"+
				"• Бот *администратор* канала (нужно для проверки подписки)\n\n"+
				"Попробуйте снова или нажмите ❌ Отменить."))
		return
	}

	// Always store as numeric id (stable for GetChatMember calls).
	storeChatID := strconv.FormatInt(chatInfo.ID, 10)
	if title == "" {
		title = chatInfo.Title
	}
	// If we don't have an invite link yet, try to build one from username.
	if inviteLink == "" && chatInfo.UserName != "" {
		inviteLink = "https://t.me/" + chatInfo.UserName
	}

	if err := AddRequiredChannel(storeChatID, title, inviteLink); err != nil {
		log.Printf("AddRequiredChannel error: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка сохранения канала в БД."))
		return
	}

	setState(adminID, StateIdle)

	doneText := fmt.Sprintf("✅ Канал добавлен в обязательную подписку:\n*%s*\n`%s`", title, storeChatID)
	if inviteLink != "" {
		doneText += "\n🔗 " + inviteLink
	} else {
		doneText += "\n\n⚠️ Не удалось определить публичную ссылку. Если канал приватный — перешлите пост из него ещё раз, и я сохраню ссылку-приглашение, либо добавьте её вручную через текст приветствия."
	}

	doneMsg := tgbotapi.NewMessage(chatID, doneText)
	doneMsg.ParseMode = "Markdown"
	doneMsg.ReplyMarkup = AdminRequiredSubsKeyboard()
	bot.Send(doneMsg)
}

// ShowRequiredChannelsList shows the list with remove inline buttons.
func ShowRequiredChannelsList(bot *tgbotapi.BotAPI, chatID int64) {
	channels, err := ListRequiredChannels()
	if err != nil {
		log.Printf("ListRequiredChannels error: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка получения списка каналов."))
		return
	}
	if len(channels) == 0 {
		msg := tgbotapi.NewMessage(chatID,
			"📭 Список каналов обязательной подписки пуст.\n\nДобавьте хотя бы один через ➕ Добавить канал подписки.")
		msg.ReplyMarkup = AdminRequiredSubsKeyboard()
		bot.Send(msg)
		return
	}

	var sb strings.Builder
	sb.WriteString("📋 *Каналы обязательной подписки:*\n\n")
	for i, c := range channels {
		title := c.Title
		if title == "" {
			title = "(без названия)"
		}
		sb.WriteString(fmt.Sprintf("%d. *%s*\n   `%s`\n", i+1, title, c.ChatID))
		if c.InviteLink != "" {
			sb.WriteString(fmt.Sprintf("   🔗 %s\n", c.InviteLink))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("Нажмите на канал, чтобы *удалить* его:")

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = RequiredChannelRemoveKeyboard(channels)
	bot.Send(msg)
}

// PromptEditSubscribeMessage asks admin for a new welcome/subscribe message text.
func PromptEditSubscribeMessage(bot *tgbotapi.BotAPI, chatID int64) {
	setState(adminID, StateAdminReqEditMsg)

	current, _ := GetSetting(SettingSubscribeMessage)

	var sb strings.Builder
	sb.WriteString("✏️ *Изменение текста приветствия*\n\n")
	sb.WriteString("Этот текст увидит пользователь, когда у него запросят обязательную подписку.\n\n")
	if strings.TrimSpace(current) != "" {
		sb.WriteString("Текущий текст:\n")
		sb.WriteString("```\n")
		sb.WriteString(current)
		sb.WriteString("\n```\n\n")
	} else {
		sb.WriteString("Сейчас используется текст по умолчанию.\n\n")
	}
	sb.WriteString("Отправьте новый текст одним сообщением.\n")
	sb.WriteString("Чтобы отменить — нажмите ❌ Отменить.\n")
	sb.WriteString("Чтобы сбросить к тексту по умолчанию — отправьте `-`.")

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminCancelOnlyKeyboard()
	bot.Send(msg)
}

// HandleEditSubscribeMessageInput stores the new welcome/subscribe message.
func HandleEditSubscribeMessageInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID
	text := message.Text
	if text == "" {
		text = message.Caption
	}
	text = strings.TrimSpace(text)

	if text == "" {
		bot.Send(tgbotapi.NewMessage(chatID,
			"⚠️ Отправьте текстом новое приветствие или нажмите ❌ Отменить."))
		return
	}

	if text == "-" {
		if err := SetSetting(SettingSubscribeMessage, ""); err != nil {
			log.Printf("SetSetting error: %v", err)
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка сохранения."))
			return
		}
		setState(adminID, StateIdle)
		ok := tgbotapi.NewMessage(chatID, "♻️ Текст приветствия сброшен. Будет использоваться текст по умолчанию.")
		ok.ReplyMarkup = AdminRequiredSubsKeyboard()
		bot.Send(ok)
		return
	}

	if err := SetSetting(SettingSubscribeMessage, text); err != nil {
		log.Printf("SetSetting error: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка сохранения."))
		return
	}
	setState(adminID, StateIdle)

	ok := tgbotapi.NewMessage(chatID, "✅ Текст приветствия обновлён.")
	ok.ReplyMarkup = AdminRequiredSubsKeyboard()
	bot.Send(ok)
}

// ResetSubscribeMessage resets the subscription message to default.
func ResetSubscribeMessage(bot *tgbotapi.BotAPI, chatID int64) {
	if err := SetSetting(SettingSubscribeMessage, ""); err != nil {
		log.Printf("SetSetting error: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка сброса."))
		return
	}
	msg := tgbotapi.NewMessage(chatID, "♻️ Текст приветствия сброшен. Будет использоваться текст по умолчанию.")
	msg.ReplyMarkup = AdminRequiredSubsKeyboard()
	bot.Send(msg)
}

// HandleRequiredChannelRemoveCallback removes a required channel by db id and refreshes list.
func HandleRequiredChannelRemoveCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID

	parts := strings.SplitN(callback.Data, ":", 2)
	if len(parts) != 2 {
		return
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}

	ch, err := GetRequiredChannelByID(id)
	if err != nil {
		answer := tgbotapi.NewCallback(callback.ID, "Канал не найден")
		bot.Send(answer)
		return
	}

	if err := RemoveRequiredChannel(ch.ChatID); err != nil {
		log.Printf("RemoveRequiredChannel error: %v", err)
		answer := tgbotapi.NewCallback(callback.ID, "Ошибка удаления")
		bot.Send(answer)
		return
	}

	channels, _ := ListRequiredChannels()
	if len(channels) == 0 {
		edit := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID,
			"📭 Список каналов обязательной подписки пуст.")
		bot.Send(edit)
	} else {
		var sb strings.Builder
		sb.WriteString("📋 *Каналы обязательной подписки:*\n\n")
		for i, c := range channels {
			title := c.Title
			if title == "" {
				title = "(без названия)"
			}
			sb.WriteString(fmt.Sprintf("%d. *%s*\n   `%s`\n", i+1, title, c.ChatID))
			if c.InviteLink != "" {
				sb.WriteString(fmt.Sprintf("   🔗 %s\n", c.InviteLink))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("Нажмите на канал, чтобы *удалить* его:")

		kb := RequiredChannelRemoveKeyboard(channels)
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, callback.Message.MessageID,
			sb.String(), kb)
		edit.ParseMode = "Markdown"
		bot.Send(edit)
	}

	answer := tgbotapi.NewCallback(callback.ID, "Канал удалён")
	bot.Send(answer)
}
