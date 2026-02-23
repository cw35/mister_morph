package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/quailyquaily/mistermorph/agent"
	"github.com/quailyquaily/mistermorph/guard"
	busruntime "github.com/quailyquaily/mistermorph/internal/bus"
	"github.com/quailyquaily/mistermorph/internal/chathistory"
	"github.com/quailyquaily/mistermorph/internal/llminspect"
	"github.com/quailyquaily/mistermorph/internal/promptprofile"
	"github.com/quailyquaily/mistermorph/internal/retryutil"
	"github.com/quailyquaily/mistermorph/internal/statepaths"
	"github.com/quailyquaily/mistermorph/internal/todo"
	"github.com/quailyquaily/mistermorph/internal/toolsutil"
	"github.com/quailyquaily/mistermorph/llm"
	"github.com/quailyquaily/mistermorph/memory"
	"github.com/quailyquaily/mistermorph/tools"
	telegramtools "github.com/quailyquaily/mistermorph/tools/telegram"
)

type runtimeTaskOptions struct {
	MemoryEnabled               bool
	MemoryShortTermDays         int
	MemoryInjectionEnabled      bool
	MemoryInjectionMaxItems     int
	SecretsRequireSkillProfiles bool
}

func runTelegramTask(ctx context.Context, d Dependencies, logger *slog.Logger, logOpts agent.LogOptions, client llm.Client, baseReg *tools.Registry, api *telegramAPI, filesEnabled bool, fileCacheDir string, filesMaxBytes int64, sharedGuard *guard.Guard, cfg agent.Config, allowedIDs map[int64]bool, job telegramJob, botUsername string, model string, history []chathistory.ChatHistoryItem, historyCap int, stickySkills []string, requestTimeout time.Duration, runtimeOpts runtimeTaskOptions, sendTelegramText func(context.Context, int64, string, string) error) (*agent.Final, *agent.Context, []string, *telegramtools.Reaction, error) {
	if sendTelegramText == nil {
		return nil, nil, nil, nil, fmt.Errorf("send telegram text callback is required")
	}
	task := job.Text
	historyWithCurrent := append([]chathistory.ChatHistoryItem(nil), history...)
	if !job.IsHeartbeat {
		historyWithCurrent = append(historyWithCurrent, newTelegramInboundHistoryItem(job))
	}
	historyRaw, err := json.MarshalIndent(map[string]any{
		"chat_history_messages": chathistory.BuildMessages(chathistory.ChannelTelegram, historyWithCurrent),
	}, "", "  ")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("render telegram history context: %w", err)
	}
	llmHistory := []llm.Message{{Role: "user", Content: string(historyRaw)}}
	if baseReg == nil {
		baseReg = registryFromDeps(d)
		toolsutil.BindTodoUpdateToolLLM(baseReg, client, model)
	}

	// Per-run registry.
	reg := buildTelegramRegistry(baseReg, job.ChatType)
	registerPlanTool(d, reg, client, model)
	toolsutil.BindTodoUpdateToolLLM(reg, client, model)
	toolsutil.SetTodoUpdateToolAddContext(reg, todo.AddResolveContext{
		Channel:          "telegram",
		ChatType:         job.ChatType,
		ChatID:           job.ChatID,
		SpeakerUserID:    job.FromUserID,
		SpeakerUsername:  job.FromUsername,
		MentionUsernames: append([]string(nil), job.MentionUsers...),
		UserInputRaw:     job.Text,
	})
	toolAPI := newTelegramToolAPI(api)
	if api != nil {
		reg.Register(telegramtools.NewSendVoiceTool(toolAPI, job.ChatID, fileCacheDir, filesMaxBytes, nil))
		if filesEnabled {
			reg.Register(telegramtools.NewSendFileTool(toolAPI, job.ChatID, fileCacheDir, filesMaxBytes))
		}
	}
	var reactTool *telegramtools.ReactTool
	if api != nil && job.MessageID != 0 {
		reactTool = telegramtools.NewReactTool(toolAPI, job.ChatID, job.MessageID, allowedIDs)
		reg.Register(reactTool)
	}

	promptSpec, loadedSkills, skillAuthProfiles, err := promptSpecForTelegram(d, ctx, logger, logOpts, task, client, model, stickySkills)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	promptprofile.ApplyPersonaIdentity(&promptSpec, logger)
	promptprofile.AppendLocalToolNotesBlock(&promptSpec, logger)
	promptprofile.AppendPlanCreateGuidanceBlock(&promptSpec, reg)
	promptprofile.AppendTelegramRuntimeBlocks(&promptSpec, isGroupChat(job.ChatType), job.MentionUsers)

	var memManager *memory.Manager
	var memIdentity memory.Identity
	if runtimeOpts.MemoryEnabled {
		memManager = memory.NewManager(statepaths.MemoryDir(), runtimeOpts.MemoryShortTermDays)
		if job.FromUserID > 0 {
			memReqCtx := memory.ContextPublic
			if strings.ToLower(strings.TrimSpace(job.ChatType)) == "private" {
				memReqCtx = memory.ContextPrivate
			}
			id, err := (&memory.Resolver{}).ResolveTelegram(ctx, job.FromUserID)
			if err != nil {
				return nil, nil, loadedSkills, nil, fmt.Errorf("memory identity: %w", err)
			}
			if id.Enabled && strings.TrimSpace(id.SubjectID) != "" {
				memIdentity = id
				if runtimeOpts.MemoryInjectionEnabled {
					maxItems := runtimeOpts.MemoryInjectionMaxItems
					snap, err := memManager.BuildInjection(id.SubjectID, memReqCtx, maxItems)
					if err != nil {
						return nil, nil, loadedSkills, nil, fmt.Errorf("memory injection: %w", err)
					}
					if strings.TrimSpace(snap) != "" {
						promptprofile.AppendMemorySummariesBlock(&promptSpec, snap)
						if logger != nil {
							logger.Info("memory_injection_applied", "source", "telegram", "subject_id", id.SubjectID, "chat_id", job.ChatID, "snapshot_len", len(snap))
						}
					} else if logger != nil {
						logger.Debug("memory_injection_skipped", "source", "telegram", "reason", "empty_snapshot", "subject_id", id.SubjectID, "chat_id", job.ChatID)
					}
				} else if logger != nil {
					logger.Debug("memory_injection_skipped", "source", "telegram", "reason", "disabled")
				}
			} else if logger != nil {
				logger.Debug("memory_identity_unavailable", "source", "telegram", "enabled", id.Enabled, "subject_id", strings.TrimSpace(id.SubjectID))
			}
		} else if logger != nil && !job.IsHeartbeat {
			logger.Debug("memory_identity_unavailable", "source", "telegram", "reason", "missing_user_id")
		}
	}

	var planUpdateHook func(runCtx *agent.Context, update agent.PlanStepUpdate)
	if !job.IsHeartbeat {
		planUpdateHook = func(runCtx *agent.Context, update agent.PlanStepUpdate) {
			if runCtx == nil || runCtx.Plan == nil {
				return
			}
			msg, err := generateTelegramPlanProgressMessage(ctx, client, model, task, runCtx.Plan, update, requestTimeout)
			if err != nil {
				logger.Warn("telegram_plan_progress_error", "error", err.Error())
				return
			}
			if strings.TrimSpace(msg) == "" {
				return
			}
			correlationID := fmt.Sprintf("telegram:plan:%d:%d", job.ChatID, job.MessageID)
			if err := sendTelegramText(context.Background(), job.ChatID, msg, correlationID); err != nil {
				logger.Warn("telegram_bus_publish_error", "channel", busruntime.ChannelTelegram, "chat_id", job.ChatID, "message_id", job.MessageID, "bus_error_code", busErrorCodeString(err), "error", err.Error())
			}
		}
	}

	engineOpts := []agent.Option{
		agent.WithLogger(logger),
		agent.WithLogOptions(logOpts),
		agent.WithSkillAuthProfiles(skillAuthProfiles, runtimeOpts.SecretsRequireSkillProfiles),
		agent.WithGuard(sharedGuard),
	}
	if planUpdateHook != nil {
		engineOpts = append(engineOpts, agent.WithPlanStepUpdate(planUpdateHook))
	}
	engine := agent.New(
		client,
		reg,
		cfg,
		promptSpec,
		engineOpts...,
	)
	meta := job.Meta
	if meta == nil {
		meta = map[string]any{
			"trigger":               "telegram",
			"telegram_chat_id":      job.ChatID,
			"telegram_message_id":   job.MessageID,
			"telegram_chat_type":    job.ChatType,
			"telegram_from_user_id": job.FromUserID,
		}
	}
	botUsername = strings.TrimPrefix(strings.TrimSpace(botUsername), "@")
	if botUsername != "" {
		meta["telegram_bot_username"] = botUsername
	}
	final, agentCtx, err := engine.Run(ctx, task, agent.RunOptions{
		Model:           model,
		History:         llmHistory,
		Meta:            meta,
		SkipTaskMessage: true,
	})
	if err != nil {
		return final, agentCtx, loadedSkills, nil, err
	}

	var reaction *telegramtools.Reaction
	if reactTool != nil {
		reaction = reactTool.LastReaction()
		if reaction != nil && logger != nil {
			logger.Info("telegram_reaction_applied",
				"chat_id", reaction.ChatID,
				"message_id", reaction.MessageID,
				"emoji", reaction.Emoji,
				"source", reaction.Source,
			)
		}
	}

	publishText := shouldPublishTelegramText(final)

	longTermSubjectID := resolveLongTermSubjectID(job, memIdentity)
	if shouldWriteMemory(publishText, memManager, longTermSubjectID) {
		if err := updateMemoryFromJob(ctx, logger, client, model, memManager, longTermSubjectID, job, history, historyCap, final, requestTimeout); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				retryutil.AsyncRetry(logger, "memory_update", 2*time.Second, requestTimeout, func(retryCtx context.Context) error {
					return updateMemoryFromJob(retryCtx, logger, client, model, memManager, longTermSubjectID, job, history, historyCap, final, requestTimeout)
				})
			}
			logger.Warn("memory_update_error", "error", err.Error())
		}
	}
	return final, agentCtx, loadedSkills, reaction, nil
}

