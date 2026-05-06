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

	text := "✍️ *Рассылка по пользователям бота*\n\n" +
		"Отправьте сообщение, которое нужно разослать.\n\n" +
		"Можно отправить:\n" +
		"• Текст\n" +
		"• Фото (до 8 шт.)\n" +
		"• Видео (до 3 шт.)\n" +
		"• Альбом (фото+видео) с подписью\n\n" +
		"После — нажмите *«👁 Предпросмотр»* чтобы продолжить."
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminComposeKeyboard()
	bot.Send(msg)
}

// StartBroadcastChannel shows channel selection / management menu.
func StartBroadcastChannel(bot *tgbotapi.BotAPI, chatID int64) {
	channels, _ := ListChannels()

	header := "📢 *Рассылка в канал*\n\n"
	if len(channels) == 0 {
		header += "У вас пока нет добавленных каналов.\nНажмите *«➕ Добавить канал»*, чтобы начать."
	} else {
		header += fmt.Sprintf("Сохранено каналов: *%d*.\n\nНиже выберите канал для рассылки или управляйте списком через меню:", len(channels))
	}

	msg := tgbotapi.NewMessage(chatID, header)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminChannelMenuKeyboard()
	bot.Send(msg)

	if len(channels) > 0 {
		pickMsg := tgbotapi.NewMessage(chatID, "👇 Выберите канал для рассылки:")
		pickMsg.ReplyMarkup = ChannelsListKeyboard(channels, "chan_select")
		bot.Send(pickMsg)
	}
}

// PromptAddChannel asks the admin to send a channel @username or id.
func PromptAddChannel(bot *tgbotapi.BotAPI, chatID int64) {
	setState(adminID, StateAdminBcastChanTarget)
	text := "➕ *Добавление канала*\n\n" +
		"Перешлите любое сообщение из канала, ИЛИ отправьте:\n" +
		"• `@username_канала`\n" +
		"• ID канала (например `-1001234567890`)\n\n" +
		"⚠️ Бот должен быть *администратором* канала и иметь право публиковать сообщения."
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
		bot.Send(tgbotapi.NewMessage(chatID, "📭 Список каналов пуст."))
		return
	}

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

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = ChannelRemoveKeyboard(channels)
	bot.Send(msg)
}

// HandleAddChannelInput processes admin's input when adding a channel.
func HandleAddChannelInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID

	var chatRef string
	var title string

	// 1. forwarded from channel
	if message.ForwardFromChat != nil && message.ForwardFromChat.IsChannel() {
		chatRef = strconv.FormatInt(message.ForwardFromChat.ID, 10)
		title = message.ForwardFromChat.Title
	} else {
		// 2. text input — @username or numeric id
		raw := strings.TrimSpace(message.Text)
		if raw == "" {
			bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Перешлите сообщение из канала или отправьте @username / id."))
			return
		}
		chatRef = raw
		// will resolve below
	}

	// Try to fetch chat info from Telegram to validate + get a nicer title
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
		log.Printf("GetChat error for %s: %v", chatRef, err)
		bot.Send(tgbotapi.NewMessage(chatID,
			"❌ Не удалось получить информацию о канале.\n"+
				"Убедитесь, что:\n"+
				"• Канал существует и публичный (или бот добавлен в него)\n"+
				"• Бот *администратор* канала\n\n"+
				"Попробуйте снова или нажмите ❌ Отменить."))
		return
	}

	// Always store as numeric id (most reliable for posting later)
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
		fmt.Sprintf("✅ Канал добавлен:\n*%s*\n`%s`", title, storeChatID))
	doneMsg.ParseMode = "Markdown"
	doneMsg.ReplyMarkup = AdminChannelMenuKeyboard()
	bot.Send(doneMsg)
}

// ── Composition: collecting media/text ────────────────────────────────────

// HandleBroadcastContent collects content for a broadcast draft.
func HandleBroadcastContent(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	draft := getBroadcastDraft()
	if draft == nil {
		setState(adminID, StateIdle)
		return
	}

	// media group: buffer & merge
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

	setBroadcastDraft(draft)

	// hint that content was added
	hint := "✅ Контент добавлен. Можно отправить ещё медиа/текст или нажать *«👁 Предпросмотр»*."
	hintMsg := tgbotapi.NewMessage(message.Chat.ID, hint)
	hintMsg.ParseMode = "Markdown"
	hintMsg.ReplyMarkup = AdminComposeKeyboard()
	bot.Send(hintMsg)
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

		hintMsg := tgbotapi.NewMessage(chatID,
			"✅ Альбом добавлен. Можно отправить ещё медиа/текст или нажать *«👁 Предпросмотр»*.")
		hintMsg.ParseMode = "Markdown"
		hintMsg.ReplyMarkup = AdminComposeKeyboard()
		bot.Send(hintMsg)
	})
	bcastMediaMu.Unlock()
}

