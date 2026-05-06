package main

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ── Admin reply-keyboard button labels (must be unique strings) ──────────────
const (
	BtnAdminPanel        = "🛠 Админ-панель"
	BtnAdminStats        = "📊 Статистика"
	BtnAdminBroadcast    = "📣 Рассылка"
	BtnAdminBack         = "⬅️ Назад"
	BtnAdminBcastUsers   = "👥 По пользователям бота"
	BtnAdminBcastChannel = "📢 В канал"
	BtnAdminAddChannel   = "➕ Добавить канал"
	BtnAdminListChannels = "📋 Список каналов"
	BtnAdminCancel       = "❌ Отменить"
	BtnAdminSkip         = "➡️ Пропустить"
	BtnAdminAddButtons   = "🔘 Добавить кнопки"
	BtnAdminNoButtons    = "🚫 Без кнопок"
	BtnAdminPreview      = "👁 Предпросмотр"

	// Mandatory subscription management
	BtnAdminRequiredSubs    = "🔔 Обязательная подписка"
	BtnAdminReqAddChannel   = "➕ Добавить канал подписки"
	BtnAdminReqListChannels = "📋 Список каналов подписки"
	BtnAdminReqEditMessage  = "✏️ Текст приветствия"
	BtnAdminReqResetMessage = "♻️ Сбросить текст"
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

// ── Admin keyboard (main) ───────────────────────────────────────────────────

func AdminKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminPanel),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// AdminPanelKeyboard — main admin panel reply keyboard.
func AdminPanelKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminStats),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminBroadcast),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminRequiredSubs),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminBack),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// AdminRequiredSubsKeyboard — manage mandatory subscription channels.
func AdminRequiredSubsKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminReqAddChannel),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminReqListChannels),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminReqEditMessage),
			tgbotapi.NewKeyboardButton(BtnAdminReqResetMessage),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminBack),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// RequiredChannelRemoveKeyboard — inline keyboard to remove required channels.
func RequiredChannelRemoveKeyboard(channels []RequiredChannel) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, c := range channels {
		title := c.Title
		if title == "" {
			title = c.ChatID
		}
		rows = append(rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					"🗑 "+title,
					fmt.Sprintf("reqchan_remove:%d", c.ID),
				),
			),
		)
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// AdminBroadcastKeyboard — choose broadcast target.
func AdminBroadcastKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminBcastUsers),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminBcastChannel),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminBack),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// AdminChannelMenuKeyboard — manage channels.
func AdminChannelMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminAddChannel),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminListChannels),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminBack),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// AdminComposeKeyboard — keyboard while composing broadcast content.
func AdminComposeKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminPreview),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminCancel),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// AdminButtonsStepKeyboard — keyboard while asking for inline buttons.
func AdminButtonsStepKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminNoButtons),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminCancel),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// AdminCancelOnlyKeyboard — only cancel.
func AdminCancelOnlyKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnAdminCancel),
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

// SubscriptionKeyboard builds an inline keyboard with one URL-button per
// required channel and a "check subscription" button at the bottom.
func SubscriptionKeyboard(lang string, channels []RequiredChannel) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, c := range channels {
		title := c.Title
		if title == "" {
			title = t(lang, "btn_subscribe")
		}
		link := c.InviteLink
		if link == "" {
			link = buildChannelLink(c.ChatID)
		}
		if link == "" {
			// can't build a URL button without a link — skip this channel
			continue
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📢 "+title, link),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(t(lang, "btn_check_sub"), "check_subscription"),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// buildChannelLink returns a usable t.me/... link for a stored chat reference.
// For @username channels — we can build https://t.me/<name>.
// For numeric -100... ids we have nothing without a stored invite_link, so we
// return "" and let the caller skip the row.
func buildChannelLink(chatRef string) string {
	if chatRef == "" {
		return ""
	}
	if chatRef[0] == '@' {
		return "https://t.me/" + chatRef[1:]
	}
	if chatRef[0] != '-' && (chatRef[0] < '0' || chatRef[0] > '9') {
		// looks like a username without @
		return "https://t.me/" + chatRef
	}
	// numeric id — no public link without stored invite link
	return ""
}

func ConfirmAdminReplyKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Да, отправить", "confirm_admin_reply"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel_admin_reply"),
		),
	)
}

// ConfirmBroadcastKeyboard — confirm or cancel a broadcast.
func ConfirmBroadcastKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Запустить рассылку", "bcast_confirm"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "bcast_cancel"),
		),
	)
}

// ChannelsListKeyboard — inline list of stored channels for selection or removal.
func ChannelsListKeyboard(channels []Channel, action string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, c := range channels {
		title := c.Title
		if title == "" {
			title = c.ChatID
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(
			"📢 "+title,
			fmt.Sprintf("%s:%d", action, c.ID),
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// ChannelRemoveKeyboard — inline keyboard with remove button per channel.
func ChannelRemoveKeyboard(channels []Channel) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, c := range channels {
		title := c.Title
		if title == "" {
			title = c.ChatID
		}
		rows = append(rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					"🗑 "+title,
					fmt.Sprintf("chan_remove:%d", c.ID),
				),
			),
		)
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildBroadcastInlineKeyboard converts admin's button list into Telegram inline keyboard.
func BuildBroadcastInlineKeyboard(buttons []InlineButton) *tgbotapi.InlineKeyboardMarkup {
	if len(buttons) == 0 {
		return nil
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, b := range buttons {
		rows = append(rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL(b.Text, b.URL),
			),
		)
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return &kb
}