func resolveLongTermSubjectID(job telegramJob, memIdentity memory.Identity) string {
	if job.IsHeartbeat {
		return heartbeatMemorySessionID
	}
	if !memIdentity.Enabled {
		return ""
	}
	return strings.TrimSpace(memIdentity.SubjectID)
}

func shouldWriteMemory(publishText bool, memManager *memory.Manager, longTermSubjectID string) bool {
	if !publishText || memManager == nil {
		return false
	}
	return strings.TrimSpace(longTermSubjectID) != ""
}

func runMAEPTask(ctx context.Context, d Dependencies, logger *slog.Logger, logOpts agent.LogOptions, client llm.Client, baseReg *tools.Registry, sharedGuard *guard.Guard, cfg agent.Config, model string, peerID string, history []llm.Message, stickySkills []string, runtimeOpts runtimeTaskOptions, task string) (*agent.Final, *agent.Context, []string, error) {
	if strings.TrimSpace(task) == "" {
		return nil, nil, nil, fmt.Errorf("empty maep task")
	}
	if baseReg == nil {
		baseReg = registryFromDeps(d)
		toolsutil.BindTodoUpdateToolLLM(baseReg, client, model)
	}
	reg := buildMAEPRegistry(baseReg)
	registerPlanTool(d, reg, client, model)
	toolsutil.BindTodoUpdateToolLLM(reg, client, model)

	promptSpec, loadedSkills, skillAuthProfiles, err := promptSpecForTelegram(d, ctx, logger, logOpts, task, client, model, stickySkills)
	if err != nil {
		return nil, nil, nil, err
	}
	promptprofile.ApplyPersonaIdentity(&promptSpec, logger)
	promptprofile.AppendLocalToolNotesBlock(&promptSpec, logger)
	promptprofile.AppendPlanCreateGuidanceBlock(&promptSpec, reg)
	promptprofile.AppendMAEPReplyPolicyBlock(&promptSpec)

	engine := agent.New(
		client,
		reg,
		cfg,
		promptSpec,
		agent.WithLogger(logger),
		agent.WithLogOptions(logOpts),
		agent.WithSkillAuthProfiles(skillAuthProfiles, runtimeOpts.SecretsRequireSkillProfiles),
		agent.WithGuard(sharedGuard),
	)
	final, runCtx, err := engine.Run(ctx, task, agent.RunOptions{
		Model:   model,
		History: history,
		Meta: map[string]any{
			"trigger": "maep_inbound",
		},
	})
	if err != nil {
		return final, runCtx, loadedSkills, err
	}
	return final, runCtx, loadedSkills, nil
}

