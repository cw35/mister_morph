package telegramcmd

import (
	_ "embed"
	"strings"
	"text/template"

	"github.com/quailyquaily/mistermorph/agent"
	"github.com/quailyquaily/mistermorph/internal/prompttmpl"
)

const (
	telegramPromptBlockTitleRuntime   = "Telegram Runtime Rules"
	telegramPromptBlockTitleMAEPReply = "MAEP Reply Policy"
)

//go:embed prompts/telegram_block.tmpl
var telegramRuntimePromptBlockTemplateSource string

//go:embed prompts/maep_block.tmpl
var maepReplyPromptBlockSource string

var telegramRuntimePromptBlockTemplate = prompttmpl.MustParse(
	"telegram_runtime_block",
	telegramRuntimePromptBlockTemplateSource,
	template.FuncMap{},
)

type telegramRuntimePromptBlockData struct {
	IsGroup bool
}

func applyTelegramRuntimePromptBlocks(spec *agent.PromptSpec, chatType string, mentionUsers []string) {
	if spec == nil {
		return
	}
	content, err := prompttmpl.Render(telegramRuntimePromptBlockTemplate, telegramRuntimePromptBlockData{
		IsGroup: isGroupChat(chatType),
	})
	if err == nil {
		appendTelegramPromptBlock(spec, telegramPromptBlockTitleRuntime, content)
	}

	if !isGroupChat(chatType) {
		return
	}
	if len(mentionUsers) > 0 {
		spec.Blocks = append(spec.Blocks, agent.PromptBlock{
			Title:   "[Group Usernames]",
			Content: strings.Join(mentionUsers, "\n"),
		})
	}
}

func applyMAEPReplyPromptRules(spec *agent.PromptSpec) {
	if spec == nil {
		return
	}
	appendTelegramPromptBlock(spec, telegramPromptBlockTitleMAEPReply, maepReplyPromptBlockSource)
}

func appendTelegramPromptBlock(spec *agent.PromptSpec, title string, content string) {
	if spec == nil {
		return
	}
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" || content == "" {
		return
	}
	for _, blk := range spec.Blocks {
		if strings.EqualFold(strings.TrimSpace(blk.Title), title) &&
			strings.TrimSpace(blk.Content) == content {
			return
		}
	}
	spec.Blocks = append(spec.Blocks, agent.PromptBlock{
		Title:   title,
		Content: content,
	})
}
