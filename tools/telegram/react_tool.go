package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type ReactTool struct {
	api              API
	defaultChatID    int64
	defaultMessageID int64
	allowedIDs       map[int64]bool
	allowedEmojis    map[string]bool
	lastReaction     *Reaction
}

var telegramStandardReactionEmojis = []string{
	"👍", "👎", "❤", "🔥", "🥰", "👏", "😁", "🤔", "🤯", "😱", "🤬", "😢", "🎉", "🤩", "🤮", "💩", "🙏", "👌", "🕊", "🤡", "🥱", "🥴", "😍", "🐳", "❤️‍🔥", "🌚", "🌭", "💯", "🤣", "⚡", "🍌", "🏆", "💔", "🤨", "😐", "🍓", "🍾", "💋", "🖕", "😈", "😴", "😭", "🤓", "👻", "👨‍💻", "👀", "🎃", "🙈", "😇", "😨", "🤝", "✍", "🤗", "🫡", "🎅", "🎄", "☃", "💅", "🤪", "🗿", "🆒", "💘", "🙉", "🦄", "😘", "💊", "🙊", "😎", "👾", "🤷‍♂️", "🤷", "🤷‍♀️", "😡",
}

func NewReactTool(api API, defaultChatID int64, defaultMessageID int64, allowedIDs map[int64]bool) *ReactTool {
	emojiSet := make(map[string]bool, len(telegramStandardReactionEmojis))
	for _, emoji := range telegramStandardReactionEmojis {
		emoji = strings.TrimSpace(emoji)
		if emoji == "" {
			continue
		}
		emojiSet[emoji] = true
	}
	return &ReactTool{
		api:              api,
		defaultChatID:    defaultChatID,
		defaultMessageID: defaultMessageID,
		allowedIDs:       allowedIDs,
		allowedEmojis:    emojiSet,
	}
}

func (t *ReactTool) Name() string { return "telegram_react" }

func (t *ReactTool) Description() string {
	return "Adds an emoji reaction to a Telegram message. Use when a light confirmation is sufficient; do not send an extra text reply when reaction alone is enough."
}

func (t *ReactTool) ParameterSchema() string {
	emojiDescription := "Emoji to react with. Allowed values: " + strings.Join(telegramStandardReactionEmojis, ",") + "."
	s := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"chat_id": map[string]any{
				"type":        "integer",
				"description": "Target Telegram chat_id. Optional in active chat context; required when reacting outside the current chat.",
			},
			"message_id": map[string]any{
				"type":        "integer",
				"description": "Target Telegram message_id. Optional in active chat context; defaults to the triggering message.",
			},
			"emoji": map[string]any{
				"type":        "string",
				"description": emojiDescription,
			},
			"is_big": map[string]any{
				"type":        "boolean",
				"description": "Optional big reaction flag.",
			},
		},
		"required": []string{"emoji"},
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return string(b)
}

func (t *ReactTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	if t == nil || t.api == nil {
		return "", fmt.Errorf("telegram_react is disabled")
	}

	chatID := t.defaultChatID
	if v, ok := params["chat_id"]; ok {
		switch x := v.(type) {
		case int64:
			chatID = x
		case int:
			chatID = int64(x)
		case float64:
			chatID = int64(x)
		}
	}
	if chatID == 0 {
		return "", fmt.Errorf("missing required param: chat_id")
	}
	if len(t.allowedIDs) > 0 && !t.allowedIDs[chatID] {
		return "", fmt.Errorf("unauthorized chat_id: %d", chatID)
	}

	messageID := t.defaultMessageID
	if v, ok := params["message_id"]; ok {
		switch x := v.(type) {
		case int64:
			messageID = x
		case int:
			messageID = int64(x)
		case float64:
			messageID = int64(x)
		}
	}
	if messageID == 0 {
		return "", fmt.Errorf("missing required param: message_id")
	}

	emoji, _ := params["emoji"].(string)
	emoji = strings.TrimSpace(emoji)
	if emoji == "" {
		return "", fmt.Errorf("missing required param: emoji")
	}
	if !t.allowedEmojis[emoji] {
		return "", fmt.Errorf("emoji is not in Telegram standard reactions list: %s", emoji)
	}

	var isBigPtr *bool
	if v, ok := params["is_big"]; ok {
		if b, ok := v.(bool); ok {
			isBig := b
			isBigPtr = &isBig
		}
	}

	if err := t.api.SetEmojiReaction(ctx, chatID, messageID, emoji, isBigPtr); err != nil {
		return "", err
	}

	t.lastReaction = &Reaction{
		ChatID:    chatID,
		MessageID: messageID,
		Emoji:     emoji,
		Source:    "tool",
	}
	return fmt.Sprintf("reacted with %s", emoji), nil
}

func (t *ReactTool) LastReaction() *Reaction {
	if t == nil {
		return nil
	}
	return t.lastReaction
}