func buildMAEPRegistry(baseReg *tools.Registry) *tools.Registry {
	reg := tools.NewRegistry()
	if baseReg == nil {
		return reg
	}
	for _, t := range baseReg.All() {
		name := strings.TrimSpace(t.Name())
		if name == "contacts_send" {
			continue
		}
		reg.Register(t)
	}
	return reg
}

func buildTelegramRegistry(baseReg *tools.Registry, chatType string) *tools.Registry {
	reg := tools.NewRegistry()
	if baseReg == nil {
		return reg
	}
	groupChat := isGroupChat(chatType)
	for _, t := range baseReg.All() {
		name := strings.TrimSpace(t.Name())
		if groupChat && strings.EqualFold(name, "contacts_send") {
			continue
		}
		reg.Register(t)
	}
	return reg
}

func generateTelegramPlanProgressMessage(ctx context.Context, client llm.Client, model string, task string, plan *agent.Plan, update agent.PlanStepUpdate, requestTimeout time.Duration) (string, error) {
	if client == nil || plan == nil || update.CompletedIndex < 0 {
		return "", nil
	}
	total := len(plan.Steps)
	if total == 0 {
		return "", nil
	}

	completed := 0
	for i := range plan.Steps {
		if plan.Steps[i].Status == agent.PlanStatusCompleted {
			completed++
		}
	}

	payload := map[string]any{
		"task":             strings.TrimSpace(task),
		"plan_summary":     strings.TrimSpace(plan.Summary),
		"completed_index":  update.CompletedIndex,
		"completed_step":   strings.TrimSpace(update.CompletedStep),
		"next_index":       update.StartedIndex,
		"next_step":        strings.TrimSpace(update.StartedStep),
		"steps_completed":  completed,
		"steps_total":      total,
		"progress_percent": int(float64(completed) / float64(total) * 100),
	}
	systemPrompt, userPrompt, err := renderTelegramPlanProgressPrompts(payload)
	if err != nil {
		return "", err
	}

	req := llm.Request{
		Model:     model,
		ForceJSON: false,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Parameters: map[string]any{
			"max_tokens": 4096,
		},
	}

	if ctx == nil {
		ctx = context.Background()
	}
	planCtx := ctx
	cancel := func() {}
	if requestTimeout > 0 {
		planCtx, cancel = context.WithTimeout(ctx, requestTimeout)
	}
	defer cancel()

	result, err := client.Chat(llminspect.WithModelScene(planCtx, "telegram.plan_progress"), req)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Text), nil
}
