package telegramcmd

import (
	_ "embed"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"text/template"

	"github.com/quailyquaily/mistermorph/agent"
	"github.com/quailyquaily/mistermorph/internal/chathistory"
	"github.com/quailyquaily/mistermorph/internal/promptprofile"
	"github.com/quailyquaily/mistermorph/internal/prompttmpl"
)

//go:embed prompts/telegram_addressing_system.tmpl
var telegramAddressingSystemPromptTemplateSource string

//go:embed prompts/telegram_addressing_user.tmpl
var telegramAddressingUserPromptTemplateSource string

var addressingPromptTemplateFuncs = template.FuncMap{
	"toJSON": func(v any) (string, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	},
}

var telegramAddressingSystemPromptTemplate = prompttmpl.MustParse("telegram_addressing_system", telegramAddressingSystemPromptTemplateSource, nil)
var telegramAddressingUserPromptTemplate = prompttmpl.MustParse("telegram_addressing_user", telegramAddressingUserPromptTemplateSource, addressingPromptTemplateFuncs)

const addressingPromptPersonaFallback = "You are MisterMorph, a general-purpose AI agent that can use tools to complete tasks."

type telegramAddressingSystemPromptData struct {
	PersonaIdentity string
}

type telegramAddressingPromptSender struct {
	ID          int64  `json:"id,omitempty"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	IsBot       bool   `json:"is_bot,omitempty"`
	ChatID      int64  `json:"chat_id,omitempty"`
	ChatType    string `json:"chat_type,omitempty"`
}

type telegramAddressingPromptCurrentMessage struct {
	Text   string                         `json:"text"`
	Sender telegramAddressingPromptSender `json:"sender"`
}

type telegramAddressingUserPromptData struct {
	CurrentMessage      telegramAddressingPromptCurrentMessage
	ChatHistoryMessages []chathistory.ChatHistoryItem
}

func renderTelegramAddressingPrompts(currentMessage telegramAddressingPromptCurrentMessage, historyMessages []chathistory.ChatHistoryItem) (string, string, error) {
	personaIdentity := loadAddressingPersonaIdentity()
	if personaIdentity == "" {
		personaIdentity = addressingPromptPersonaFallback
	}

	systemPrompt, err := prompttmpl.Render(telegramAddressingSystemPromptTemplate, telegramAddressingSystemPromptData{
		PersonaIdentity: personaIdentity,
	})
	if err != nil {
		return "", "", err
	}
	userPrompt, err := prompttmpl.Render(telegramAddressingUserPromptTemplate, telegramAddressingUserPromptData{
		CurrentMessage:      currentMessage,
		ChatHistoryMessages: historyMessages,
	})
	if err != nil {
		return "", "", err
	}
	return systemPrompt, userPrompt, nil
}

var silentPromptProfileLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func loadAddressingPersonaIdentity() string {
	spec := agent.PromptSpec{}
	promptprofile.ApplyPersonaIdentity(&spec, silentPromptProfileLogger)
	persona := strings.TrimSpace(spec.Identity)
	if persona == "" {
		return ""
	}
	return persona
}
