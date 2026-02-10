package telegramcmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/quailyquaily/mistermorph/agent"
	"github.com/quailyquaily/mistermorph/guard"
	"github.com/quailyquaily/mistermorph/internal/llmconfig"
	"github.com/quailyquaily/mistermorph/llm"
	"github.com/quailyquaily/mistermorph/tools"
	"github.com/spf13/cobra"
)

type Dependencies struct {
	LoggerFromViper                func() (*slog.Logger, error)
	LogOptionsFromViper            func() agent.LogOptions
	CreateLLMClient                func(provider, endpoint, apiKey, model string, timeout time.Duration) (llm.Client, error)
	LLMProviderFromViper           func() string
	LLMEndpointForProvider         func(provider string) string
	LLMAPIKeyForProvider           func(provider string) string
	LLMModelForProvider            func(provider string) string
	RegistryFromViper              func() *tools.Registry
	RegisterPlanTool               func(reg *tools.Registry, client llm.Client, model string)
	GuardFromViper                 func(logger *slog.Logger) *guard.Guard
	PromptSpecForTelegram          func(ctx context.Context, logger *slog.Logger, logOpts agent.LogOptions, task string, client llm.Client, model string, stickySkills []string) (agent.PromptSpec, []string, []string, error)
	FormatFinalOutput              func(final *agent.Final) string
	BuildHeartbeatTask             func(checklistPath string) (string, bool, error)
	BuildHeartbeatMeta             func(source string, interval time.Duration, checklistPath string, checklistEmpty bool, extra map[string]any) map[string]any
}

var deps Dependencies

func NewCommand(d Dependencies) *cobra.Command {
	deps = d
	return newTelegramCmd()
}

func loggerFromViper() (*slog.Logger, error) {
	if deps.LoggerFromViper == nil {
		return nil, fmt.Errorf("LoggerFromViper dependency missing")
	}
	return deps.LoggerFromViper()
}

func logOptionsFromViper() agent.LogOptions {
	if deps.LogOptionsFromViper == nil {
		return agent.LogOptions{}
	}
	return deps.LogOptionsFromViper()
}

func llmProviderFromViper() string {
	if deps.LLMProviderFromViper == nil {
		return ""
	}
	return deps.LLMProviderFromViper()
}

func llmEndpointForProvider(provider string) string {
	if deps.LLMEndpointForProvider == nil {
		return ""
	}
	return deps.LLMEndpointForProvider(provider)
}

func llmAPIKeyForProvider(provider string) string {
	if deps.LLMAPIKeyForProvider == nil {
		return ""
	}
	return deps.LLMAPIKeyForProvider(provider)
}

func llmModelForProvider(provider string) string {
	if deps.LLMModelForProvider == nil {
		return ""
	}
	return deps.LLMModelForProvider(provider)
}

func llmEndpointFromViper() string {
	return llmEndpointForProvider(llmProviderFromViper())
}

func llmAPIKeyFromViper() string {
	return llmAPIKeyForProvider(llmProviderFromViper())
}

func llmModelFromViper() string {
	return llmModelForProvider(llmProviderFromViper())
}

func llmClientFromConfig(cfg llmconfig.ClientConfig) (llm.Client, error) {
	if deps.CreateLLMClient == nil {
		return nil, fmt.Errorf("CreateLLMClient dependency missing")
	}
	return deps.CreateLLMClient(cfg.Provider, cfg.Endpoint, cfg.APIKey, cfg.Model, cfg.RequestTimeout)
}

func registryFromViper() *tools.Registry {
	if deps.RegistryFromViper == nil {
		return nil
	}
	return deps.RegistryFromViper()
}

func registerPlanTool(reg *tools.Registry, client llm.Client, model string) {
	if deps.RegisterPlanTool == nil {
		return
	}
	deps.RegisterPlanTool(reg, client, model)
}

func guardFromViper(log *slog.Logger) *guard.Guard {
	if deps.GuardFromViper == nil {
		return nil
	}
	return deps.GuardFromViper(log)
}

func promptSpecForTelegram(ctx context.Context, logger *slog.Logger, logOpts agent.LogOptions, task string, client llm.Client, model string, stickySkills []string) (agent.PromptSpec, []string, []string, error) {
	if deps.PromptSpecForTelegram == nil {
		return agent.PromptSpec{}, nil, nil, fmt.Errorf("PromptSpecForTelegram dependency missing")
	}
	return deps.PromptSpecForTelegram(ctx, logger, logOpts, task, client, model, stickySkills)
}

func formatFinalOutput(final *agent.Final) string {
	if deps.FormatFinalOutput == nil {
		return ""
	}
	return deps.FormatFinalOutput(final)
}

func buildHeartbeatTask(checklistPath string) (string, bool, error) {
	if deps.BuildHeartbeatTask == nil {
		return "", true, fmt.Errorf("BuildHeartbeatTask dependency missing")
	}
	return deps.BuildHeartbeatTask(checklistPath)
}

func buildHeartbeatMeta(source string, interval time.Duration, checklistPath string, checklistEmpty bool, extra map[string]any) map[string]any {
	if deps.BuildHeartbeatMeta == nil {
		return map[string]any{
			"trigger":   "heartbeat",
			"heartbeat": map[string]any{"source": source, "interval": interval.String()},
		}
	}
	return deps.BuildHeartbeatMeta(source, interval, checklistPath, checklistEmpty, extra)
}
