package slackcmd

import (
	"fmt"
	"strings"

	"github.com/quailyquaily/mistermorph/internal/channelopts"
	slackruntime "github.com/quailyquaily/mistermorph/internal/channelruntime/slack"
	"github.com/quailyquaily/mistermorph/internal/configutil"
	"github.com/spf13/cobra"
)

func NewCommand(d Dependencies) *cobra.Command {
	return newSlackCmd(d)
}

func newSlackCmd(d Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slack",
		Short: "Run a Slack bot with Socket Mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			botToken := strings.TrimSpace(configutil.FlagOrViperString(cmd, "slack-bot-token", "slack.bot_token"))
			if botToken == "" {
				return fmt.Errorf("missing slack.bot_token (set via --slack-bot-token or MISTER_MORPH_SLACK_BOT_TOKEN)")
			}
			appToken := strings.TrimSpace(configutil.FlagOrViperString(cmd, "slack-app-token", "slack.app_token"))
			if appToken == "" {
				return fmt.Errorf("missing slack.app_token (set via --slack-app-token or MISTER_MORPH_SLACK_APP_TOKEN)")
			}

			runOpts := channelopts.BuildSlackRunOptions(channelopts.SlackConfigFromViper(), channelopts.SlackInput{
				BotToken:                      botToken,
				AppToken:                      appToken,
				AllowedTeamIDs:                configutil.FlagOrViperStringArray(cmd, "slack-allowed-team-id", "slack.allowed_team_ids"),
				AllowedChannelIDs:             configutil.FlagOrViperStringArray(cmd, "slack-allowed-channel-id", "slack.allowed_channel_ids"),
				GroupTriggerMode:              strings.TrimSpace(configutil.FlagOrViperString(cmd, "slack-group-trigger-mode", "slack.group_trigger_mode")),
				AddressingConfidenceThreshold: configutil.FlagOrViperFloat64(cmd, "slack-addressing-confidence-threshold", "slack.addressing_confidence_threshold"),
				AddressingInterjectThreshold:  configutil.FlagOrViperFloat64(cmd, "slack-addressing-interject-threshold", "slack.addressing_interject_threshold"),
				TaskTimeout:                   configutil.FlagOrViperDuration(cmd, "slack-task-timeout", "slack.task_timeout"),
				MaxConcurrency:                configutil.FlagOrViperInt(cmd, "slack-max-concurrency", "slack.max_concurrency"),
				InspectPrompt:                 configutil.FlagOrViperBool(cmd, "inspect-prompt", ""),
				InspectRequest:                configutil.FlagOrViperBool(cmd, "inspect-request", ""),
			})
			return slackruntime.Run(cmd.Context(), slackruntime.Dependencies(d), runOpts)
		},
	}

	cmd.Flags().String("slack-bot-token", "", "Slack bot token (xoxb-...).")
	cmd.Flags().String("slack-app-token", "", "Slack app-level token for Socket Mode (xapp-...).")
	cmd.Flags().StringArray("slack-allowed-team-id", nil, "Allowed Slack team id(s). If empty, defaults to the bot's home team.")
	cmd.Flags().StringArray("slack-allowed-channel-id", nil, "Allowed Slack channel id(s). If empty, allows all channels in allowed teams.")
	cmd.Flags().String("slack-group-trigger-mode", "smart", "Group trigger mode: strict|smart|talkative.")
	cmd.Flags().Float64("slack-addressing-confidence-threshold", 0.6, "Minimum confidence (0-1) required to accept an addressing LLM decision.")
	cmd.Flags().Float64("slack-addressing-interject-threshold", 0.6, "Minimum interject (0-1) required to accept an addressing LLM decision.")
	cmd.Flags().Duration("slack-task-timeout", 0, "Per-message agent timeout (0 uses --timeout).")
	cmd.Flags().Int("slack-max-concurrency", 3, "Max number of Slack conversations processed concurrently.")
	cmd.Flags().Bool("inspect-prompt", false, "Dump prompts (messages) to ./dump/prompt_slack_YYYYMMDD_HHmmss.md.")
	cmd.Flags().Bool("inspect-request", false, "Dump LLM request/response payloads to ./dump/request_slack_YYYYMMDD_HHmmss.md.")

	return cmd
}
