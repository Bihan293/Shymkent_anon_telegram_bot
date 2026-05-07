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

// ── Broadcast draft storage (single admin) ────────────────────────────────
var (
	bcastDraft *BroadcastDraft
	bcastMu    sync.Mutex

	// media-group buffer for broadcast composition
	bcastMediaBuffer = make(map[string][]tgbotapi.Message)
	bcastMediaTimers = make(map[string]*time.Timer)
	bcastMediaMu     sync.Mutex
)

func setBroadcastDraft(d *BroadcastDraft) {
	bcastMu.Lock()
	defer bcastMu.Unlock()
	bcastDraft = d
}

func getBroadcastDraft() *BroadcastDraft {
	bcastMu.Lock()
	defer bcastMu.Unlock()
	return bcastDraft
}

func deleteBroadcastDraft() {
	bcastMu.Lock()
	defer bcastMu.Unlock()
	bcastDraft = nil
}

// ── Entry points (from handlers) ──────────────────────────────────────────

// StartBroadcastUsers begins composing broadcast to all bot users.
func StartBroadcastUsers(bot *tgbotapi.BotAPI, chatID int64) {
	setBroadcastDraft(&BroadcastDraft{Target: BroadcastUsers})
	setState(adminID, StateAdminBcastUsersContent)

	count, _ := GetUsersCount()

	text := "✍️ *Рассылка по пользователям бота*\n" +
		"━━━━━━━━━━━━━━━━━━━━\n\n" +
		fmt.Sprintf("👥 Получателей: *%d*\n\n", count) +
		"📤 *Отправьте сообщение для рассылки:*\n" +
		"• Текст\n" +
		"• Фото (до 8 шт.)\n" +
		"• Видео (до 3 шт.)\n" +
		"• Альбом с подписью\n\n" +
		"💡 _Как только вы пришлёте контент — сразу появится предпросмотр и кнопки подтверждения._\n\n" +
		"❌ Чтобы отменить — нажмите кнопку ниже."
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminCancelOnlyKeyboard()
	bot.Send(msg)
}

// StartBroadcastChannel shows channel selection / management menu.
func StartBroadcastChannel(bot *tgbotapi.BotAPI, chatID int64) {
	channels, _ := ListChannels()

	header := "📢 *Рассылка в канал*\n" +
		"━━━━━━━━━━━━━━━━━━━━\n\n"
	if len(channels) == 0 {
		header += "📭 У вас пока нет добавленных каналов.\n\nНажмите *«➕ Добавить канал»*, чтобы начать."
	} else {
		header += fmt.Sprintf("📡 Сохранено каналов: *%d*\n\nВыберите канал для рассылки или управляйте списком через меню ниже.", len(channels))
	}

	msg := tgbotapi.NewMessage(chatID, header)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminChannelMenuKeyboard()
	bot.Send(msg)

	if len(channels) > 0 {
		pickMsg := tgbotapi.NewMessage(chatID, "👇 *Выберите канал для рассылки:*")
		pickMsg.ParseMode = "Markdown"
		pickMsg.ReplyMarkup = ChannelsListKeyboard(channels, "chan_select")
		bot.Send(pickMsg)
	}
}

// PromptAddChannel asks the admin to send a channel @username or id.
func PromptAddChannel(bot *tgbotapi.BotAPI, chatID int64) {
	setState(adminID, StateAdminBcastChanTarget)
	text := "➕ *Добавление канала для рассылки*\n" +
		"━━━━━━━━━━━━━━━━━━━━\n\n" +
		"Отправьте канал одним из способов:\n\n" +
		"📨 *Перешлите* любой пост из канала (рекомендуется)\n" +
		"💬 Отправьте `@username_канала`\n" +
		"🆔 Отправьте ID: `-1001234567890`\n\n" +
		"⚠️ *Важно:* бот должен быть админом канала с правом публикации.\n\n" +
		"❌ Чтобы отменить — нажмите кнопку ниже."
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminCancelOnlyKeyboard()
	bot.Send(msg)
}

