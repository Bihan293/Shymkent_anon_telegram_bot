package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// OpenRequiredSubsMenu shows the mandatory-subscription management menu.
//
// IMPORTANT: this menu used to render with ParseMode="Markdown" and embed
// user-controlled values (channel titles, t.me/some_channel invite links,
// custom welcome text). Any underscore/asterisk/backtick in those values
// caused Telegram to reject the whole message with a parse error, so the
// "🔔 Обязательная подписка" button looked completely dead. The menu now
// renders as plain text so it can never fail because of bad characters.
func OpenRequiredSubsMenu(bot *tgbotapi.BotAPI, chatID int64) {
	setState(adminID, StateIdle)
	setAdminMenu("reqsubs")

	channels, err := ListRequiredChannels()
	if err != nil {
		log.Printf("OpenRequiredSubsMenu: ListRequiredChannels error: %v", err)
	}
	custom, _ := GetSetting(SettingSubscribeMessage)

	var sb strings.Builder
	sb.WriteString("🔔 Обязательная подписка\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n\n")

	if len(channels) == 0 {
		sb.WriteString("📭 Каналов нет.\n")
		sb.WriteString("Подписка не требуется — пользователи могут пользоваться ботом свободно.\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("📡 Активных каналов: %d\n\n", len(channels)))
		for i, c := range channels {
			title := c.Title
			if title == "" {
				title = "(без названия)"
			}
			sb.WriteString(fmt.Sprintf("%d. %s\n   ID: %s\n", i+1, title, c.ChatID))
			if c.InviteLink != "" {
				sb.WriteString(fmt.Sprintf("   🔗 %s\n", c.InviteLink))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("✏️ Текст приветствия:\n")
	if strings.TrimSpace(custom) == "" {
		sb.WriteString("(по умолчанию)\n\n")
	} else {
		short := custom
		if len([]rune(short)) > 200 {
			short = string([]rune(short)[:200]) + "..."
		}
		sb.WriteString(short + "\n\n")
	}

	sb.WriteString("👇 Выберите действие:")

	msg := tgbotapi.NewMessage(chatID, sb.String())
	// Plain text on purpose — see comment above.
	msg.ReplyMarkup = AdminRequiredSubsKeyboard()
	safeSend(bot, msg)
}

// PromptAddRequiredChannel asks the admin to send a channel @username, id, or forward.
func PromptAddRequiredChannel(bot *tgbotapi.BotAPI, chatID int64) {
	setState(adminID, StateAdminReqChanAdd)
	text := "➕ Добавление канала обязательной подписки\n" +
		"━━━━━━━━━━━━━━━━━━━━\n\n" +
		"Отправьте канал одним из способов:\n\n" +
		"📨 Перешлите любой пост из канала (рекомендуется)\n" +
		"🔗 Отправьте ссылку: https://t.me/имя_канала\n" +
		"🆔 Отправьте ID: -1001234567890\n" +
		"💬 Отправьте @username канала\n\n" +
		"⚠️ Важно: бот должен быть админом канала, иначе не сможет проверить подписку.\n\n" +
		"❌ Чтобы отменить — нажмите кнопку ниже."
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = AdminCancelOnlyKeyboard()
	safeSend(bot, msg)
}

// HandleAddRequiredChannelInput processes admin input when adding a required channel.
// All Telegram API calls are done in a goroutine so the webhook handler never hangs.
func HandleAddRequiredChannelInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID

	var chatRef string
	var title string
	var inviteLink string

	// 1. forwarded from channel — preferred path, works for private channels too.
	if message.ForwardFromChat != nil && message.ForwardFromChat.IsChannel() {
		chatRef = strconv.FormatInt(message.ForwardFromChat.ID, 10)
		title = message.ForwardFromChat.Title
		if message.ForwardFromChat.UserName != "" {
			inviteLink = "https://t.me/" + message.ForwardFromChat.UserName
		}
	} else {
		raw := strings.TrimSpace(message.Text)
		if raw == "" {
			msg := tgbotapi.NewMessage(chatID,
				"⚠️ Перешлите пост из канала, отправьте ссылку, @username или ID.")
			msg.ReplyMarkup = AdminCancelOnlyKeyboard()
			safeSend(bot, msg)
			return
		}

		ref := ParseChatReference(raw)
		switch ref.Kind {
		case ChatRefID:
			chatRef = strconv.FormatInt(ref.ChatID, 10)
		case ChatRefUsername:
			chatRef = ref.Username
			inviteLink = ref.InviteLink
		default:
			em := tgbotapi.NewMessage(chatID, FormatChatRefError(ref))
			em.ReplyMarkup = AdminCancelOnlyKeyboard()
			safeSend(bot, em)
			return
		}
	}

	// Immediate feedback so the admin sees the bot is working
	wait := tgbotapi.NewMessage(chatID, "⏳ Проверяю канал, подождите...")
	waitSent, _ := bot.Send(wait)

	// Run the slow Telegram API call asynchronously so the webhook returns instantly
	go func() {
		resolveAndSaveRequiredChannel(bot, chatID, waitSent.MessageID, chatRef, title, inviteLink)
	}()
}

func resolveAndSaveRequiredChannel(bot *tgbotapi.BotAPI, chatID int64, waitMsgID int, chatRef, title, inviteLink string) {
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

	// Delete the "checking..." message
	if waitMsgID != 0 {
		del := tgbotapi.NewDeleteMessage(chatID, waitMsgID)
		if _, derr := bot.Request(del); derr != nil {
			log.Printf("delete wait msg failed: %v", derr)
		}
	}

	if err != nil {
		log.Printf("GetChat (required) error for %s: %v", chatRef, err)
		// IMPORTANT: clear the state on error too, otherwise the admin gets
		// stuck in StateAdminReqChanAdd and every subsequent message keeps
		// looping into this handler — which is exactly what the user reported.
		setState(adminID, StateIdle)
		errMsg := tgbotapi.NewMessage(chatID,
			"❌ Не удалось получить информацию о канале.\n\n"+
				"Возможные причины:\n"+
				"• Канал не существует или приватный\n"+
				"• Бот не добавлен в канал как админ\n"+
				"• Неверный ID/ссылка\n\n"+
				"Для приватного канала — перешлите пост из него.\n\n"+
				"Возвращаю в меню обязательной подписки.")
		errMsg.ReplyMarkup = AdminRequiredSubsKeyboard()
		safeSend(bot, errMsg)
		// Show the menu again so admin always has a working keyboard.
		OpenRequiredSubsMenu(bot, chatID)
		return
	}

	storeChatID := strconv.FormatInt(chatInfo.ID, 10)
	if title == "" {
		title = chatInfo.Title
	}
	if inviteLink == "" && chatInfo.UserName != "" {
		inviteLink = "https://t.me/" + chatInfo.UserName
	}

	if err := AddRequiredChannel(storeChatID, title, inviteLink); err != nil {
		log.Printf("AddRequiredChannel error: %v", err)
		setState(adminID, StateIdle)
		safeSend(bot, tgbotapi.NewMessage(chatID,
			"❌ Ошибка сохранения канала в БД. Попробуйте ещё раз."))
		OpenRequiredSubsMenu(bot, chatID)
		return
	}

	setState(adminID, StateIdle)

	displayTitle := title
	if displayTitle == "" {
		displayTitle = "(без названия)"
	}
	doneText := fmt.Sprintf("✅ Канал успешно добавлен!\n\n📡 %s\n🆔 %s", displayTitle, storeChatID)
	if inviteLink != "" {
		doneText += "\n🔗 " + inviteLink
	} else {
		doneText += "\n\n⚠️ Ссылка не определена. Если канал приватный — перешлите пост из него ещё раз."
	}

	doneMsg := tgbotapi.NewMessage(chatID, doneText)
	doneMsg.ReplyMarkup = AdminRequiredSubsKeyboard()
	safeSend(bot, doneMsg)

	// Show updated menu
	OpenRequiredSubsMenu(bot, chatID)
}

// ShowRequiredChannelsList shows the list with remove inline buttons.
func ShowRequiredChannelsList(bot *tgbotapi.BotAPI, chatID int64) {
	channels, err := ListRequiredChannels()
	if err != nil {
		log.Printf("ListRequiredChannels error: %v", err)
		safeSend(bot, tgbotapi.NewMessage(chatID, "❌ Ошибка получения списка каналов."))
		return
	}
	if len(channels) == 0 {
		msg := tgbotapi.NewMessage(chatID,
			"📭 Список пуст.\n\nДобавьте хотя бы один канал через ➕ Добавить канал подписки.")
		msg.ReplyMarkup = AdminRequiredSubsKeyboard()
		safeSend(bot, msg)
		return
	}

	var sb strings.Builder
	sb.WriteString("📋 Каналы обязательной подписки\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n\n")
	for i, c := range channels {
		title := c.Title
		if title == "" {
			title = "(без названия)"
		}
		sb.WriteString(fmt.Sprintf("%d. %s\n   ID: %s\n", i+1, title, c.ChatID))
		if c.InviteLink != "" {
			sb.WriteString(fmt.Sprintf("   🔗 %s\n", c.InviteLink))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("👇 Нажмите 🗑 чтобы удалить канал:")

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ReplyMarkup = RequiredChannelRemoveKeyboard(channels)
	safeSend(bot, msg)
}

// PromptEditSubscribeMessage asks admin for a new welcome/subscribe message text.
func PromptEditSubscribeMessage(bot *tgbotapi.BotAPI, chatID int64) {
	setState(adminID, StateAdminReqChanEditMsg)

	current, _ := GetSetting(SettingSubscribeMessage)

	var sb strings.Builder
	sb.WriteString("✏️ Изменение текста приветствия\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n\n")
	sb.WriteString("Этот текст увидит пользователь, когда у него запросят обязательную подписку.\n\n")
	if strings.TrimSpace(current) != "" {
		sb.WriteString("📝 Текущий текст:\n" + current + "\n\n")
	} else {
		sb.WriteString("📝 Сейчас используется текст по умолчанию.\n\n")
	}
	sb.WriteString("✉️ Отправьте новый текст одним сообщением.\n")
	sb.WriteString("♻️ Отправьте \"-\" чтобы сбросить к тексту по умолчанию.\n")
	sb.WriteString("❌ Чтобы отменить — нажмите кнопку ниже.")

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ReplyMarkup = AdminCancelOnlyKeyboard()
	safeSend(bot, msg)
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
		safeSend(bot, tgbotapi.NewMessage(chatID,
			"⚠️ Отправьте текстом новое приветствие или нажмите ❌ Отменить."))
		return
	}

	if text == "-" {
		if err := SetSetting(SettingSubscribeMessage, ""); err != nil {
			log.Printf("SetSetting error: %v", err)
			setState(adminID, StateIdle)
			safeSend(bot, tgbotapi.NewMessage(chatID, "❌ Ошибка сохранения."))
			return
		}
		setState(adminID, StateIdle)
		ok := tgbotapi.NewMessage(chatID,
			"♻️ Текст приветствия сброшен.\nБудет использоваться текст по умолчанию.")
		ok.ReplyMarkup = AdminRequiredSubsKeyboard()
		safeSend(bot, ok)
		return
	}

	if err := SetSetting(SettingSubscribeMessage, text); err != nil {
		log.Printf("SetSetting error: %v", err)
		setState(adminID, StateIdle)
		safeSend(bot, tgbotapi.NewMessage(chatID, "❌ Ошибка сохранения."))
		return
	}
	setState(adminID, StateIdle)

	ok := tgbotapi.NewMessage(chatID, "✅ Текст приветствия обновлён.")
	ok.ReplyMarkup = AdminRequiredSubsKeyboard()
	safeSend(bot, ok)
}

// ResetSubscribeMessage resets the subscription message to default.
func ResetSubscribeMessage(bot *tgbotapi.BotAPI, chatID int64) {
	if err := SetSetting(SettingSubscribeMessage, ""); err != nil {
		log.Printf("SetSetting error: %v", err)
		safeSend(bot, tgbotapi.NewMessage(chatID, "❌ Ошибка сброса."))
		return
	}
	msg := tgbotapi.NewMessage(chatID,
		"♻️ Текст приветствия сброшен.\nБудет использоваться текст по умолчанию.")
	msg.ReplyMarkup = AdminRequiredSubsKeyboard()
	safeSend(bot, msg)
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
		if _, err := bot.Send(edit); err != nil {
			log.Printf("edit empty list failed: %v", err)
		}
	} else {
		var sb strings.Builder
		sb.WriteString("📋 Каналы обязательной подписки\n")
		sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n\n")
		for i, c := range channels {
			title := c.Title
			if title == "" {
				title = "(без названия)"
			}
			sb.WriteString(fmt.Sprintf("%d. %s\n   ID: %s\n", i+1, title, c.ChatID))
			if c.InviteLink != "" {
				sb.WriteString(fmt.Sprintf("   🔗 %s\n", c.InviteLink))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("👇 Нажмите 🗑 чтобы удалить канал:")

		kb := RequiredChannelRemoveKeyboard(channels)
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, callback.Message.MessageID,
			sb.String(), kb)
		if _, err := bot.Send(edit); err != nil {
			log.Printf("edit list failed: %v", err)
		}
	}

	answer := tgbotapi.NewCallback(callback.ID, "✅ Канал удалён")
	bot.Send(answer)
}
