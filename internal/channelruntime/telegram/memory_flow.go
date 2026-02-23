package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/quailyquaily/mistermorph/agent"
	"github.com/quailyquaily/mistermorph/internal/chathistory"
	"github.com/quailyquaily/mistermorph/internal/entryutil"
	"github.com/quailyquaily/mistermorph/internal/jsonutil"
	"github.com/quailyquaily/mistermorph/internal/llminspect"
	"github.com/quailyquaily/mistermorph/llm"
	"github.com/quailyquaily/mistermorph/memory"
)

func updateMemoryFromJob(ctx context.Context, logger *slog.Logger, client llm.Client, model string, mgr *memory.Manager, longTermSubjectID string, job telegramJob, history []chathistory.ChatHistoryItem, historyCap int, final *agent.Final, requestTimeout time.Duration) error {
	if mgr == nil || client == nil {
		return nil
	}
	output := formatFinalOutput(final)
	date := time.Now().UTC()
	longTermSubjectID = strings.TrimSpace(longTermSubjectID)
	meta := buildMemoryWriteMeta(job)

	ctxInfo := MemoryDraftContext{
		SessionID:          meta.SessionID,
		ChatID:             job.ChatID,
		ChatType:           job.ChatType,
		CounterpartyID:     job.FromUserID,
		CounterpartyName:   strings.TrimSpace(job.FromDisplayName),
		CounterpartyHandle: strings.TrimSpace(job.FromUsername),
		TimestampUTC:       time.Now().UTC().Format(time.RFC3339),
	}
	if ctxInfo.CounterpartyName == "" {
		ctxInfo.CounterpartyName = strings.TrimSpace(strings.Join([]string{job.FromFirstName, job.FromLastName}, " "))
	}
	_, existingContent, hasExisting, err := mgr.LoadShortTerm(date, meta.SessionID)
	if err != nil {
		return err
	}

	memCtx := ctx
	cancel := func() {}
	if requestTimeout > 0 {
		memCtx, cancel = context.WithTimeout(ctx, requestTimeout)
	}
	defer cancel()
	ctxInfo.CounterpartyLabel = buildMemoryCounterpartyLabel(meta, ctxInfo)

	draftHistory := buildMemoryDraftHistory(history, job, output, date, historyCap)
	draft, err := BuildMemoryDraft(memCtx, client, model, draftHistory, job.Text, output, existingContent, ctxInfo)
	if err != nil {
		return err
	}
	draft.Promote = EnforceLongTermPromotionRules(draft.Promote, nil, job.Text)

	var mergedContent memory.ShortTermContent
	if hasExisting && HasDraftContent(draft) {
		semantic, mergeErr := SemanticMergeShortTerm(memCtx, client, model, existingContent, draft)
		if mergeErr != nil {
			return mergeErr
		}
		mergedContent = semantic
	} else {
		createdAt := date.UTC().Format(entryutil.TimestampLayout)
		mergedContent = memory.MergeShortTerm(existingContent, draft, createdAt)
	}

	_, err = mgr.WriteShortTerm(date, mergedContent, meta)
	if err != nil {
		return err
	}
	if longTermSubjectID != "" {
		if _, err := mgr.UpdateLongTerm(longTermSubjectID, draft.Promote); err != nil {
			return err
		}
	}
	if logger != nil {
		attrs := []any{"session_id", meta.SessionID}
		if longTermSubjectID != "" {
			attrs = append(attrs, "subject_id", longTermSubjectID)
		}
		logger.Debug("memory_update_ok", attrs...)
	}
	return nil
}

const heartbeatMemorySessionID = "heartbeat"