// ShowChannelsList shows admin all stored channels with remove buttons.
func ShowChannelsList(bot *tgbotapi.BotAPI, chatID int64) {
	channels, err := ListChannels()
	if err != nil {
		log.Printf("ListChannels error: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка получения списка каналов."))
		return
	}
	if len(channels) == 0 {
		msg := tgbotapi.NewMessage(chatID, "📭 *Список каналов пуст.*")
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = AdminChannelMenuKeyboard()
		bot.Send(msg)
		return
	}

	var sb strings.Builder
	sb.WriteString("📋 *Список каналов*\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n\n")
	for i, c := range channels {
		title := c.Title
		if title == "" {
			title = "(без названия)"
		}
		sb.WriteString(fmt.Sprintf("`%d.` *%s*\n   ID: `%s`\n\n", i+1, title, c.ChatID))
	}
	sb.WriteString("👇 Нажмите 🗑 чтобы удалить канал:")

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = ChannelRemoveKeyboard(channels)
	bot.Send(msg)
}

// HandleAddChannelInput processes admin's input when adding a channel.
// Async: validates Telegram chat in goroutine so webhook never hangs.
func HandleAddChannelInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID

	var chatRef string
	var title string

	if message.ForwardFromChat != nil && message.ForwardFromChat.IsChannel() {
		chatRef = strconv.FormatInt(message.ForwardFromChat.ID, 10)
		title = message.ForwardFromChat.Title
	} else {
		raw := strings.TrimSpace(message.Text)
		if raw == "" {
			bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Перешлите пост из канала или отправьте @username / ID."))
			return
		}
		chatRef = raw
	}

	wait := tgbotapi.NewMessage(chatID, "⏳ Проверяю канал, подождите...")
	waitSent, _ := bot.Send(wait)

	go func() {
		resolveAndSaveBroadcastChannel(bot, chatID, waitSent.MessageID, chatRef, title)
	}()
}

func resolveAndSaveBroadcastChannel(bot *tgbotapi.BotAPI, chatID int64, waitMsgID int, chatRef, title string) {
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
		log.Printf("GetChat error for %s: %v", chatRef, err)
		errMsg := tgbotapi.NewMessage(chatID,
			"❌ *Не удалось получить информацию о канале.*\n\n"+
				"Возможные причины:\n"+
				"• Канал не существует\n"+
				"• Бот не добавлен в канал как админ\n"+
				"• Неверный ID/username\n\n"+
				"Попробуйте снова или нажмите ❌ Отменить.")
		errMsg.ParseMode = "Markdown"
		errMsg.ReplyMarkup = AdminCancelOnlyKeyboard()
		bot.Send(errMsg)
		return
	}

	storeChatID := strconv.FormatInt(chatInfo.ID, 10)
	if title == "" {
		title = chatInfo.Title
	}

	if err := AddChannel(storeChatID, title); err != nil {
		log.Printf("AddChannel error: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка сохранения канала в БД."))
		return
	}

	setState(adminID, StateIdle)

	doneMsg := tgbotapi.NewMessage(chatID,
		fmt.Sprintf("✅ *Канал добавлен!*\n\n📡 *%s*\n🆔 `%s`", title, storeChatID))
	doneMsg.ParseMode = "Markdown"
	doneMsg.ReplyMarkup = AdminChannelMenuKeyboard()
	bot.Send(doneMsg)
}

// ── Composition: collecting media/text ────────────────────────────────────

