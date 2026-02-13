package agent

import (
	_ "embed"
	"strings"

	"github.com/quailyquaily/mistermorph/tools"
)

//go:embed prompts/block_plan_create.tmpl
var planCreateBlockTemplateSource string

const planCreatePromptBlockTitle = "Plan Create Guidance"

func augmentPromptSpecForRegistry(spec PromptSpec, registry *tools.Registry) PromptSpec {
	if registry == nil {
		return spec
	}
	if _, ok := registry.Get("plan_create"); !ok {
		return spec
	}

	out := spec
	out.Blocks = append([]PromptBlock{}, spec.Blocks...)
	out.Blocks = appendPromptBlock(out.Blocks, PromptBlock{
		Title:   planCreatePromptBlockTitle,
		Content: strings.TrimSpace(planCreateBlockTemplateSource),
	})
	return out
}

func appendPromptBlock(blocks []PromptBlock, block PromptBlock) []PromptBlock {
	title := strings.TrimSpace(block.Title)
	content := strings.TrimSpace(block.Content)
	if title == "" || content == "" {
		return blocks
	}
	for _, existing := range blocks {
		if strings.EqualFold(strings.TrimSpace(existing.Title), title) &&
			strings.TrimSpace(existing.Content) == content {
			return blocks
		}
	}
	return append(blocks, PromptBlock{Title: title, Content: content})
}
