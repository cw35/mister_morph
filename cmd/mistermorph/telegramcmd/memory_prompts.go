package telegramcmd

import (
	_ "embed"
	"encoding/json"
	"text/template"

	"github.com/quailyquaily/mistermorph/internal/chathistory"
	"github.com/quailyquaily/mistermorph/internal/prompttmpl"
	"github.com/quailyquaily/mistermorph/memory"
)

//go:embed prompts/memory_draft_system.tmpl
var memoryDraftSystemPromptTemplateSource string

//go:embed prompts/memory_draft_user.tmpl
var memoryDraftUserPromptTemplateSource string

var memoryPromptTemplateFuncs = template.FuncMap{
	"toJSON": func(v any) (string, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	},
}

var memoryDraftSystemPromptTemplate = prompttmpl.MustParse("telegram_memory_draft_system", memoryDraftSystemPromptTemplateSource, nil)
var memoryDraftUserPromptTemplate = prompttmpl.MustParse("telegram_memory_draft_user", memoryDraftUserPromptTemplateSource, memoryPromptTemplateFuncs)

type memoryDraftUserPromptData struct {
	SessionContext       MemoryDraftContext
	ChatHistory          chathistory.ContextPayload
	CurrentTask          string
	CurrentOutput        string
	ExistingSummaryItems []memory.SummaryItem
}

func renderMemoryDraftPrompts(
	ctxInfo MemoryDraftContext,
	history []chathistory.ChatHistoryItem,
	task string,
	output string,
	existing memory.ShortTermContent,
) (string, string, error) {
	systemPrompt, err := prompttmpl.Render(memoryDraftSystemPromptTemplate, struct{}{})
	if err != nil {
		return "", "", err
	}
	userPrompt, err := prompttmpl.Render(memoryDraftUserPromptTemplate, memoryDraftUserPromptData{
		SessionContext:       ctxInfo,
		ChatHistory:          chathistory.BuildContextPayload(chathistory.ChannelTelegram, history),
		CurrentTask:          task,
		CurrentOutput:        output,
		ExistingSummaryItems: existing.SummaryItems,
	})
	if err != nil {
		return "", "", err
	}
	return systemPrompt, userPrompt, nil
}
