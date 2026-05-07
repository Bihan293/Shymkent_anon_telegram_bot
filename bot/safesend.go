package main

import (
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ── Safe send helpers ─────────────────────────────────────────────────────
//
// The bot used to send a lot of messages with ParseMode="Markdown" while
// embedding user-controlled values (channel titles, invite links such as
// https://t.me/some_channel, custom welcome texts, …). Any underscore /
// asterisk / backtick / square-bracket inside those values made Telegram
// reject the whole message with a Bad Request: can't parse entities error
// — and because the original code ignored bot.Send's error, the bot looked
// completely frozen ("🔔 Обязательная подписка" appeared dead).
//
// safeSend transparently retries without ParseMode if the formatted send
// fails, and always logs the original failure so we never silently lose a
// reply again.

// safeSend sends a message and, if it fails because of a Markdown parse
// error or any other Telegram-side issue, retries the same payload as plain
// text so the user always gets a reply. Returns the sent message (empty on
// total failure).
func safeSend(bot *tgbotapi.BotAPI, msg tgbotapi.MessageConfig) tgbotapi.Message {
	sent, err := bot.Send(msg)
	if err == nil {
		return sent
	}
	log.Printf("safeSend: primary send failed (parse_mode=%q): %v", msg.ParseMode, err)

	// If Markdown was the culprit — strip it and retry as plain text.
	if msg.ParseMode != "" {
		plain := msg
		plain.ParseMode = ""
		plain.Text = stripMarkdown(msg.Text)
		sent, err = bot.Send(plain)
		if err == nil {
			return sent
		}
		log.Printf("safeSend: plain retry also failed: %v", err)
	}

	// Last-ditch: try with no keyboard, no formatting.
	bare := tgbotapi.NewMessage(msg.ChatID, stripMarkdown(msg.Text))
	sent, err = bot.Send(bare)
	if err != nil {
		log.Printf("safeSend: bare retry failed: %v", err)
	}
	return sent
}

// stripMarkdown removes the most common Telegram Markdown control characters
// from a string so it can be safely sent as plain text. It does NOT try to
// be a perfect un-formatter — its only job is to make sure no parse error
// happens on the retry.
func stripMarkdown(s string) string {
	if s == "" {
		return s
	}
	// Drop the markers but keep the surrounding text.
	r := strings.NewReplacer(
		"```", "",
		"`", "",
		"**", "",
		"*", "",
		"__", "",
		"_", "",
		"~~", "",
		"~", "",
	)
	return r.Replace(s)
}

// escapeMarkdown escapes the legacy-Markdown special characters so a
// user-supplied string can be safely embedded inside a ParseMode="Markdown"
// message without breaking parsing. This is the simple flavor (Markdown,
// not MarkdownV2) and matches what the rest of the bot uses.
func escapeMarkdown(s string) string {
	if s == "" {
		return s
	}
	// In legacy Markdown the dangerous characters are: _ * ` [
	r := strings.NewReplacer(
		"\\", "\\\\",
		"_", "\\_",
		"*", "\\*",
		"`", "\\`",
		"[", "\\[",
	)
	return r.Replace(s)
}
