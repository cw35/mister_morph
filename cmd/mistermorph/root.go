package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/quailyquaily/mistermorph/agent"
	"github.com/quailyquaily/mistermorph/cmd/mistermorph/contactscmd"
	"github.com/quailyquaily/mistermorph/cmd/mistermorph/daemoncmd"
	"github.com/quailyquaily/mistermorph/cmd/mistermorph/maepcmd"
	"github.com/quailyquaily/mistermorph/cmd/mistermorph/runcmd"
	"github.com/quailyquaily/mistermorph/cmd/mistermorph/skillscmd"
	"github.com/quailyquaily/mistermorph/cmd/mistermorph/slackcmd"
	"github.com/quailyquaily/mistermorph/cmd/mistermorph/telegramcmd"
	"github.com/quailyquaily/mistermorph/internal/heartbeatutil"
	"github.com/quailyquaily/mistermorph/internal/llmconfig"
	"github.com/quailyquaily/mistermorph/internal/llmutil"
	"github.com/quailyquaily/mistermorph/internal/logutil"
	"github.com/quailyquaily/mistermorph/internal/pathutil"
	"github.com/quailyquaily/mistermorph/internal/skillsutil"
	"github.com/quailyquaily/mistermorph/internal/toolsutil"
	"github.com/quailyquaily/mistermorph/llm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	envPrefix = "MISTER_MORPH"
)

