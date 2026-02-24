package main

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func UserKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("✉️ Создать сообщение"),
		),
	)
}

func AdminKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📊 Статистика"),
		),
	)
}

func ConfirmSendKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Отправить", "confirm_send"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel_send"),
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

func SubscriptionKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📢 Подписаться на канал", ChannelLink),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Проверить подписку", "check_subscription"),
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