// AskForButtons prompts admin to add inline buttons or skip.
func AskForButtons(bot *tgbotapi.BotAPI, chatID int64) {
	state := getState(adminID)
	switch state {
	case StateAdminBcastUsersContent, StateAdminBcastUsersConfirm:
		setState(adminID, StateAdminBcastUsersButtons)
	case StateAdminBcastChanContent, StateAdminBcastChanConfirm:
		setState(adminID, StateAdminBcastChanButtons)
	}

	text := "🔘 *Добавление inline-кнопок*\n\n" +
		"Отправьте кнопки одним сообщением в формате:\n" +
		"`Текст кнопки - https://example.com`\n\n" +
		"Каждая кнопка с новой строки. Например:\n" +
		"```\n" +
		"Наш канал - https://t.me/Kazakhstan_anon\n" +
		"Сайт - https://example.com\n" +
		"```\n\n" +
		"Или нажмите *«🚫 Без кнопок»*, чтобы пропустить."
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
			"❌ "+err.Error()+"\n\nПопробуйте снова или нажмите 🚫 Без кнопок."))
		return false
	}
	draft.Buttons = buttons
	setBroadcastDraft(draft)
	return true
}

// parseButtonsInput parses lines of "Text - https://url" into InlineButton list.
func parseButtonsInput(input string) ([]InlineButton, error) {
	lines := strings.Split(input, "\n")
	var out []InlineButton
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		// split on " - " (space dash space) — safer than just "-"
		idx := strings.LastIndex(ln, " - ")
		if idx == -1 {
			// try " — " (em dash) or just "-"
			idx = strings.LastIndex(ln, " — ")
			if idx == -1 {
				return nil, fmt.Errorf("неверный формат строки: %q. Используйте `Текст - https://url`", ln)
			}
		}
		name := strings.TrimSpace(ln[:idx])
		url := strings.TrimSpace(ln[idx+3:])
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

// SendBroadcastPreview shows admin a preview of the broadcast and a confirm keyboard.
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

	// Validate limits
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

	header := "👁 *ПРЕДПРОСМОТР РАССЫЛКИ*"
	if draft.Target == BroadcastUsers {
		count, _ := GetUsersCount()
		header += fmt.Sprintf("\nЦель: пользователи бота (≈ %d чел.)", count)
	} else {
		header += fmt.Sprintf("\nЦель: канал `%s`", draft.ChannelID)
	}

	intro := tgbotapi.NewMessage(chatID, header)
	intro.ParseMode = "Markdown"
	bot.Send(intro)

	// Send the actual preview as the message will look to receivers
	inlineKB := BuildBroadcastInlineKeyboard(draft.Buttons)
	sendBroadcastContent(bot, chatID, draft, inlineKB)

	// Now ask for confirmation via inline buttons
	if draft.Target == BroadcastUsers {
		setState(adminID, StateAdminBcastUsersConfirm)
	} else {
		setState(adminID, StateAdminBcastChanConfirm)
	}

	confirmMsg := tgbotapi.NewMessage(chatID, "Запустить рассылку?")
	confirmMsg.ReplyMarkup = ConfirmBroadcastKeyboard()
	bot.Send(confirmMsg)
}

// sendBroadcastContent sends the broadcast content to a single chat (preview or real recipient).
func sendBroadcastContent(bot *tgbotapi.BotAPI, chatID int64, draft *BroadcastDraft, replyMarkup *tgbotapi.InlineKeyboardMarkup) error {
	totalMedia := len(draft.PhotoIDs) + len(draft.VideoIDs)

	// Album: telegram media-groups don't support inline reply markup,
	// so we send the album first (without buttons), then a follow-up text
	// message with the inline keyboard if there are buttons.
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

	// Single photo
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

	// Single video
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

	// Text only
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

	// Capture local copy because we'll reset draft after launching
	d := *draft
	deleteBroadcastDraft()
	setState(adminID, StateIdle)

	go func() {
		startMsg := tgbotapi.NewMessage(chatID, "🚀 Рассылка запущена...")
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
		// Don't broadcast to admin itself (already saw preview)
		// — actually we keep it, admin is also a user. But we'll send anyway.
		if err := sendBroadcastContent(bot, uid, d, inlineKB); err != nil {
			failed++
			log.Printf("broadcast to %d failed: %v", uid, err)
		} else {
			sent++
		}
		// Respect Telegram limits ~30 msg/sec — keep it safe at ~25 msg/sec
		time.Sleep(40 * time.Millisecond)
	}

	report := fmt.Sprintf(
		"✅ *Рассылка завершена*\n\n"+
			"📤 Отправлено: *%d*\n"+
			"❌ Ошибок: *%d*\n"+
			"👥 Всего: *%d*",
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

	rep := tgbotapi.NewMessage(adminChatID, "✅ Сообщение отправлено в канал!")
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