// HandleBroadcastContent collects content for a broadcast draft and IMMEDIATELY
// shows preview + buttons-question (no manual "Preview" button needed).
func HandleBroadcastContent(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	draft := getBroadcastDraft()
	if draft == nil {
		setState(adminID, StateIdle)
		return
	}

	// media group: buffer & merge — preview will be triggered after timer fires
	if message.MediaGroupID != "" {
		handleBroadcastMediaGroup(bot, message)
		return
	}

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

	// must have content
	if draft.Text == "" && len(draft.PhotoIDs) == 0 && len(draft.VideoIDs) == 0 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID,
			"⚠️ Пустое сообщение. Отправьте текст, фото или видео."))
		return
	}

	setBroadcastDraft(draft)

	// Auto-advance to buttons step
	AskForButtons(bot, message.Chat.ID)
}

func handleBroadcastMediaGroup(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	groupID := message.MediaGroupID
	chatID := message.Chat.ID

	bcastMediaMu.Lock()
	bcastMediaBuffer[groupID] = append(bcastMediaBuffer[groupID], *message)
	if t, ok := bcastMediaTimers[groupID]; ok {
		t.Stop()
	}
	bcastMediaTimers[groupID] = time.AfterFunc(1500*time.Millisecond, func() {
		bcastMediaMu.Lock()
		messages := bcastMediaBuffer[groupID]
		delete(bcastMediaBuffer, groupID)
		delete(bcastMediaTimers, groupID)
		bcastMediaMu.Unlock()

		draft := getBroadcastDraft()
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
		setBroadcastDraft(draft)

		// Auto-advance to buttons step
		AskForButtons(bot, chatID)
	})
	bcastMediaMu.Unlock()
}

// AskForButtons prompts admin to add inline buttons or skip.
// Sends preview FIRST so admin sees what they're sending, then asks about buttons.
func AskForButtons(bot *tgbotapi.BotAPI, chatID int64) {
	state := getState(adminID)
	switch state {
	case StateAdminBcastUsersContent, StateAdminBcastUsersConfirm:
		setState(adminID, StateAdminBcastUsersButtons)
	case StateAdminBcastChanContent, StateAdminBcastChanConfirm:
		setState(adminID, StateAdminBcastChanButtons)
	}

	draft := getBroadcastDraft()
	if draft == nil {
		setState(adminID, StateIdle)
		return
	}

	// Show preview of the content first
	previewHeader := tgbotapi.NewMessage(chatID, "👁 *ПРЕДПРОСМОТР СООБЩЕНИЯ*\n━━━━━━━━━━━━━━━━━━━━")
	previewHeader.ParseMode = "Markdown"
	bot.Send(previewHeader)

	// Send the actual content as it will look (without buttons yet)
	sendBroadcastContent(bot, chatID, draft, nil)

	text := "🔘 *Шаг 2: добавить кнопки?*\n" +
		"━━━━━━━━━━━━━━━━━━━━\n\n" +
		"Если хотите добавить кнопки-ссылки под сообщением — отправьте их в формате:\n\n" +
		"```\n" +
		"Название | https://ссылка\n" +
		"Ещё кнопка | https://example.com\n" +
		"```\n\n" +
		"📌 *Правила:*\n" +
		"• Каждая кнопка с новой строки\n" +
		"• Разделитель: символ `|` (вертикальная черта)\n" +
		"• Ссылка должна начинаться с `http://`, `https://` или `tg://`\n" +
		"• Максимум 10 кнопок\n\n" +
		"💡 _Старый формат `Текст - https://...` тоже поддерживается._\n\n" +
		"👇 Или нажмите *«🚫 Без кнопок»* чтобы пропустить."
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminButtonsStepKeyboard()
	bot.Send(msg)
}

// HandleButtonsInput parses admin's buttons input.
// Returns true if successful (or skipped), false on parse error.
func HandleButtonsInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message) bool {
	draft := getBroadcastDraft()
	if draft == nil {
		setState(adminID, StateIdle)
		return false
	}

	text := strings.TrimSpace(message.Text)
	if text == "" {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID,
			"⚠️ Отправьте кнопки текстом или нажмите 🚫 Без кнопок."))
		return false
	}

	buttons, err := parseButtonsInput(text)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID,
			"❌ "+err.Error()+"\n\nПримеры правильного формата:\n```\nНаш канал | https://t.me/channel\nСайт | https://example.com\n```\n\nПопробуйте снова или нажмите 🚫 Без кнопок."))
		return false
	}
	draft.Buttons = buttons
	setBroadcastDraft(draft)
	return true
}

