package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ChatRefKind tells which form of chat reference we extracted from user input.
type ChatRefKind int

const (
	ChatRefInvalid ChatRefKind = iota
	ChatRefID                  // numeric chat id (e.g. -1001234567890)
	ChatRefUsername            // @public_username
	ChatRefPrivate             // private invite link (t.me/+xxx, t.me/joinchat/xxx) — cannot be used directly
)

// ParsedChatRef is the result of parsing user input describing a Telegram chat.
type ParsedChatRef struct {
	Kind       ChatRefKind
	ChatID     int64  // valid when Kind == ChatRefID
	Username   string // valid when Kind == ChatRefUsername (with leading @)
	InviteLink string // public link (https://t.me/<name>) when known
	Hint       string // when invalid — short reason in Russian
}

// ParseChatReference accepts pretty much any reasonable form a Telegram user
// might enter when adding a channel and converts it into a uniform reference.
//
// Supported inputs:
//   • https://t.me/channel              -> @channel
//   • http://t.me/channel               -> @channel
//   • t.me/channel                      -> @channel
//   • telegram.me/channel, telegram.dog/channel
//   • https://t.me/channel/123 (post link) -> @channel
//   • @channel                          -> @channel
//   • channel                           -> @channel
//   • -1001234567890                    -> numeric id
//   • 1234567890                        -> numeric id (rare, but accepted)
//
// Private invite links (t.me/+abc, t.me/joinchat/abc) are NOT usable directly:
// in that case Kind=ChatRefPrivate and the caller should ask user to forward a
// post from the channel instead.
func ParseChatReference(raw string) ParsedChatRef {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedChatRef{Kind: ChatRefInvalid, Hint: "пустой ввод"}
	}

	// Strip surrounding quotes / backticks if user pasted them.
	raw = strings.Trim(raw, "`'\"")

	// Pure numeric ID (with optional leading minus)
	if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return ParsedChatRef{Kind: ChatRefID, ChatID: id}
	}

	// Looks like @username
	if strings.HasPrefix(raw, "@") {
		uname := strings.TrimSpace(raw[1:])
		if !isValidUsername(uname) {
			return ParsedChatRef{Kind: ChatRefInvalid, Hint: "некорректный @username"}
		}
		return ParsedChatRef{
			Kind:       ChatRefUsername,
			Username:   "@" + uname,
			InviteLink: "https://t.me/" + uname,
		}
	}

	// URL forms ─ normalize the input first
	candidate := raw
	lower := strings.ToLower(candidate)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		// Things like "t.me/channel" — add scheme so url.Parse works.
		if strings.HasPrefix(lower, "t.me/") ||
			strings.HasPrefix(lower, "telegram.me/") ||
			strings.HasPrefix(lower, "telegram.dog/") {
			candidate = "https://" + candidate
		}
	}

	if u, err := url.Parse(candidate); err == nil && u.Host != "" {
		host := strings.ToLower(u.Host)
		if host == "t.me" || host == "www.t.me" || host == "telegram.me" || host == "telegram.dog" {
			path := strings.TrimSpace(strings.Trim(u.Path, "/"))
			if path == "" {
				return ParsedChatRef{Kind: ChatRefInvalid, Hint: "пустая ссылка t.me"}
			}
			// Take the first path segment as candidate username.
			seg := path
			if i := strings.Index(path, "/"); i != -1 {
				seg = path[:i]
			}
			seg = strings.TrimSpace(seg)

			// Private invite link forms — cannot be resolved server-side.
			if strings.HasPrefix(seg, "+") {
				return ParsedChatRef{
					Kind: ChatRefPrivate,
					Hint: "это приватная ссылка-приглашение",
				}
			}
			if strings.EqualFold(seg, "joinchat") {
				return ParsedChatRef{
					Kind: ChatRefPrivate,
					Hint: "это приватная ссылка-приглашение",
				}
			}
			// Some posts look like t.me/c/<internal>/<msg> — also private.
			if strings.EqualFold(seg, "c") {
				return ParsedChatRef{
					Kind: ChatRefPrivate,
					Hint: "это ссылка на пост приватного канала",
				}
			}

			// Otherwise it's a public username (t.me/<name> or t.me/<name>/123)
			if !isValidUsername(seg) {
				return ParsedChatRef{Kind: ChatRefInvalid, Hint: "некорректный username в ссылке"}
			}
			return ParsedChatRef{
				Kind:       ChatRefUsername,
				Username:   "@" + seg,
				InviteLink: "https://t.me/" + seg,
			}
		}
	}

	// Fallback: treat as bare username if it looks like one
	if isValidUsername(raw) {
		return ParsedChatRef{
			Kind:       ChatRefUsername,
			Username:   "@" + raw,
			InviteLink: "https://t.me/" + raw,
		}
	}

	return ParsedChatRef{
		Kind: ChatRefInvalid,
		Hint: "не удалось распознать ссылку, @username или ID",
	}
}

// isValidUsername returns true if s looks like a Telegram username:
// 5+ chars, [A-Za-z0-9_], starts with a letter.
func isValidUsername(s string) bool {
	if s == "" {
		return false
	}
	if len(s) < 4 || len(s) > 32 {
		// Telegram usernames are 5..32, but bots can be 4 — be lenient
	}
	first := s[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// BuildChatInfoConfig builds a tgbotapi.ChatInfoConfig usable with bot.GetChat
// from a parsed reference. Returns ok=false if reference is unusable.
func BuildChatInfoConfig(ref ParsedChatRef) (tgbotapi.ChatInfoConfig, bool) {
	switch ref.Kind {
	case ChatRefID:
		return tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: ref.ChatID}}, true
	case ChatRefUsername:
		return tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{SuperGroupUsername: ref.Username}}, true
	default:
		return tgbotapi.ChatInfoConfig{}, false
	}
}

// FormatChatRefError returns a friendly Russian error explaining why parsing failed.
func FormatChatRefError(ref ParsedChatRef) string {
	switch ref.Kind {
	case ChatRefPrivate:
		return "❌ Это приватная ссылка-приглашение.\n\n" +
			"Так канал добавить нельзя. Сделайте так:\n" +
			"1️⃣ Добавьте бота админом в этот канал\n" +
			"2️⃣ Перешлите сюда любой пост из канала\n\n" +
			"После этого канал будет добавлен автоматически."
	case ChatRefInvalid:
		hint := ref.Hint
		if hint == "" {
			hint = "не удалось распознать"
		}
		return fmt.Sprintf("❌ Не удалось распознать ввод (%s).\n\n"+
			"Поддерживаемые форматы:\n"+
			"• `https://t.me/имя_канала`\n"+
			"• `t.me/имя_канала`\n"+
			"• `@имя_канала`\n"+
			"• `имя_канала`\n"+
			"• `-1001234567890` (числовой ID)\n"+
			"• Перешлите пост из канала", hint)
	}
	return ""
}
