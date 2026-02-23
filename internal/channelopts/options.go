package channelopts

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	slackruntime "github.com/quailyquaily/mistermorph/internal/channelruntime/slack"
	telegramruntime "github.com/quailyquaily/mistermorph/internal/channelruntime/telegram"
	"github.com/spf13/viper"
)

type ConfigReader interface {
	GetStringSlice(string) []string
	GetString(string) string
	GetFloat64(string) float64
	GetDuration(string) time.Duration
	GetInt(string) int
	GetInt64(string) int64
	GetBool(string) bool
}

type TelegramConfig struct {
	AllowedChatIDsRaw                    []string
	DefaultGroupTriggerMode              string
	DefaultAddressingConfidenceThreshold float64
	DefaultAddressingInterjectThreshold  float64
	PollTimeout                          time.Duration
	TaskTimeout                          time.Duration
	GlobalTaskTimeout                    time.Duration
	MaxConcurrency                       int
	FileCacheDir                         string
	ServerListen                         string
	ServerAuthToken                      string
	ServerMaxQueue                       int
	BusMaxInFlight                       int
	RequestTimeout                       time.Duration
	AgentMaxSteps                        int
	AgentParseRetries                    int
	AgentMaxTokenBudget                  int
	FileCacheMaxAge                      time.Duration
	FileCacheMaxFiles                    int
	FileCacheMaxTotalBytes               int64
	HeartbeatEnabled                     bool
	HeartbeatInterval                    time.Duration
	MemoryEnabled                        bool
	MemoryShortTermDays                  int
	MemoryInjectionEnabled               bool
	MemoryInjectionMaxItems              int
	SecretsRequireSkillProfiles          bool
}

type TelegramInput struct {
	BotToken                      string
	AllowedChatIDs                []int64
	GroupTriggerMode              string
	AddressingConfidenceThreshold float64
	AddressingInterjectThreshold  float64
	PollTimeout                   time.Duration
	TaskTimeout                   time.Duration
	MaxConcurrency                int
	FileCacheDir                  string
	Hooks                         telegramruntime.Hooks
	InspectPrompt                 bool
	InspectRequest                bool
}

func TelegramConfigFromReader(r ConfigReader) TelegramConfig {
	if r == nil {
		return TelegramConfig{}
	}
	return TelegramConfig{
		AllowedChatIDsRaw:                    append([]string(nil), r.GetStringSlice("telegram.allowed_chat_ids")...),
		DefaultGroupTriggerMode:              strings.TrimSpace(r.GetString("telegram.group_trigger_mode")),
		DefaultAddressingConfidenceThreshold: r.GetFloat64("telegram.addressing_confidence_threshold"),
		DefaultAddressingInterjectThreshold:  r.GetFloat64("telegram.addressing_interject_threshold"),
		PollTimeout:                          r.GetDuration("telegram.poll_timeout"),
		TaskTimeout:                          r.GetDuration("telegram.task_timeout"),
		GlobalTaskTimeout:                    r.GetDuration("timeout"),
		MaxConcurrency:                       r.GetInt("telegram.max_concurrency"),
		FileCacheDir:                         strings.TrimSpace(r.GetString("file_cache_dir")),
		ServerListen:                         strings.TrimSpace(r.GetString("server.listen")),
		ServerAuthToken:                      strings.TrimSpace(r.GetString("server.auth_token")),
		ServerMaxQueue:                       r.GetInt("server.max_queue"),
		BusMaxInFlight:                       r.GetInt("bus.max_inflight"),
		RequestTimeout:                       r.GetDuration("llm.request_timeout"),
		AgentMaxSteps:                        r.GetInt("max_steps"),
		AgentParseRetries:                    r.GetInt("parse_retries"),
		AgentMaxTokenBudget:                  r.GetInt("max_token_budget"),
		FileCacheMaxAge:                      r.GetDuration("file_cache.max_age"),
		FileCacheMaxFiles:                    r.GetInt("file_cache.max_files"),
		FileCacheMaxTotalBytes:               r.GetInt64("file_cache.max_total_bytes"),
		HeartbeatEnabled:                     r.GetBool("heartbeat.enabled"),
		HeartbeatInterval:                    r.GetDuration("heartbeat.interval"),
		MemoryEnabled:                        r.GetBool("memory.enabled"),
		MemoryShortTermDays:                  r.GetInt("memory.short_term_days"),
		MemoryInjectionEnabled:               r.GetBool("memory.injection.enabled"),
		MemoryInjectionMaxItems:              r.GetInt("memory.injection.max_items"),
		SecretsRequireSkillProfiles:          r.GetBool("secrets.require_skill_profiles"),
	}
}

func TelegramConfigFromViper() TelegramConfig {
	return TelegramConfigFromReader(viper.GetViper())
}

