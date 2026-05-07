package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ── Anon-target chat (where anonymous messages get delivered) ─────────────
//
// By default anonymous messages are forwarded to the bot admin's DM. Admin can
// configure a separate chat / channel id to receive them instead by pressing
// "📥 Куда отправлять анонимки" in the admin panel and providing a link, @username,
// numeric id, or by forwarding a post.
//
// Stored in the `settings` table under SettingAnonTargetChatID.

// GetAnonTargetChatID returns the configured chat id (numeric, as int64) where
// anonymous messages should be sent. Returns 0 if not set — caller should fall
// back to adminID.
func GetAnonTargetChatID() int64 {
	v, _ := GetSetting(SettingAnonTargetChatID)
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// GetAnonTargetTitle returns the human-readable title of the anon target chat
// (empty if not configured).
func GetAnonTargetTitle() string {
	v, _ := GetSetting(SettingAnonTargetTitle)
	return v
}

// AnonDeliveryChatID returns the chat id where anonymous messages should
// actually be delivered: configured target if set, otherwise adminID.
func AnonDeliveryChatID() int64 {
	if id := GetAnonTargetChatID(); id != 0 {
		return id
	}
	return adminID
}

// OpenAnonTargetMenu shows current anon-target configuration and management options.
func OpenAnonTargetMenu(bot *tgbotapi.BotAPI, chatID int64) {
	setState(adminID, StateIdle)
	setAdminMenu("anontarget")

	id := GetAnonTargetChatID()
	title := GetAnonTargetTitle()

	var sb strings.Builder
	sb.WriteString("📥 *Куда отправлять анонимки*\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n\n")

	if id == 0 {
		sb.WriteString("📍 *Текущий получатель:* админ (ваш личный чат)\n\n")
		sb.WriteString("Сейчас все анонимные сообщения приходят вам в личку. Вы можете указать канал или чат, и анонимки будут отправляться туда вместо личных сообщений.\n\n")
	} else {
		display := title
		if display == "" {
			display = "(без названия)"
		}
		sb.WriteString(fmt.Sprintf("📍 *Текущий получатель:* %s\n", display))
		sb.WriteString(fmt.Sprintf("🆔 `%d`\n\n", id))
		sb.WriteString("Все анонимки отправляются туда. Кнопки управления (бан/ответ) тоже работают там.\n\n")
	}

	sb.WriteString("👇 Выберите действие:")

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminAnonTargetKeyboard(id != 0)
	bot.Send(msg)
}

// PromptSetAnonTarget asks admin to send chat ref (link/@username/id/forward).
func PromptSetAnonTarget(bot *tgbotapi.BotAPI, chatID int64) {
	setState(adminID, StateAdminAnonTargetSet)

	text := "📥 *Куда отправлять анонимки*\n" +
		"━━━━━━━━━━━━━━━━━━━━\n\n" +
		"Отправьте чат / канал одним из способов:\n\n" +
		"📨 *Перешлите* любой пост из канала (рекомендуется)\n" +
		"🔗 Отправьте ссылку: `https://t.me/имя_канала`\n" +
		"💬 Отправьте `@username_канала`\n" +
		"🆔 Отправьте ID: `-1001234567890`\n\n" +
		"⚠️ *Важно:* бот должен быть админом этого канала / чата с правом публикации, иначе анонимки не дойдут.\n\n" +
		"❌ Чтобы отменить — нажмите кнопку ниже."
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminCancelOnlyKeyboard()
	bot.Send(msg)
}

// HandleSetAnonTargetInput processes admin input when setting the anon target.
func HandleSetAnonTargetInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID

	var chatRef string
	var title string

	// Forwarded post — preferred path (works for private channels too)
	if message.ForwardFromChat != nil && message.ForwardFromChat.IsChannel() {
		chatRef = strconv.FormatInt(message.ForwardFromChat.ID, 10)
		title = message.ForwardFromChat.Title
	} else {
		raw := strings.TrimSpace(message.Text)
		if raw == "" {
			msg := tgbotapi.NewMessage(chatID,
				"⚠️ Перешлите пост из канала, отправьте ссылку, @username или ID.")
			msg.ReplyMarkup = AdminCancelOnlyKeyboard()
			bot.Send(msg)
			return
		}

		ref := ParseChatReference(raw)
		switch ref.Kind {
		case ChatRefID:
			chatRef = strconv.FormatInt(ref.ChatID, 10)
		case ChatRefUsername:
			chatRef = ref.Username
		default:
			em := tgbotapi.NewMessage(chatID, FormatChatRefError(ref))
			em.ReplyMarkup = AdminCancelOnlyKeyboard()
			bot.Send(em)
			return
		}
	}

	wait := tgbotapi.NewMessage(chatID, "⏳ Проверяю чат, подождите...")
	waitSent, _ := bot.Send(wait)

	go func() {
		resolveAndSaveAnonTarget(bot, chatID, waitSent.MessageID, chatRef, title)
	}()
}

