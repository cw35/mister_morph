package telegramcmd

import (
	_ "embed"
	"encoding/json"
	"text/template"

	"github.com/quailyquaily/mistermorph/internal/prompttmpl"
	"github.com/quailyquaily/mistermorph/llm"
)

//go:embed prompts/maep_feedback_system.tmpl
var maepFeedbackSystemPromptTemplateSource string

//go:embed prompts/maep_feedback_user.tmpl
var maepFeedbackUserPromptTemplateSource string

var maepPromptTemplateFuncs = template.FuncMap{
	"toJSON": func(v any) (string, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	},
}

var maepFeedbackSystemPromptTemplate = prompttmpl.MustParse("telegram_maep_feedback_system", maepFeedbackSystemPromptTemplateSource, nil)
var maepFeedbackUserPromptTemplate = prompttmpl.MustParse("telegram_maep_feedback_user", maepFeedbackUserPromptTemplateSource, maepPromptTemplateFuncs)

type maepFeedbackUserPromptData struct {
	RecentTurns  []llm.Message
	InboundText  string
	AllowedNext  []string
	SignalBounds string
}

func renderMAEPFeedbackPrompts(recentTurns []llm.Message, inboundText string) (string, string, error) {
	systemPrompt, err := prompttmpl.Render(maepFeedbackSystemPromptTemplate, struct{}{})
	if err != nil {
		return "", "", err
	}
	userPrompt, err := prompttmpl.Render(maepFeedbackUserPromptTemplate, maepFeedbackUserPromptData{
		RecentTurns:  recentTurns,
		InboundText:  inboundText,
		AllowedNext:  []string{"continue", "wrap_up", "switch_topic"},
		SignalBounds: "[0,1]",
	})
	if err != nil {
		return "", "", err
	}
	return systemPrompt, userPrompt, nil
}