func buildMemoryWriteMeta(job telegramJob) memory.WriteMeta {
	if job.IsHeartbeat {
		return memory.WriteMeta{SessionID: heartbeatMemorySessionID}
	}
	meta := memory.WriteMeta{SessionID: fmt.Sprintf("tg:%d", job.ChatID)}
	if contactID := telegramMemoryContactID(job.FromUsername, job.FromUserID); contactID != "" {
		meta.ContactIDs = []string{contactID}
	}
	contactNickname := strings.TrimSpace(job.FromDisplayName)
	if contactNickname == "" {
		contactNickname = strings.TrimSpace(strings.Join([]string{job.FromFirstName, job.FromLastName}, " "))
	}
	if contactNickname != "" {
		meta.ContactNicknames = []string{contactNickname}
	}
	return meta
}

func buildMemoryDraftHistory(history []chathistory.ChatHistoryItem, job telegramJob, output string, now time.Time, maxItems int) []chathistory.ChatHistoryItem {
	out := append([]chathistory.ChatHistoryItem{}, history...)
	out = append(out, newTelegramInboundHistoryItem(job))
	if strings.TrimSpace(output) != "" {
		out = append(out, newTelegramOutboundAgentHistoryItem(job.ChatID, job.ChatType, output, now, ""))
	}
	if maxItems > 0 && len(out) > maxItems {
		out = out[len(out)-maxItems:]
	}
	return out
}

type MemoryDraftContext struct {
	SessionID          string `json:"session_id,omitempty"`
	ChatID             int64  `json:"chat_id,omitempty"`
	ChatType           string `json:"chat_type,omitempty"`
	CounterpartyID     int64  `json:"counterparty_id,omitempty"`
	CounterpartyName   string `json:"counterparty_name,omitempty"`
	CounterpartyHandle string `json:"counterparty_handle,omitempty"`
	CounterpartyLabel  string `json:"counterparty_label,omitempty"`
	TimestampUTC       string `json:"timestamp_utc,omitempty"`
}

func buildMemoryCounterpartyLabel(meta memory.WriteMeta, ctxInfo MemoryDraftContext) string {
	contactID := firstNonEmptyString(meta.ContactIDs...)
	if contactID == "" {
		contactID = strings.TrimSpace(ctxInfo.CounterpartyHandle)
	}
	nickname := firstNonEmptyString(meta.ContactNicknames...)
	if nickname == "" {
		nickname = strings.TrimSpace(ctxInfo.CounterpartyName)
	}
	if nickname != "" && contactID != "" {
		if strings.EqualFold(nickname, contactID) {
			return nickname
		}
		return nickname + "(" + contactID + ")"
	}
	if nickname != "" {
		return nickname
	}
	return contactID
}

func firstNonEmptyString(values ...string) string {
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v != "" {
			return v
		}
	}
	return ""
}