// parseButtonsInput parses lines like:
//   "Text | https://url"   (preferred)
//   "Text - https://url"   (legacy)
//   "Text — https://url"   (em dash)
// into an InlineButton list.
func parseButtonsInput(input string) ([]InlineButton, error) {
	lines := strings.Split(input, "\n")
	var out []InlineButton
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}

		var name, url string
		var found bool

		// Preferred: "Text | URL"
		if i := strings.LastIndex(ln, "|"); i >= 0 {
			name = strings.TrimSpace(ln[:i])
			url = strings.TrimSpace(ln[i+1:])
			found = true
		} else if i := strings.LastIndex(ln, " - "); i >= 0 {
			name = strings.TrimSpace(ln[:i])
			url = strings.TrimSpace(ln[i+3:])
			found = true
		} else if i := strings.LastIndex(ln, " — "); i >= 0 {
			name = strings.TrimSpace(ln[:i])
			url = strings.TrimSpace(ln[i+len(" — "):])
			found = true
		}

		if !found {
			return nil, fmt.Errorf("неверный формат строки: %q. Используйте `Текст | https://url`", ln)
		}
		if name == "" || url == "" {
			return nil, fmt.Errorf("пустой текст или ссылка в строке: %q", ln)
		}
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "tg://") {
			return nil, fmt.Errorf("ссылка должна начинаться с http(s):// или tg://: %q", url)
		}
		out = append(out, InlineButton{Text: name, URL: url})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("не найдено ни одной кнопки")
	}
	if len(out) > 10 {
		return nil, fmt.Errorf("слишком много кнопок (максимум 10)")
	}
	return out, nil
}

// ── Preview ────────────────────────────────────────────────────────────────

// SendBroadcastPreview shows admin a final preview with confirm/cancel inline keyboard.
func SendBroadcastPreview(bot *tgbotapi.BotAPI, chatID int64) {
	draft := getBroadcastDraft()
	if draft == nil {
		bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Нет черновика рассылки."))
		setState(adminID, StateIdle)
		return
	}

	if draft.Text == "" && len(draft.PhotoIDs) == 0 && len(draft.VideoIDs) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Контент пуст. Отправьте текст или медиа."))
		return
	}

	if len(draft.PhotoIDs) > MaxPhotos {
		bot.Send(tgbotapi.NewMessage(chatID,
			fmt.Sprintf("⚠️ Слишком много фото. Максимум: %d.", MaxPhotos)))
		return
	}
	if len(draft.VideoIDs) > MaxVideos {
		bot.Send(tgbotapi.NewMessage(chatID,
			fmt.Sprintf("⚠️ Слишком много видео. Максимум: %d.", MaxVideos)))
		return
	}

	header := "👁 *ФИНАЛЬНЫЙ ПРЕДПРОСМОТР*\n━━━━━━━━━━━━━━━━━━━━\n"
	if draft.Target == BroadcastUsers {
		count, _ := GetUsersCount()
		header += fmt.Sprintf("🎯 Цель: пользователи бота\n👥 Получателей: *%d*\n", count)
	} else {
		header += fmt.Sprintf("🎯 Цель: канал `%s`\n", draft.ChannelID)
	}
	if len(draft.Buttons) > 0 {
		header += fmt.Sprintf("🔘 Кнопок: *%d*\n", len(draft.Buttons))
	} else {
		header += "🔘 Кнопок: нет\n"
	}

	intro := tgbotapi.NewMessage(chatID, header)
	intro.ParseMode = "Markdown"
	bot.Send(intro)

	// Send the actual preview as the message will look to receivers
	inlineKB := BuildBroadcastInlineKeyboard(draft.Buttons)
	sendBroadcastContent(bot, chatID, draft, inlineKB)

	if draft.Target == BroadcastUsers {
		setState(adminID, StateAdminBcastUsersConfirm)
	} else {
		setState(adminID, StateAdminBcastChanConfirm)
	}

	confirmMsg := tgbotapi.NewMessage(chatID, "🚀 *Запустить рассылку?*")
	confirmMsg.ParseMode = "Markdown"
	confirmMsg.ReplyMarkup = ConfirmBroadcastKeyboard()
	bot.Send(confirmMsg)
}

