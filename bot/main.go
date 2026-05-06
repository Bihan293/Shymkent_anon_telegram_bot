package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var adminID int64

func main() {
	botToken := os.Getenv("BOT_TOKEN")
	adminIDStr := os.Getenv("ADMIN_ID")
	baseURL := os.Getenv("BASE_URL")
	databaseURL := os.Getenv("DATABASE_URL")
	port := os.Getenv("PORT")

	if botToken == "" || adminIDStr == "" || baseURL == "" || databaseURL == "" {
		log.Fatal("BOT_TOKEN, ADMIN_ID, BASE_URL, DATABASE_URL are required")
	}

	if port == "" {
		port = "8080"
	}

	var err error
	adminID, err = strconv.ParseInt(adminIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid ADMIN_ID: %v", err)
	}

	// Подключение к базе данных
	if err := InitDB(databaseURL); err != nil {
		log.Fatalf("Database init error: %v", err)
	}
	log.Println("Database connected")

	// Создание бота
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Bot init error: %v", err)
	}

	// Настройка webhook
	webhookURL := strings.TrimRight(baseURL, "/") + "/webhook"
	wh, err := tgbotapi.NewWebhook(webhookURL)
	if err != nil {
		log.Fatalf("Webhook create error: %v", err)
	}

	_, err = bot.Request(wh)
	if err != nil {
		log.Fatalf("Webhook set error: %v", err)
	}

	log.Printf("Webhook set: %s", webhookURL)

	// HTTP обработчики

	// Webhook
	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}

		var update tgbotapi.Update
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		processUpdate(bot, update)
		w.WriteHeader(http.StatusOK)
	})

	// /alive endpoint for uptime robot
	http.HandleFunc("/alive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("alive"))
	})

	// Health check
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	log.Printf("Server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func processUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	// Callback query (inline кнопки)
	if update.CallbackQuery != nil {
		// Track user when they interact via callback too
		if u := update.CallbackQuery.From; u != nil {
			_ = UpsertUser(u.ID, u.UserName, u.FirstName)
		}

		// Mandatory subscription guard for callbacks too — but allow:
		// • check_subscription (so user can verify after subscribing)
		// • lang:* (language picker shown before sub check on first start)
		cb := update.CallbackQuery
		if cb.From != nil && cb.From.ID != adminID {
			data := cb.Data
			allow := data == "check_subscription" ||
				strings.HasPrefix(data, "lang:")
			if !allow && !IsSubscribed(bot, cb.From.ID) {
				lang := getUserLang(cb.From.ID)
				if lang == "" {
					lang = LangRU
				}
				if cb.Message != nil {
					sendSubscriptionMessage(bot, cb.Message.Chat.ID, lang)
				}
				ack := tgbotapi.NewCallback(cb.ID, "")
				bot.Send(ack)
				return
			}
		}

		HandleCallback(bot, update.CallbackQuery)
		return
	}

	// Сообщения
	if update.Message == nil {
		return
	}

	message := update.Message

	// Track every interacting user (for broadcasts)
	if message.From != nil {
		_ = UpsertUser(message.From.ID, message.From.UserName, message.From.FirstName)
	}

	// ── Admin: handle admin-panel reply-keyboard buttons FIRST (before commands) ──
	if message.From != nil && message.From.ID == adminID {
		if handleAdminPanelButton(bot, message) {
			return
		}
		// Admin in a multi-step state (broadcast/reply/required-subs) — route to that handler
		state := getState(adminID)
		if state == StateAdminReqChanAdd {
			HandleAddRequiredChannelInput(bot, message)
			return
		}
		if state == StateAdminReqEditMsg {
			HandleEditSubscribeMessageInput(bot, message)
			return
		}
		if IsBroadcastState(state) {
			if handleAdminBroadcastFlow(bot, message, state) {
				return
			}
		}
	}

	// ── Mandatory subscription guard for non-admin users ─────────────────
	// Any action (command/button/text/media) is blocked until the user is
	// subscribed to all configured required channels. If no channels are
	// configured, IsSubscribed returns true and nothing is shown.
	if message.From != nil && message.From.ID != adminID {
		if !IsSubscribed(bot, message.From.ID) {
			lang := getUserLang(message.From.ID)
			if lang == "" {
				lang = LangRU
			}
			// Reset any in-progress flow so user starts clean after subscribing.
			deleteDraft(message.From.ID)
			setState(message.From.ID, StateIdle)
			sendSubscriptionMessage(bot, message.Chat.ID, lang)
			return
		}
	}

	// Команды
	if message.IsCommand() {
		switch message.Command() {
		case "start":
			HandleStart(bot, message)
		case "info":
			HandleInfo(bot, message)
		case "cancel":
			handleCancelCommand(bot, message)
		case "stats":
			HandleStatistics(bot, message)
		case "help":
			HandleHelp(bot, message)
		case "admin":
			handleAdminCommand(bot, message)
		}
		return
	}

	// ── Button: Send anonymous message (both languages) ─────────────
	btnSendRU := t(LangRU, "btn_send_anon")
	btnSendKZ := t(LangKZ, "btn_send_anon")
	if message.Text == btnSendRU || message.Text == btnSendKZ {
		HandleCreateMessage(bot, message)
		return
	}

	// ── Button: Help (both languages) ───────────────────────────────
	btnHelpRU := t(LangRU, "btn_help")
	btnHelpKZ := t(LangKZ, "btn_help")
	if message.Text == btnHelpRU || message.Text == btnHelpKZ {
		HandleHelp(bot, message)
		return
	}

	// ── Button: Change language (both languages) ────────────────────
	btnLangRU := t(LangRU, "btn_change_lang")
	btnLangKZ := t(LangKZ, "btn_change_lang")
	if message.Text == btnLangRU || message.Text == btnLangKZ {
		HandleChangeLang(bot, message)
		return
	}

	// ── Button: Cancel (both languages) ─────────────────────────────
	btnCancelRU := t(LangRU, "btn_cancel")
	btnCancelKZ := t(LangKZ, "btn_cancel")
	if message.Text == btnCancelRU || message.Text == btnCancelKZ {
		handleCancelButton(bot, message)
		return
	}

	// Обычное сообщение (текст/фото/видео) — обработка как анонимное
	HandleUserMessage(bot, message)
}