func BuildMemoryDraft(ctx context.Context, client llm.Client, model string, history []chathistory.ChatHistoryItem, task string, output string, existing memory.ShortTermContent, ctxInfo MemoryDraftContext) (memory.SessionDraft, error) {
	if client == nil {
		return memory.SessionDraft{}, fmt.Errorf("nil llm client")
	}

	sys, user, err := renderMemoryDraftPrompts(ctxInfo, history, task, output, existing)
	if err != nil {
		return memory.SessionDraft{}, fmt.Errorf("render memory draft prompts: %w", err)
	}

	res, err := client.Chat(llminspect.WithModelScene(ctx, "memory.draft"), llm.Request{
		Model:     model,
		ForceJSON: true,
		Messages: []llm.Message{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
		Parameters: map[string]any{
			"max_tokens": 10240,
		},
	})
	if err != nil {
		return memory.SessionDraft{}, err
	}

	raw := strings.TrimSpace(res.Text)
	if raw == "" {
		return memory.SessionDraft{}, fmt.Errorf("empty memory draft response")
	}

	var out memory.SessionDraft
	if err := jsonutil.DecodeWithFallback(raw, &out); err != nil {
		return memory.SessionDraft{}, fmt.Errorf("invalid memory draft json")
	}
	out.SummaryItems = normalizeMemorySummaryItems(out.SummaryItems)
	return out, nil
}

func EnforceLongTermPromotionRules(promote memory.PromoteDraft, history []llm.Message, task string) memory.PromoteDraft {
	if !hasExplicitMemoryRequest(history, task) {
		return memory.PromoteDraft{}
	}
	return limitPromoteToOne(promote)
}

func hasExplicitMemoryRequest(history []llm.Message, task string) bool {
	texts := make([]string, 0, len(history)+1)
	for _, m := range history {
		if strings.ToLower(strings.TrimSpace(m.Role)) != "user" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		texts = append(texts, content)
	}
	if strings.TrimSpace(task) != "" {
		texts = append(texts, task)
	}
	if len(texts) == 0 {
		return false
	}
	combined := strings.ToLower(strings.Join(texts, "\n"))
	return containsExplicitMemoryRequest(combined)
}

func containsExplicitMemoryRequest(lowerText string) bool {
	if strings.TrimSpace(lowerText) == "" {
		return false
	}
	keywords := []string{
		"记住",
		"记下来",
		"别忘",
		"记得",
		"长期记忆",
		"写入长期记忆",
		"加入长期记忆",
		"记到长期",
		"remember",
		"don't forget",
		"dont forget",
		"long-term memory",
		"add to memory",
		"save this",
		"keep this",
		"store this",
		"memorize",
	}
	for _, k := range keywords {
		if strings.Contains(lowerText, k) {
			return true
		}
	}
	return false
}

func limitPromoteToOne(promote memory.PromoteDraft) memory.PromoteDraft {
	if item, ok := firstNonEmptyText(promote.GoalsProjects); ok {
		return memory.PromoteDraft{GoalsProjects: []string{item}}
	}
	if item, ok := firstKVItem(promote.KeyFacts); ok {
		return memory.PromoteDraft{KeyFacts: []memory.KVItem{item}}
	}
	return memory.PromoteDraft{}
}

func firstNonEmptyText(items []string) (string, bool) {
	for _, raw := range items {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		return v, true
	}
	return "", false
}

func firstKVItem(items []memory.KVItem) (memory.KVItem, bool) {
	for _, it := range items {
		title := strings.TrimSpace(it.Title)
		value := strings.TrimSpace(it.Value)
		if title == "" && value == "" {
			continue
		}
		it.Title = title
		it.Value = value
		return it, true
	}
	return memory.KVItem{}, false
}

func SemanticMergeShortTerm(ctx context.Context, client llm.Client, model string, existing memory.ShortTermContent, draft memory.SessionDraft) (memory.ShortTermContent, error) {
	if client == nil {
		return memory.ShortTermContent{}, fmt.Errorf("nil llm client")
	}
	incoming := memory.MergeShortTerm(memory.ShortTermContent{}, draft, time.Now().UTC().Format(entryutil.TimestampLayout))
	if len(incoming.SummaryItems) == 0 {
		return memory.NormalizeShortTermContent(existing), nil
	}
	combined := make([]memory.SummaryItem, 0, len(incoming.SummaryItems)+len(existing.SummaryItems))
	combined = append(combined, incoming.SummaryItems...)
	combined = append(combined, existing.SummaryItems...)

	resolver := entryutil.NewLLMSemanticResolver(client, model)
	deduped, err := memory.SemanticDedupeSummaryItems(llminspect.WithModelScene(ctx, "memory.semantic_dedupe"), combined, resolver)
	if err != nil {
		return memory.ShortTermContent{}, err
	}

	merged := memory.NormalizeShortTermContent(memory.ShortTermContent{SummaryItems: deduped})
	return merged, nil
}

func HasDraftContent(draft memory.SessionDraft) bool {
	return len(normalizeMemorySummaryItems(draft.SummaryItems)) > 0
}

func normalizeMemorySummaryItems(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	out := make([]string, 0, len(input))
	seen := make(map[string]bool, len(input))
	for _, raw := range input {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