// sendBroadcastContent sends the broadcast content to a single chat.
func sendBroadcastContent(bot *tgbotapi.BotAPI, chatID int64, draft *BroadcastDraft, replyMarkup *tgbotapi.InlineKeyboardMarkup) error {
	totalMedia := len(draft.PhotoIDs) + len(draft.VideoIDs)

	if totalMedia > 1 {
		var mediaGroup []interface{}
		first := true
		for _, pid := range draft.PhotoIDs {
			item := tgbotapi.NewInputMediaPhoto(tgbotapi.FileID(pid))
			if first {
				if draft.Text != "" {
					item.Caption = draft.Text
				}
				first = false
			}
			mediaGroup = append(mediaGroup, item)
		}
		for _, vid := range draft.VideoIDs {
			item := tgbotapi.NewInputMediaVideo(tgbotapi.FileID(vid))
			if first {
				if draft.Text != "" {
					item.Caption = draft.Text
				}
				first = false
			}
			mediaGroup = append(mediaGroup, item)
		}

		mg := tgbotapi.NewMediaGroup(chatID, mediaGroup)
		if _, err := bot.SendMediaGroup(mg); err != nil {
			return err
		}

		if replyMarkup != nil {
			btnMsg := tgbotapi.NewMessage(chatID, "👇")
			btnMsg.ReplyMarkup = *replyMarkup
			if _, err := bot.Send(btnMsg); err != nil {
				return err
			}
		}
		return nil
	}

	if len(draft.PhotoIDs) == 1 {
		ph := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(draft.PhotoIDs[0]))
		if draft.Text != "" {
			ph.Caption = draft.Text
		}
		if replyMarkup != nil {
			ph.ReplyMarkup = *replyMarkup
		}
		_, err := bot.Send(ph)
		return err
	}

	if len(draft.VideoIDs) == 1 {
		v := tgbotapi.NewVideo(chatID, tgbotapi.FileID(draft.VideoIDs[0]))
		if draft.Text != "" {
			v.Caption = draft.Text
		}
		if replyMarkup != nil {
			v.ReplyMarkup = *replyMarkup
		}
		_, err := bot.Send(v)
		return err
	}

	if draft.Text == "" {
		return fmt.Errorf("empty broadcast")
	}
	msg := tgbotapi.NewMessage(chatID, draft.Text)
	if replyMarkup != nil {
		msg.ReplyMarkup = *replyMarkup
	}
	_, err := bot.Send(msg)
	return err
}

// ── Execution ─────────────────────────────────────────────────────────────

// ExecuteBroadcast runs the broadcast (in a goroutine, reports back to admin).
func ExecuteBroadcast(bot *tgbotapi.BotAPI, chatID int64) {
	draft := getBroadcastDraft()
	if draft == nil {
		bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Нет черновика рассылки."))
		return
	}

	d := *draft
	deleteBroadcastDraft()
	setState(adminID, StateIdle)

	go func() {
		startMsg := tgbotapi.NewMessage(chatID, "🚀 *Рассылка запущена...*")
		startMsg.ParseMode = "Markdown"
		startMsg.ReplyMarkup = AdminPanelKeyboard()
		bot.Send(startMsg)

		switch d.Target {
		case BroadcastUsers:
			executeBroadcastUsers(bot, chatID, &d)
		case BroadcastChannel:
			executeBroadcastChannel(bot, chatID, &d)
		}
	}()
}