func Execute() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mistermorph",
		Short: "Unified Agent CLI",
	}

	cobra.OnInitialize(initConfig)

	cmd.PersistentFlags().String("config", "", "Config file path (optional).")
	_ = viper.BindPFlag("config", cmd.PersistentFlags().Lookup("config"))

	// Global logging flags (usable across subcommands like run/serve/telegram).
	cmd.PersistentFlags().String("log-level", "", "Logging level: debug|info|warn|error (defaults to info).")
	cmd.PersistentFlags().String("log-format", "text", "Logging format: text|json.")
	cmd.PersistentFlags().Bool("log-add-source", false, "Include source file:line in logs.")
	cmd.PersistentFlags().Bool("log-include-thoughts", true, "Include model thoughts in logs (may contain sensitive info).")
	cmd.PersistentFlags().Bool("log-include-tool-params", true, "Include tool params in logs (redacted).")
	cmd.PersistentFlags().Bool("log-include-skill-contents", false, "Include loaded SKILL.md contents in logs (truncated).")
	cmd.PersistentFlags().Int("log-max-thought-chars", 2000, "Max characters of thought to log.")
	cmd.PersistentFlags().Int("log-max-json-bytes", 32768, "Max bytes of JSON params to log.")
	cmd.PersistentFlags().Int("log-max-string-value-chars", 2000, "Max characters per string value in logged params.")
	cmd.PersistentFlags().Int("log-max-skill-content-chars", 8000, "Max characters of SKILL.md content to log.")
	cmd.PersistentFlags().StringArray("log-redact-key", nil, "Extra param keys to redact in logs (repeatable).")
	cmd.PersistentFlags().String("health-listen", "0.0.0.0:8787", "Health listen address (empty disables health; serve uses this as on/off only).")

	_ = viper.BindPFlag("logging.level", cmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("logging.format", cmd.PersistentFlags().Lookup("log-format"))
	_ = viper.BindPFlag("logging.add_source", cmd.PersistentFlags().Lookup("log-add-source"))
	_ = viper.BindPFlag("logging.include_thoughts", cmd.PersistentFlags().Lookup("log-include-thoughts"))
	_ = viper.BindPFlag("logging.include_tool_params", cmd.PersistentFlags().Lookup("log-include-tool-params"))
	_ = viper.BindPFlag("logging.include_skill_contents", cmd.PersistentFlags().Lookup("log-include-skill-contents"))
	_ = viper.BindPFlag("logging.max_thought_chars", cmd.PersistentFlags().Lookup("log-max-thought-chars"))
	_ = viper.BindPFlag("logging.max_json_bytes", cmd.PersistentFlags().Lookup("log-max-json-bytes"))
	_ = viper.BindPFlag("logging.max_string_value_chars", cmd.PersistentFlags().Lookup("log-max-string-value-chars"))
	_ = viper.BindPFlag("logging.max_skill_content_chars", cmd.PersistentFlags().Lookup("log-max-skill-content-chars"))
	_ = viper.BindPFlag("logging.redact_keys", cmd.PersistentFlags().Lookup("log-redact-key"))
	_ = viper.BindPFlag("health.listen", cmd.PersistentFlags().Lookup("health-listen"))

	viper.SetDefault("logging.format", "text")
	viper.SetDefault("logging.add_source", false)
	viper.SetDefault("logging.include_thoughts", true)
	viper.SetDefault("logging.include_tool_params", true)
	viper.SetDefault("logging.include_skill_contents", false)
	viper.SetDefault("logging.max_thought_chars", 2000)
	viper.SetDefault("logging.max_json_bytes", 32*1024)
	viper.SetDefault("logging.max_string_value_chars", 2000)
	viper.SetDefault("logging.max_skill_content_chars", 8000)

	cmd.AddCommand(runcmd.New(runcmd.Dependencies{
		RegistryFromViper: registryFromViper,
		GuardFromViper:    guardFromViper,
		RegisterPlanTool:  toolsutil.RegisterPlanTool,
	}))
	cmd.AddCommand(daemoncmd.NewServeCmd(daemoncmd.ServeDependencies{
		RegistryFromViper: registryFromViper,
		GuardFromViper:    guardFromViper,
	}))
	cmd.AddCommand(daemoncmd.NewSubmitCmd())
	cmd.AddCommand(telegramcmd.NewCommand(telegramcmd.Dependencies{
		LoggerFromViper:     logutil.LoggerFromViper,
		LogOptionsFromViper: logutil.LogOptionsFromViper,
		CreateLLMClient: func(provider, endpoint, apiKey, model string, timeout time.Duration) (llm.Client, error) {
			return llmutil.ClientFromConfig(llmconfig.ClientConfig{
				Provider:       provider,
				Endpoint:       endpoint,
				APIKey:         apiKey,
				Model:          model,
				RequestTimeout: timeout,
			})
		},
		LLMProviderFromViper:   llmutil.ProviderFromViper,
		LLMEndpointForProvider: llmutil.EndpointForProvider,
		LLMAPIKeyForProvider:   llmutil.APIKeyForProvider,
		LLMModelForProvider:    llmutil.ModelForProvider,
		RegistryFromViper:      registryFromViper,
		RegisterPlanTool:       toolsutil.RegisterPlanTool,
		GuardFromViper:         guardFromViper,
		PromptSpecForTelegram: func(ctx context.Context, logger *slog.Logger, logOpts agent.LogOptions, task string, client llm.Client, model string, stickySkills []string) (agent.PromptSpec, []string, []string, error) {
			cfg := skillsutil.SkillsConfigFromViper()
			if len(stickySkills) > 0 {
				cfg.Requested = append(cfg.Requested, stickySkills...)
			}
			return skillsutil.PromptSpecWithSkills(ctx, logger, logOpts, task, client, model, cfg)
		},
		BuildHeartbeatTask: heartbeatutil.BuildHeartbeatTask,
		BuildHeartbeatMeta: func(source string, interval time.Duration, checklistPath string, checklistEmpty bool, extra map[string]any) map[string]any {
			return heartbeatutil.BuildHeartbeatMeta(source, interval, checklistPath, checklistEmpty, nil, extra)
		},
	}))
	cmd.AddCommand(slackcmd.NewCommand(slackcmd.Dependencies{
		LoggerFromViper:     logutil.LoggerFromViper,
		LogOptionsFromViper: logutil.LogOptionsFromViper,
		CreateLLMClient: func(provider, endpoint, apiKey, model string, timeout time.Duration) (llm.Client, error) {
			return llmutil.ClientFromConfig(llmconfig.ClientConfig{
				Provider:       provider,
				Endpoint:       endpoint,
				APIKey:         apiKey,
				Model:          model,
				RequestTimeout: timeout,
			})
		},
		LLMProviderFromViper:   llmutil.ProviderFromViper,
		LLMEndpointForProvider: llmutil.EndpointForProvider,
		LLMAPIKeyForProvider:   llmutil.APIKeyForProvider,
		LLMModelForProvider:    llmutil.ModelForProvider,
		RegistryFromViper:      registryFromViper,
		RegisterPlanTool:       toolsutil.RegisterPlanTool,
		GuardFromViper:         guardFromViper,
		PromptSpecForSlack: func(ctx context.Context, logger *slog.Logger, logOpts agent.LogOptions, task string, client llm.Client, model string, stickySkills []string) (agent.PromptSpec, []string, []string, error) {
			cfg := skillsutil.SkillsConfigFromViper()
			if len(stickySkills) > 0 {
				cfg.Requested = append(cfg.Requested, stickySkills...)
			}
			return skillsutil.PromptSpecWithSkills(ctx, logger, logOpts, task, client, model, cfg)
		},
	}))
	cmd.AddCommand(newToolsCmd())
	cmd.AddCommand(skillscmd.New())
	cmd.AddCommand(maepcmd.New())
	cmd.AddCommand(contactscmd.New())
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func initConfig() {
	initViperDefaults()

	viper.SetEnvPrefix(envPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	cfgFile := strings.TrimSpace(viper.GetString("config"))
	if cfgFile == "" {
		return
	}
	cfgFile = pathutil.ExpandHomePath(cfgFile)

	viper.SetConfigFile(cfgFile)
	if err := viper.ReadInConfig(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to read config: %v\n", err)
		return
	}

	expandConfiguredDirKey("file_state_dir")
	expandConfiguredDirKey("file_cache_dir")
}

func expandConfiguredDirKey(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	raw := strings.TrimSpace(viper.GetString(key))
	if raw == "" {
		return
	}
	viper.Set(key, pathutil.ExpandHomePath(raw))
}
