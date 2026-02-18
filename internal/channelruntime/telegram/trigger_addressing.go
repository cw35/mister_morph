package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/quailyquaily/mistermorph/internal/chathistory"
	"github.com/quailyquaily/mistermorph/internal/grouptrigger"
	"github.com/quailyquaily/mistermorph/internal/jsonutil"
	"github.com/quailyquaily/mistermorph/internal/llminspect"
	"github.com/quailyquaily/mistermorph/llm"
	"github.com/quailyquaily/mistermorph/tools"
)

type telegramAddressingLLMOutput struct {
	Addressed      bool                           `json:"addressed"`
	Confidence     float64                        `json:"confidence"`
	WannaInterject telegramWannaInterjectDecision `json:"wanna_interject"`
	Interject      float64                        `json:"interject"`
	Impulse        float64                        `json:"impulse"`
	IsLightweight  bool                           `json:"is_lightweight"`
	Reason         string                         `json:"reason"`
}

// telegramWannaInterjectDecision accepts either bool or numeric values.
// Prompt iterations can produce true/false or 0..1; we normalize both to bool.
type telegramWannaInterjectDecision bool

func (w *telegramWannaInterjectDecision) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*w = false
		return nil
	}

	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*w = telegramWannaInterjectDecision(b)
		return nil
	}

	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		*w = telegramWannaInterjectDecision(f >= 0.5)
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		normalized := strings.ToLower(strings.TrimSpace(s))
		switch normalized {
		case "true":
			*w = true
			return nil
		case "false":
			*w = false
			return nil
		}
		fv, parseErr := strconv.ParseFloat(normalized, 64)
		if parseErr != nil {
			return fmt.Errorf("unsupported wanna_interject value: %q", s)
		}
		*w = telegramWannaInterjectDecision(fv >= 0.5)
		return nil
	}

	return fmt.Errorf("unsupported wanna_interject json: %s", raw)
}

func addressingDecisionViaLLM(
	ctx context.Context,
	client llm.Client,
	model string,
	msg *telegramMessage,
	text string,
	history []chathistory.ChatHistoryItem,
	addressingTool tools.Tool,
) (grouptrigger.Addressing, bool, error) {
	if ctx == nil || client == nil {
		return grouptrigger.Addressing{}, false, nil
	}
	text = strings.TrimSpace(text)
	model = strings.TrimSpace(model)
	if model == "" {
		return grouptrigger.Addressing{}, false, fmt.Errorf("missing model for addressing_llm")
	}

	historyMessages := chathistory.BuildMessages(chathistory.ChannelTelegram, history)
	currentMessage := telegramAddressingPromptCurrentMessage{
		Text: text,
	}
	if msg != nil {
		if msg.From != nil {
			currentMessage.Sender.ID = msg.From.ID
			currentMessage.Sender.IsBot = msg.From.IsBot
			currentMessage.Sender.Username = strings.TrimSpace(msg.From.Username)
			currentMessage.Sender.DisplayName = strings.TrimSpace(telegramDisplayName(msg.From))
		}
		if msg.Chat != nil {
			currentMessage.Sender.ChatID = msg.Chat.ID
			currentMessage.Sender.ChatType = strings.TrimSpace(msg.Chat.Type)
		}
	}
	sys, user, err := renderTelegramAddressingPrompts(currentMessage, historyMessages)
	if err != nil {
		return grouptrigger.Addressing{}, false, fmt.Errorf("render addressing prompts: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}
	var llmTools []llm.Tool
	if t := llmToolFromTool(addressingTool); t != nil {
		llmTools = append(llmTools, *t)
	}

	const maxToolRounds = 3
	var out telegramAddressingLLMOutput
	for round := 0; ; round++ {
		res, err := client.Chat(llminspect.WithModelScene(ctx, "telegram.addressing_decision"), llm.Request{
			Model:     model,
			ForceJSON: true,
			Messages:  messages,
			Tools:     llmTools,
		})
		if err != nil {
			return grouptrigger.Addressing{}, false, err
		}

		if len(res.ToolCalls) > 0 {
			if round >= maxToolRounds {
				return grouptrigger.Addressing{}, false, fmt.Errorf("addressing_llm exceeded tool-call rounds")
			}
			messages = append(messages, llm.Message{
				Role:      "assistant",
				Content:   strings.TrimSpace(res.Text),
				ToolCalls: res.ToolCalls,
			})
			for _, tc := range res.ToolCalls {
				observation := executeAddressingToolCall(ctx, addressingTool, tc)
				if strings.TrimSpace(tc.ID) != "" {
					messages = append(messages, llm.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    observation,
					})
					continue
				}
				messages = append(messages, llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("Tool Result (%s):\n%s", strings.TrimSpace(tc.Name), observation),
				})
			}
			continue
		}

		raw := strings.TrimSpace(res.Text)
		if raw == "" {
			return grouptrigger.Addressing{}, false, fmt.Errorf("empty addressing_llm response")
		}
		if err := jsonutil.DecodeWithFallback(raw, &out); err != nil {
			return grouptrigger.Addressing{}, false, fmt.Errorf("invalid addressing_llm json")
		}
		break
	}

	addressing := grouptrigger.Addressing{
		Addressed:      out.Addressed,
		Confidence:     clampAddressing01(out.Confidence),
		WannaInterject: bool(out.WannaInterject),
		Interject:      clampAddressing01(out.Interject),
		Impulse:        clampAddressing01(out.Impulse),
		IsLightweight:  out.IsLightweight,
		Reason:         strings.TrimSpace(out.Reason),
	}
	return addressing, true, nil
}

func llmToolFromTool(t tools.Tool) *llm.Tool {
	if t == nil {
		return nil
	}
	name := strings.TrimSpace(t.Name())
	if name == "" {
		return nil
	}
	return &llm.Tool{
		Name:           name,
		Description:    strings.TrimSpace(t.Description()),
		ParametersJSON: strings.TrimSpace(t.ParameterSchema()),
	}
}

func executeAddressingToolCall(ctx context.Context, t tools.Tool, call llm.ToolCall) string {
	name := strings.TrimSpace(call.Name)
	if t == nil || !strings.EqualFold(strings.TrimSpace(t.Name()), name) {
		if name == "" {
			name = "<empty>"
		}
		return fmt.Sprintf("error: tool '%s' not found", name)
	}
	observation, err := t.Execute(ctx, call.Arguments)
	if err != nil {
		if strings.TrimSpace(observation) == "" {
			observation = fmt.Sprintf("error: %s", err.Error())
		} else {
			observation = fmt.Sprintf("%s\n\nerror: %s", observation, err.Error())
		}
	}
	if strings.TrimSpace(observation) == "" {
		return "ok"
	}
	return observation
}

func clampAddressing01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
