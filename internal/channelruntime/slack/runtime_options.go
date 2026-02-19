package slack

import (
	"strings"
	"time"

	"github.com/quailyquaily/mistermorph/internal/healthcheck"
)

type runtimeLoopOptions struct {
	BotToken                      string
	AppToken                      string
	AllowedTeamIDs                []string
	AllowedChannelIDs             []string
	GroupTriggerMode              string
	AddressingConfidenceThreshold float64
	AddressingInterjectThreshold  float64
	TaskTimeout                   time.Duration
	MaxConcurrency                int
	HealthListen                  string
	ServerAuthToken               string
	ServerMaxQueue                int
	Hooks                         Hooks
	BaseURL                       string
	BusMaxInFlight                int
	RequestTimeout                time.Duration
	AgentMaxSteps                 int
	AgentParseRetries             int
	AgentMaxTokenBudget           int
	SecretsRequireSkillProfiles   bool
	InspectPrompt                 bool
	InspectRequest                bool
}

func resolveRuntimeLoopOptionsFromRunOptions(opts RunOptions) runtimeLoopOptions {
	out := runtimeLoopOptions{
		BotToken:                      strings.TrimSpace(opts.BotToken),
		AppToken:                      strings.TrimSpace(opts.AppToken),
		AllowedTeamIDs:                normalizeRunStringSlice(opts.AllowedTeamIDs),
		AllowedChannelIDs:             normalizeRunStringSlice(opts.AllowedChannelIDs),
		GroupTriggerMode:              strings.TrimSpace(opts.GroupTriggerMode),
		AddressingConfidenceThreshold: opts.AddressingConfidenceThreshold,
		AddressingInterjectThreshold:  opts.AddressingInterjectThreshold,
		TaskTimeout:                   opts.TaskTimeout,
		MaxConcurrency:                opts.MaxConcurrency,
		HealthListen:                  strings.TrimSpace(opts.HealthListen),
		ServerAuthToken:               strings.TrimSpace(opts.ServerAuthToken),
		ServerMaxQueue:                opts.ServerMaxQueue,
		BaseURL:                       strings.TrimSpace(opts.BaseURL),
		Hooks:                         opts.Hooks,
		BusMaxInFlight:                opts.BusMaxInFlight,
		RequestTimeout:                opts.RequestTimeout,
		AgentMaxSteps:                 opts.AgentMaxSteps,
		AgentParseRetries:             opts.AgentParseRetries,
		AgentMaxTokenBudget:           opts.AgentMaxTokenBudget,
		SecretsRequireSkillProfiles:   opts.SecretsRequireSkillProfiles,
		InspectPrompt:                 opts.InspectPrompt,
		InspectRequest:                opts.InspectRequest,
	}
	return normalizeRuntimeLoopOptions(out)
}

func normalizeRuntimeLoopOptions(opts runtimeLoopOptions) runtimeLoopOptions {
	opts.BotToken = strings.TrimSpace(opts.BotToken)
	opts.AppToken = strings.TrimSpace(opts.AppToken)
	opts.AllowedTeamIDs = normalizeRunStringSlice(opts.AllowedTeamIDs)
	opts.AllowedChannelIDs = normalizeRunStringSlice(opts.AllowedChannelIDs)
	opts.GroupTriggerMode = strings.ToLower(strings.TrimSpace(opts.GroupTriggerMode))
	opts.HealthListen = healthcheck.NormalizeListen(strings.TrimSpace(opts.HealthListen))
	opts.ServerAuthToken = strings.TrimSpace(opts.ServerAuthToken)
	opts.BaseURL = strings.TrimSpace(opts.BaseURL)

	if opts.TaskTimeout <= 0 {
		opts.TaskTimeout = 10 * time.Minute
	}
	if opts.MaxConcurrency <= 0 {
		opts.MaxConcurrency = 3
	}
	if opts.BusMaxInFlight <= 0 {
		opts.BusMaxInFlight = 1024
	}
	if opts.ServerMaxQueue <= 0 {
		opts.ServerMaxQueue = 100
	}
	if opts.RequestTimeout <= 0 {
		opts.RequestTimeout = 90 * time.Second
	}
	if opts.AgentMaxSteps <= 0 {
		opts.AgentMaxSteps = 15
	}
	if opts.AgentParseRetries <= 0 {
		opts.AgentParseRetries = 2
	}
	if opts.GroupTriggerMode == "" {
		opts.GroupTriggerMode = "smart"
	}
	if opts.BaseURL == "" {
		opts.BaseURL = "https://slack.com/api"
	}
	opts.AddressingConfidenceThreshold = normalizeThreshold(opts.AddressingConfidenceThreshold, 0.6, 0.6)
	opts.AddressingInterjectThreshold = normalizeThreshold(opts.AddressingInterjectThreshold, 0.6, 0.6)
	return opts
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
