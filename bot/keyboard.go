package main

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ── Language selection keyboard (inline) ─────────────────────────────────────

func LanguageKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇷🇺 Русский", "lang:ru"),
			tgbotapi.NewInlineKeyboardButtonData("🇰🇿 Қазақша", "lang:kz"),
		),
	)
}

// ── User main keyboard (reply keyboard at bottom) ───────────────────────────

func UserKeyboard(lang string) tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(t(lang, "btn_send_anon")),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(t(lang, "btn_help")),
			tgbotapi.NewKeyboardButton(t(lang, "btn_change_lang")),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// ── Cancel keyboard shown when user is composing a message ──────────────────

func CancelKeyboard(lang string) tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(t(lang, "btn_cancel")),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// ── Admin keyboard ──────────────────────────────────────────────────────────

func AdminKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📊 Статистика"),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// ── Inline keyboards ────────────────────────────────────────────────────────

func ConfirmSendKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "btn_confirm_send"), "confirm_send"),
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "btn_cancel_send"), "cancel_send"),
		),
	)
}

// WelcomeInlineKeyboard provides an inline button to start creating an anonymous message.
func WelcomeInlineKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "btn_send_anon"), "start_anon"),
		),
	)
}

func BanKeyboard(anonNumber int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚫 Забанить", fmt.Sprintf("ban:%d", anonNumber)),
			tgbotapi.NewInlineKeyboardButtonData("💬 Сообщение", fmt.Sprintf("reply:%d", anonNumber)),
		),
	)
}

func ConfirmBanKeyboard(anonNumber int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Да, забанить", fmt.Sprintf("confirm_ban:%d", anonNumber)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", fmt.Sprintf("cancel_ban:%d", anonNumber)),
		),
	)
}

func UnbanKeyboard(anonNumber int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Разбанить", fmt.Sprintf("unban:%d", anonNumber)),
			tgbotapi.NewInlineKeyboardButtonData("💬 Сообщение", fmt.Sprintf("reply:%d", anonNumber)),
		),
	)
}

func InfoKeyboard(anonNumber int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚫 Забанить", fmt.Sprintf("ban:%d", anonNumber)),
			tgbotapi.NewInlineKeyboardButtonData("✅ Разбанить", fmt.Sprintf("unban:%d", anonNumber)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💬 Сообщение", fmt.Sprintf("reply:%d", anonNumber)),
		),
	)
}

func SubscriptionKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL(t(lang, "btn_subscribe"), ChannelLink),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t(lang, "btn_check_sub"), "check_subscription"),
		),
	)
}

func ConfirmAdminReplyKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Да, отправить", "confirm_admin_reply"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel_admin_reply"),
		),
	)
}