func BuildTelegramRunOptions(cfg TelegramConfig, in TelegramInput) (telegramruntime.RunOptions, error) {
	allowedChatIDs := append([]int64(nil), in.AllowedChatIDs...)
	if len(allowedChatIDs) == 0 {
		ids, err := ParseTelegramAllowedChatIDs(cfg.AllowedChatIDsRaw)
		if err != nil {
			return telegramruntime.RunOptions{}, err
		}
		allowedChatIDs = ids
	}

	groupTriggerMode := strings.TrimSpace(in.GroupTriggerMode)
	if groupTriggerMode == "" {
		groupTriggerMode = strings.TrimSpace(cfg.DefaultGroupTriggerMode)
	}
	addressingConfidenceThreshold := in.AddressingConfidenceThreshold
	if addressingConfidenceThreshold <= 0 {
		addressingConfidenceThreshold = cfg.DefaultAddressingConfidenceThreshold
	}
	addressingInterjectThreshold := in.AddressingInterjectThreshold
	if addressingInterjectThreshold <= 0 {
		addressingInterjectThreshold = cfg.DefaultAddressingInterjectThreshold
	}
	pollTimeout := in.PollTimeout
	if pollTimeout <= 0 {
		pollTimeout = cfg.PollTimeout
	}
	taskTimeout := in.TaskTimeout
	if taskTimeout <= 0 {
		taskTimeout = cfg.TaskTimeout
	}
	if taskTimeout <= 0 {
		taskTimeout = cfg.GlobalTaskTimeout
	}
	maxConcurrency := in.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = cfg.MaxConcurrency
	}
	fileCacheDir := strings.TrimSpace(in.FileCacheDir)
	if fileCacheDir == "" {
		fileCacheDir = strings.TrimSpace(cfg.FileCacheDir)
	}
	serverListen := normalizeServerListen(cfg.ServerListen)

	return telegramruntime.RunOptions{
		BotToken:                      strings.TrimSpace(in.BotToken),
		AllowedChatIDs:                allowedChatIDs,
		GroupTriggerMode:              groupTriggerMode,
		AddressingConfidenceThreshold: addressingConfidenceThreshold,
		AddressingInterjectThreshold:  addressingInterjectThreshold,
		PollTimeout:                   pollTimeout,
		TaskTimeout:                   taskTimeout,
		MaxConcurrency:                maxConcurrency,
		FileCacheDir:                  fileCacheDir,
		ServerListen:                  serverListen,
		ServerAuthToken:               cfg.ServerAuthToken,
		ServerMaxQueue:                cfg.ServerMaxQueue,
		BusMaxInFlight:                cfg.BusMaxInFlight,
		RequestTimeout:                cfg.RequestTimeout,
		AgentMaxSteps:                 cfg.AgentMaxSteps,
		AgentParseRetries:             cfg.AgentParseRetries,
		AgentMaxTokenBudget:           cfg.AgentMaxTokenBudget,
		FileCacheMaxAge:               cfg.FileCacheMaxAge,
		FileCacheMaxFiles:             cfg.FileCacheMaxFiles,
		FileCacheMaxTotalBytes:        cfg.FileCacheMaxTotalBytes,
		HeartbeatEnabled:              cfg.HeartbeatEnabled,
		HeartbeatInterval:             cfg.HeartbeatInterval,
		MemoryEnabled:                 cfg.MemoryEnabled,
		MemoryShortTermDays:           cfg.MemoryShortTermDays,
		MemoryInjectionEnabled:        cfg.MemoryInjectionEnabled,
		MemoryInjectionMaxItems:       cfg.MemoryInjectionMaxItems,
		SecretsRequireSkillProfiles:   cfg.SecretsRequireSkillProfiles,
		Hooks:                         in.Hooks,
		InspectPrompt:                 in.InspectPrompt,
		InspectRequest:                in.InspectRequest,
	}, nil
}

func ParseTelegramAllowedChatIDs(values []string) ([]int64, error) {
	if len(values) == 0 {
		return []int64{}, nil
	}
	out := make([]int64, 0, len(values))
	seen := map[int64]struct{}{}
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid telegram.allowed_chat_ids entry %q: %w", raw, err)
		}
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
		return []int64{}, nil
	}
	return out, nil
}

type SlackConfig struct {
	AllowedTeamIDs                       []string
	AllowedChannelIDs                    []string
	DefaultGroupTriggerMode              string
	DefaultAddressingConfidenceThreshold float64
	DefaultAddressingInterjectThreshold  float64
	TaskTimeout                          time.Duration
	GlobalTaskTimeout                    time.Duration
	MaxConcurrency                       int
	ServerListen                         string
	ServerAuthToken                      string
	ServerMaxQueue                       int
	BaseURL                              string
	BusMaxInFlight                       int
	RequestTimeout                       time.Duration
	AgentMaxSteps                        int
	AgentParseRetries                    int
	AgentMaxTokenBudget                  int
	SecretsRequireSkillProfiles          bool
}

type SlackInput struct {
	BotToken                      string
	AppToken                      string
	AllowedTeamIDs                []string
	AllowedChannelIDs             []string
	GroupTriggerMode              string
	AddressingConfidenceThreshold float64
	AddressingInterjectThreshold  float64
	TaskTimeout                   time.Duration
	MaxConcurrency                int
	BaseURL                       string
	Hooks                         slackruntime.Hooks
	InspectPrompt                 bool
	InspectRequest                bool
}

