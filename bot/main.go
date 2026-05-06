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
		HandleCallback(bot, update.CallbackQuery)
		return
	}

	// Сообщения
	if update.Message == nil {
		return
	}

	message := update.Message

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

	// ── Admin: Statistics ───────────────────────────────────────────
	if message.Text == "📊 Статистика" {
		HandleStatistics(bot, message)
		return
	}

	// Обычное сообщение (текст/фото/видео) — обработка как анонимное
	HandleUserMessage(bot, message)
}

// handleCancelCommand allows admin to cancel reply mode at any time via /cancel.
func handleCancelCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID
	lang := getUserLang(userID)

	// Admin cancel reply mode
	if userID == adminID {
		state := getState(adminID)
		if state == StateAdminReplyContent || state == StateAdminReplyConfirm {
			deleteAdminReplyDraft()
			setState(adminID, StateIdle)
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Ответ отменён.")
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