// handleAdminCommand — quick entry to admin panel via /admin.
func handleAdminCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From.ID != adminID {
		return
	}
	openAdminPanel(bot, message.Chat.ID)
}

// openAdminPanel sends the admin panel main view.
func openAdminPanel(bot *tgbotapi.BotAPI, chatID int64) {
	setState(adminID, StateIdle)
	deleteBroadcastDraft()

	text := "🛠 *Админ-панель*\n\n" +
		"Выберите раздел:\n\n" +
		"📊 *Статистика* — общие показатели бота\n" +
		"📣 *Рассылка* — отправка сообщений пользователям или в канал\n" +
		"🔔 *Обязательная подписка* — каналы, на которые должен подписаться пользователь"
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = AdminPanelKeyboard()
	bot.Send(msg)
}

// handleAdminPanelButton handles the admin reply-keyboard buttons.
// Returns true if the message was a known admin-panel button (and was handled).
func handleAdminPanelButton(bot *tgbotapi.BotAPI, message *tgbotapi.Message) bool {
	chatID := message.Chat.ID

	switch message.Text {
	case BtnAdminPanel:
		openAdminPanel(bot, chatID)
		return true

	case BtnAdminStats:
		HandleStatistics(bot, message)
		return true

	case BtnAdminBroadcast:
		setState(adminID, StateIdle)
		deleteBroadcastDraft()
		msg := tgbotapi.NewMessage(chatID,
			"📣 *Рассылка*\n\nВыберите цель рассылки:")
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = AdminBroadcastKeyboard()
		bot.Send(msg)
		return true

	case BtnAdminBcastUsers:
		StartBroadcastUsers(bot, chatID)
		return true

	case BtnAdminBcastChannel:
		StartBroadcastChannel(bot, chatID)
		return true

	case BtnAdminAddChannel:
		PromptAddChannel(bot, chatID)
		return true

	case BtnAdminListChannels:
		ShowChannelsList(bot, chatID)
		return true

	case BtnAdminRequiredSubs:
		OpenRequiredSubsMenu(bot, chatID)
		return true

	case BtnAdminReqAddChannel:
		PromptAddRequiredChannel(bot, chatID)
		return true

	case BtnAdminReqListChannels:
		ShowRequiredChannelsList(bot, chatID)
		return true

	case BtnAdminReqEditMessage:
		PromptEditSubscribeMessage(bot, chatID)
		return true

	case BtnAdminReqResetMessage:
		ResetSubscribeMessage(bot, chatID)
		return true

	case BtnAdminBack:
		// Context-dependent back: cancel any draft and return to main admin panel
		deleteBroadcastDraft()
		setState(adminID, StateIdle)
		openAdminPanel(bot, chatID)
		return true

	case BtnAdminCancel:
		// Cancel any in-progress admin operation
		state := getState(adminID)
		if IsBroadcastState(state) || state == StateAdminReplyContent || state == StateAdminReplyConfirm ||
			state == StateAdminReqChanAdd || state == StateAdminReqEditMsg {
			deleteBroadcastDraft()
			deleteAdminReplyDraft()
			setState(adminID, StateIdle)
			msg := tgbotapi.NewMessage(chatID, "❌ Действие отменено.")
			msg.ReplyMarkup = AdminPanelKeyboard()
			bot.Send(msg)
			return true
		}
		// Otherwise let it fall through — user might be canceling regular flow
		return false

	case BtnAdminPreview:
		state := getState(adminID)
		if state == StateAdminBcastUsersContent || state == StateAdminBcastChanContent {
			// After content collected → ask for buttons step
			AskForButtons(bot, chatID)
			return true
		}
		return false

	case BtnAdminNoButtons:
		state := getState(adminID)
		if state == StateAdminBcastUsersButtons || state == StateAdminBcastChanButtons {
			draft := getBroadcastDraft()
			if draft != nil {
				draft.Buttons = nil
				setBroadcastDraft(draft)
			}
			SendBroadcastPreview(bot, chatID)
			return true
		}
		return false
	}

	return false
}