func SlackConfigFromReader(r ConfigReader) SlackConfig {
	if r == nil {
		return SlackConfig{}
	}
	return SlackConfig{
		AllowedTeamIDs:                       append([]string(nil), r.GetStringSlice("slack.allowed_team_ids")...),
		AllowedChannelIDs:                    append([]string(nil), r.GetStringSlice("slack.allowed_channel_ids")...),
		DefaultGroupTriggerMode:              strings.TrimSpace(r.GetString("slack.group_trigger_mode")),
		DefaultAddressingConfidenceThreshold: r.GetFloat64("slack.addressing_confidence_threshold"),
		DefaultAddressingInterjectThreshold:  r.GetFloat64("slack.addressing_interject_threshold"),
		TaskTimeout:                          r.GetDuration("slack.task_timeout"),
		GlobalTaskTimeout:                    r.GetDuration("timeout"),
		MaxConcurrency:                       r.GetInt("slack.max_concurrency"),
		ServerListen:                         strings.TrimSpace(r.GetString("server.listen")),
		ServerAuthToken:                      strings.TrimSpace(r.GetString("server.auth_token")),
		ServerMaxQueue:                       r.GetInt("server.max_queue"),
		BaseURL:                              strings.TrimSpace(r.GetString("slack.base_url")),
		BusMaxInFlight:                       r.GetInt("bus.max_inflight"),
		RequestTimeout:                       r.GetDuration("llm.request_timeout"),
		AgentMaxSteps:                        r.GetInt("max_steps"),
		AgentParseRetries:                    r.GetInt("parse_retries"),
		AgentMaxTokenBudget:                  r.GetInt("max_token_budget"),
		SecretsRequireSkillProfiles:          r.GetBool("secrets.require_skill_profiles"),
	}
}

func SlackConfigFromViper() SlackConfig {
	return SlackConfigFromReader(viper.GetViper())
}

func BuildSlackRunOptions(cfg SlackConfig, in SlackInput) slackruntime.RunOptions {
	allowedTeamIDs := append([]string(nil), in.AllowedTeamIDs...)
	if len(allowedTeamIDs) == 0 {
		allowedTeamIDs = append([]string(nil), cfg.AllowedTeamIDs...)
	}
	allowedChannelIDs := append([]string(nil), in.AllowedChannelIDs...)
	if len(allowedChannelIDs) == 0 {
		allowedChannelIDs = append([]string(nil), cfg.AllowedChannelIDs...)
	}

	groupTriggerMode := strings.TrimSpace(in.GroupTriggerMode)
	if groupTriggerMode == "" {
		groupTriggerMode = strings.TrimSpace(cfg.DefaultGroupTriggerMode)
	}
	addressingConfidenceThreshold := in.AddressingConfidenceThreshold
	if addressingConfidenceThreshold <= 0 {
		addressingConfidenceThreshold = cfg.DefaultAddressingConfidenceThreshold
	}
	addressingInterjectThreshold := in.AddressingInterjectThreshold
	if addressingInterjectThreshold <= 0 {
		addressingInterjectThreshold = cfg.DefaultAddressingInterjectThreshold
	}
	taskTimeout := in.TaskTimeout
	if taskTimeout <= 0 {
		taskTimeout = cfg.TaskTimeout
	}
	if taskTimeout <= 0 {
		taskTimeout = cfg.GlobalTaskTimeout
	}
	maxConcurrency := in.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = cfg.MaxConcurrency
	}
	serverListen := normalizeServerListen(cfg.ServerListen)
	baseURL := strings.TrimSpace(in.BaseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(cfg.BaseURL)
	}

	return slackruntime.RunOptions{
		BotToken:                      strings.TrimSpace(in.BotToken),
		AppToken:                      strings.TrimSpace(in.AppToken),
		AllowedTeamIDs:                allowedTeamIDs,
		AllowedChannelIDs:             allowedChannelIDs,
		GroupTriggerMode:              groupTriggerMode,
		AddressingConfidenceThreshold: addressingConfidenceThreshold,
		AddressingInterjectThreshold:  addressingInterjectThreshold,
		TaskTimeout:                   taskTimeout,
		MaxConcurrency:                maxConcurrency,
		ServerListen:                  serverListen,
		ServerAuthToken:               cfg.ServerAuthToken,
		ServerMaxQueue:                cfg.ServerMaxQueue,
		BaseURL:                       baseURL,
		BusMaxInFlight:                cfg.BusMaxInFlight,
		RequestTimeout:                cfg.RequestTimeout,
		AgentMaxSteps:                 cfg.AgentMaxSteps,
		AgentParseRetries:             cfg.AgentParseRetries,
		AgentMaxTokenBudget:           cfg.AgentMaxTokenBudget,
		SecretsRequireSkillProfiles:   cfg.SecretsRequireSkillProfiles,
		Hooks:                         in.Hooks,
		InspectPrompt:                 in.InspectPrompt,
		InspectRequest:                in.InspectRequest,
	}
}

func normalizeServerListen(listen string) string {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return "127.0.0.1:8787"
	}
	return listen
}