func resolveAndSaveAnonTarget(bot *tgbotapi.BotAPI, chatID int64, waitMsgID int, chatRef, title string) {
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

	if waitMsgID != 0 {
		del := tgbotapi.NewDeleteMessage(chatID, waitMsgID)
		bot.Request(del)
	}

	if err != nil {
		log.Printf("GetChat (anon target) error for %s: %v", chatRef, err)
		errMsg := tgbotapi.NewMessage(chatID,
			"❌ *Не удалось получить информацию о чате.*\n\n"+
				"Возможные причины:\n"+
				"• Чат / канал не существует\n"+
				"• Бот не добавлен туда как админ\n"+
				"• Неверная ссылка / ID / username\n\n"+
				"Для приватного канала — перешлите пост из него.\n\n"+
				"Попробуйте снова или нажмите ❌ Отменить.")
		errMsg.ParseMode = "Markdown"
		errMsg.ReplyMarkup = AdminCancelOnlyKeyboard()
		bot.Send(errMsg)
		return
	}

	storedID := strconv.FormatInt(chatInfo.ID, 10)
	if title == "" {
		title = chatInfo.Title
		if title == "" && chatInfo.UserName != "" {
			title = "@" + chatInfo.UserName
		}
	}

	// Smoke-test: try sending a confirmation message to the target so we know it works.
	probe := tgbotapi.NewMessage(chatInfo.ID,
		"✅ Этот чат настроен как получатель анонимных сообщений бота.\n\nСюда будут приходить все анонимки от пользователей.")
	if _, err := bot.Send(probe); err != nil {
		log.Printf("anon target probe failed: %v", err)
		errMsg := tgbotapi.NewMessage(chatID,
			fmt.Sprintf("❌ *Не удалось отправить пробное сообщение в чат* `%s`.\n\nПроверьте, что бот добавлен туда *администратором* с правом публикации сообщений.\n\nОшибка: %v", storedID, err))
		errMsg.ParseMode = "Markdown"
		errMsg.ReplyMarkup = AdminCancelOnlyKeyboard()
		bot.Send(errMsg)
		return
	}

	if err := SetSetting(SettingAnonTargetChatID, storedID); err != nil {
		log.Printf("SetSetting (anon target id) error: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка сохранения в БД."))
		return
	}
	if err := SetSetting(SettingAnonTargetTitle, title); err != nil {
		log.Printf("SetSetting (anon target title) error: %v", err)
	}

	setState(adminID, StateIdle)

	doneText := fmt.Sprintf("✅ *Получатель анонимок обновлён!*\n\n📡 *%s*\n🆔 `%s`\n\nТеперь все анонимные сообщения будут приходить туда.", title, storedID)
	doneMsg := tgbotapi.NewMessage(chatID, doneText)
	doneMsg.ParseMode = "Markdown"
	bot.Send(doneMsg)

	OpenAnonTargetMenu(bot, chatID)
}

// ResetAnonTarget removes the configured target — anonymous messages will go
// back to admin DM.
func ResetAnonTarget(bot *tgbotapi.BotAPI, chatID int64) {
	if err := SetSetting(SettingAnonTargetChatID, ""); err != nil {
		log.Printf("ResetAnonTarget: SetSetting id error: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка сброса."))
		return
	}
	if err := SetSetting(SettingAnonTargetTitle, ""); err != nil {
		log.Printf("ResetAnonTarget: SetSetting title error: %v", err)
	}
	setState(adminID, StateIdle)

	msg := tgbotapi.NewMessage(chatID, "♻️ *Сброшено.*\nАнонимки снова будут приходить вам в личный чат.")
	msg.ParseMode = "Markdown"
	bot.Send(msg)

	OpenAnonTargetMenu(bot, chatID)
}