func executeBroadcastUsers(bot *tgbotapi.BotAPI, adminChatID int64, d *BroadcastDraft) {
	ids, err := GetAllUserIDs()
	if err != nil {
		log.Printf("GetAllUserIDs error: %v", err)
		bot.Send(tgbotapi.NewMessage(adminChatID, "❌ Ошибка получения списка пользователей."))
		return
	}
	if len(ids) == 0 {
		bot.Send(tgbotapi.NewMessage(adminChatID, "📭 В базе нет пользователей."))
		return
	}

	inlineKB := BuildBroadcastInlineKeyboard(d.Buttons)

	var sent, failed int
	for _, uid := range ids {
		if err := sendBroadcastContent(bot, uid, d, inlineKB); err != nil {
			failed++
			log.Printf("broadcast to %d failed: %v", uid, err)
		} else {
			sent++
		}
		// ~25 msg/sec to be safe under Telegram limits
		time.Sleep(40 * time.Millisecond)
	}

	report := fmt.Sprintf(
		"✅ *Рассылка завершена*\n"+
			"━━━━━━━━━━━━━━━━━━━━\n\n"+
			"📤 Отправлено: *%d*\n"+
			"❌ Ошибок: *%d*\n"+
			"👥 Всего получателей: *%d*",
		sent, failed, len(ids),
	)
	rep := tgbotapi.NewMessage(adminChatID, report)
	rep.ParseMode = "Markdown"
	rep.ReplyMarkup = AdminPanelKeyboard()
	bot.Send(rep)
}

func executeBroadcastChannel(bot *tgbotapi.BotAPI, adminChatID int64, d *BroadcastDraft) {
	if d.ChannelID == "" {
		bot.Send(tgbotapi.NewMessage(adminChatID, "❌ Не указан канал для рассылки."))
		return
	}

	chatID, err := strconv.ParseInt(d.ChannelID, 10, 64)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(adminChatID,
			fmt.Sprintf("❌ Некорректный ID канала: %s", d.ChannelID)))
		return
	}

	inlineKB := BuildBroadcastInlineKeyboard(d.Buttons)

	if err := sendBroadcastContent(bot, chatID, d, inlineKB); err != nil {
		log.Printf("channel broadcast failed: %v", err)
		errMsg := tgbotapi.NewMessage(adminChatID,
			fmt.Sprintf("❌ Не удалось отправить в канал: %v\n\nПроверьте, что бот *администратор* канала.", err))
		errMsg.ParseMode = "Markdown"
		errMsg.ReplyMarkup = AdminPanelKeyboard()
		bot.Send(errMsg)
		return
	}

	rep := tgbotapi.NewMessage(adminChatID, "✅ *Сообщение отправлено в канал!*")
	rep.ParseMode = "Markdown"
	rep.ReplyMarkup = AdminPanelKeyboard()
	bot.Send(rep)
}

// CancelBroadcast wipes the current draft and resets state.
func CancelBroadcast(bot *tgbotapi.BotAPI, chatID int64) {
	deleteBroadcastDraft()
	setState(adminID, StateIdle)
	msg := tgbotapi.NewMessage(chatID, "❌ Рассылка отменена.")
	msg.ReplyMarkup = AdminPanelKeyboard()
	bot.Send(msg)
}

// IsBroadcastState returns true if admin is currently in any broadcast-related state.
func IsBroadcastState(state string) bool {
	switch state {
	case StateAdminBcastUsersContent,
		StateAdminBcastUsersButtons,
		StateAdminBcastUsersConfirm,
		StateAdminBcastChanTarget,
		StateAdminBcastChanContent,
		StateAdminBcastChanButtons,
		StateAdminBcastChanConfirm:
		return true
	}
	return false
}