// handleAdminBroadcastFlow routes content sent by admin during broadcast composition.
// Returns true if the message was consumed by the broadcast flow.
func handleAdminBroadcastFlow(bot *tgbotapi.BotAPI, message *tgbotapi.Message, state string) bool {
	chatID := message.Chat.ID

	switch state {
	case StateAdminBcastChanTarget:
		HandleAddChannelInput(bot, message)
		return true

	case StateAdminBcastUsersContent, StateAdminBcastChanContent:
		// Anything that's not the preview/cancel button is treated as content
		HandleBroadcastContent(bot, message)
		return true

	case StateAdminBcastUsersButtons, StateAdminBcastChanButtons:
		if message.Text == BtnAdminNoButtons {
			draft := getBroadcastDraft()
			if draft != nil {
				draft.Buttons = nil
				setBroadcastDraft(draft)
			}
			SendBroadcastPreview(bot, chatID)
			return true
		}
		if HandleButtonsInput(bot, message) {
			SendBroadcastPreview(bot, chatID)
		}
		return true

	case StateAdminBcastUsersConfirm, StateAdminBcastChanConfirm:
		// Wait for inline confirm/cancel — but if admin types something else,
		// just remind them.
		bot.Send(tgbotapi.NewMessage(chatID,
			"⏳ Подтвердите рассылку кнопками выше или нажмите ❌ Отменить."))
		return true
	}

	return false
}

// handleCancelCommand allows admin to cancel reply mode at any time via /cancel.
func handleCancelCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID
	lang := getUserLang(userID)

	// Admin: cancel any pending operation
	if userID == adminID {
		state := getState(adminID)
		if state == StateAdminReplyContent || state == StateAdminReplyConfirm {
			deleteAdminReplyDraft()
			setState(adminID, StateIdle)
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Ответ отменён.")
			msg.ReplyMarkup = AdminPanelKeyboard()
			bot.Send(msg)
			return
		}
		if IsBroadcastState(state) {
			deleteBroadcastDraft()
			setState(adminID, StateIdle)
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Рассылка отменена.")
			msg.ReplyMarkup = AdminPanelKeyboard()
			bot.Send(msg)
			return
		}
	}

	// Regular user cancel
	deleteDraft(userID)
	setState(userID, StateIdle)

	isAdmin := userID == adminID

	cancelMsg := tgbotapi.NewMessage(message.Chat.ID, t(lang, "cancelled"))
	if isAdmin {
		cancelMsg.ReplyMarkup = AdminKeyboard()
	} else {
		cancelMsg.ReplyMarkup = UserKeyboard(lang)
	}
	bot.Send(cancelMsg)

	// Show welcome again
	welcomeMsg := tgbotapi.NewMessage(message.Chat.ID, t(lang, "welcome"))
	welcomeMsg.ParseMode = "Markdown"
	welcomeMsg.ReplyMarkup = WelcomeInlineKeyboard(lang)
	bot.Send(welcomeMsg)
}

// handleCancelButton handles the cancel reply-keyboard button press.
func handleCancelButton(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID
	lang := getUserLang(userID)

	// Clean up any draft
	deleteDraft(userID)
	setState(userID, StateIdle)

	isAdmin := userID == adminID

	cancelMsg := tgbotapi.NewMessage(message.Chat.ID, t(lang, "cancelled"))
	if isAdmin {
		cancelMsg.ReplyMarkup = AdminKeyboard()
	} else {
		cancelMsg.ReplyMarkup = UserKeyboard(lang)
	}
	bot.Send(cancelMsg)

	// Show welcome again
	welcomeMsg := tgbotapi.NewMessage(message.Chat.ID, t(lang, "welcome"))
	welcomeMsg.ParseMode = "Markdown"
	welcomeMsg.ReplyMarkup = WelcomeInlineKeyboard(lang)
	bot.Send(welcomeMsg)
}
