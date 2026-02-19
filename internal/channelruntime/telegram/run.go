package telegram

import (
	"context"
	"strings"
	"time"
)

type RunOptions struct {
	BotToken                      string
	AllowedChatIDs                []int64
	GroupTriggerMode              string
	AddressingConfidenceThreshold float64
	AddressingInterjectThreshold  float64
	WithMAEP                      bool
	MAEPListenAddrs               []string
	PollTimeout                   time.Duration
	TaskTimeout                   time.Duration
	MaxConcurrency                int
	FileCacheDir                  string
	HealthListen                  string
	ServerAuthToken               string
	ServerMaxQueue                int
	BusMaxInFlight                int
	RequestTimeout                time.Duration
	AgentMaxSteps                 int
	AgentParseRetries             int
	AgentMaxTokenBudget           int
	FileCacheMaxAge               time.Duration
	FileCacheMaxFiles             int
	FileCacheMaxTotalBytes        int64
	HeartbeatEnabled              bool
	HeartbeatInterval             time.Duration
	MemoryEnabled                 bool
	MemoryShortTermDays           int
	MemoryInjectionEnabled        bool
	MemoryInjectionMaxItems       int
	SecretsRequireSkillProfiles   bool
	MAEPMaxTurnsPerSession        int
	MAEPSessionCooldown           time.Duration
	Hooks                         Hooks
	InspectPrompt                 bool
	InspectRequest                bool
}

func Run(ctx context.Context, d Dependencies, opts RunOptions) error {
	return runTelegramLoop(ctx, d, resolveRuntimeLoopOptionsFromRunOptions(opts))
}

func normalizeAllowedChatIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return []int64{}
	}
	out := make([]int64, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return []int64{}
	}
	return out
}

func normalizeRunStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}
