package slackcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/quailyquaily/mistermorph/agent"
	"github.com/quailyquaily/mistermorph/contacts"
	"github.com/quailyquaily/mistermorph/guard"
	busruntime "github.com/quailyquaily/mistermorph/internal/bus"
	slackbus "github.com/quailyquaily/mistermorph/internal/bus/adapters/slack"
	"github.com/quailyquaily/mistermorph/internal/chathistory"
	"github.com/quailyquaily/mistermorph/internal/configutil"
	"github.com/quailyquaily/mistermorph/internal/healthcheck"
	"github.com/quailyquaily/mistermorph/internal/idempotency"
	"github.com/quailyquaily/mistermorph/internal/jsonutil"
	"github.com/quailyquaily/mistermorph/internal/llmconfig"
	"github.com/quailyquaily/mistermorph/internal/llminspect"
	"github.com/quailyquaily/mistermorph/internal/promptprofile"
	"github.com/quailyquaily/mistermorph/internal/statepaths"
	"github.com/quailyquaily/mistermorph/internal/todo"
	"github.com/quailyquaily/mistermorph/internal/toolsutil"
	"github.com/quailyquaily/mistermorph/llm"
	"github.com/quailyquaily/mistermorph/tools"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type slackJob struct {
	ConversationKey string
	TeamID          string
	ChannelID       string
	ChatType        string
	MessageTS       string
	ThreadTS        string
	UserID          string
	Text            string
	SentAt          time.Time
	Version         uint64
	MentionUsers    []string
}

type slackConversationWorker struct {
	Jobs    chan slackJob
	Version uint64
}

type slackSocketEnvelope struct {
	EnvelopeID string          `json:"envelope_id,omitempty"`
	Type       string          `json:"type,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type slackEventAuthorization struct {
	TeamID string `json:"team_id,omitempty"`
	UserID string `json:"user_id,omitempty"`
	IsBot  bool   `json:"is_bot,omitempty"`
}

type slackEventsAPIPayload struct {
	TeamID         string                    `json:"team_id,omitempty"`
	EventID        string                    `json:"event_id,omitempty"`
	EventTime      int64                     `json:"event_time,omitempty"`
	Event          json.RawMessage           `json:"event,omitempty"`
	Authorizations []slackEventAuthorization `json:"authorizations,omitempty"`
}

type slackEvent struct {
	Type        string `json:"type,omitempty"`
	Subtype     string `json:"subtype,omitempty"`
	User        string `json:"user,omitempty"`
	Text        string `json:"text,omitempty"`
	Channel     string `json:"channel,omitempty"`
	ChannelType string `json:"channel_type,omitempty"`
	TS          string `json:"ts,omitempty"`
	ThreadTS    string `json:"thread_ts,omitempty"`
	BotID       string `json:"bot_id,omitempty"`
	Team        string `json:"team,omitempty"`
	EventTS     string `json:"event_ts,omitempty"`
}

type slackInboundEvent struct {
	TeamID          string
	ChannelID       string
	ChatType        string
	MessageTS       string
	ThreadTS        string
	UserID          string
	Text            string
	EventID         string
	SentAt          time.Time
	MentionUsers    []string
	IsAppMention    bool
	IsThreadMessage bool
}

type slackGroupTriggerDecision struct {
	Reason            string
	UsedAddressingLLM bool

	AddressingLLMAttempted  bool
	AddressingLLMOK         bool
	AddressingLLMAddressed  bool
	AddressingLLMConfidence float64
	AddressingLLMInterject  float64
	AddressingImpulse       float64
}

type slackAddressingLLMDecision struct {
	Addressed  bool    `json:"addressed"`
	Confidence float64 `json:"confidence"`
	Interject  float64 `json:"interject"`
	Impulse    float64 `json:"impulse"`
	Reason     string  `json:"reason"`
}

const slackStickySkillsCap = 16

var slackMentionPattern = regexp.MustCompile(`<@([A-Z0-9]+)(?:\|[^>]+)?>`)

func newSlackCmd() *cobra.Command {
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

			allowedTeams := toAllowlist(configutil.FlagOrViperStringArray(cmd, "slack-allowed-team-id", "slack.allowed_team_ids"))
			allowedChannels := toAllowlist(configutil.FlagOrViperStringArray(cmd, "slack-allowed-channel-id", "slack.allowed_channel_ids"))

			logger, err := loggerFromViper()
			if err != nil {
				return err
			}
			slog.SetDefault(logger)

			inprocBus, err := busruntime.StartInproc(busruntime.BootstrapOptions{
				MaxInFlight: viper.GetInt("bus.max_inflight"),
				Logger:      logger,
				Component:   "slack",
			})
			if err != nil {
				return err
			}
			defer inprocBus.Close()

			contactsStore := contacts.NewFileStore(statepaths.ContactsDir())
			if err := contactsStore.Ensure(context.Background()); err != nil {
				return err
			}
			slackInboundAdapter, err := slackbus.NewInboundAdapter(slackbus.InboundAdapterOptions{
				Bus:   inprocBus,
				Store: contactsStore,
			})
			if err != nil {
				return err
			}

			httpClient := &http.Client{Timeout: 30 * time.Second}
			api := newSlackAPI(httpClient, "https://slack.com/api", botToken, appToken)
			auth, err := api.authTest(cmd.Context())
			if err != nil {
				return fmt.Errorf("slack auth.test: %w", err)
			}
			botUserID := strings.TrimSpace(auth.UserID)
			if botUserID == "" {
				return fmt.Errorf("slack auth.test returned empty user_id")
			}
			if len(allowedTeams) == 0 && strings.TrimSpace(auth.TeamID) != "" {
				allowedTeams[strings.TrimSpace(auth.TeamID)] = true
			}

			slackDeliveryAdapter, err := slackbus.NewDeliveryAdapter(slackbus.DeliveryAdapterOptions{
				SendText: func(ctx context.Context, target any, text string, opts slackbus.SendTextOptions) error {
					deliverTarget, ok := target.(slackbus.DeliveryTarget)
					if !ok {
						return fmt.Errorf("slack target is invalid")
					}
					return api.postMessage(ctx, deliverTarget.ChannelID, text, opts.ThreadTS)
				},
			})
			if err != nil {
				return err
			}

			requestTimeout := viper.GetDuration("llm.request_timeout")
			client, err := llmClientFromConfig(llmconfig.ClientConfig{
				Provider:       llmProviderFromViper(),
				Endpoint:       llmEndpointFromViper(),
				APIKey:         llmAPIKeyFromViper(),
				Model:          llmModelFromViper(),
				RequestTimeout: requestTimeout,
			})
			if err != nil {
				return err
			}
			if configutil.FlagOrViperBool(cmd, "inspect-request", "") {
				inspector, err := llminspect.NewRequestInspector(llminspect.Options{
					Mode:            "slack",
					Task:            "slack",
					TimestampFormat: "20060102_150405",
				})
				if err != nil {
					return err
				}
				defer func() { _ = inspector.Close() }()
				if err := llminspect.SetDebugHook(client, inspector.Dump); err != nil {
					return fmt.Errorf("inspect-request requires uniai provider client")
				}
			}
			if configutil.FlagOrViperBool(cmd, "inspect-prompt", "") {
				inspector, err := llminspect.NewPromptInspector(llminspect.Options{
					Mode:            "slack",
					Task:            "slack",
					TimestampFormat: "20060102_150405",
				})
				if err != nil {
					return err
				}
				defer func() { _ = inspector.Close() }()
				client = &llminspect.PromptClient{Base: client, Inspector: inspector}
			}

			logOpts := logOptionsFromViper()
			reg := registryFromViper()
			if reg == nil {
				reg = tools.NewRegistry()
			}
			registerPlanTool(reg, client, llmModelFromViper())
			toolsutil.BindTodoUpdateToolLLM(reg, client, llmModelFromViper())

			cfg := agent.Config{
				MaxSteps:       viper.GetInt("max_steps"),
				ParseRetries:   viper.GetInt("parse_retries"),
				MaxTokenBudget: viper.GetInt("max_token_budget"),
			}
			taskTimeout := configutil.FlagOrViperDuration(cmd, "slack-task-timeout", "slack.task_timeout")
			if taskTimeout <= 0 {
				taskTimeout = viper.GetDuration("timeout")
			}
			if taskTimeout <= 0 {
				taskTimeout = 10 * time.Minute
			}
			maxConc := configutil.FlagOrViperInt(cmd, "slack-max-concurrency", "slack.max_concurrency")
			if maxConc <= 0 {
				maxConc = 3
			}
			sem := make(chan struct{}, maxConc)

			groupTriggerMode := strings.ToLower(strings.TrimSpace(configutil.FlagOrViperString(cmd, "slack-group-trigger-mode", "slack.group_trigger_mode")))
			if groupTriggerMode == "" {
				groupTriggerMode = "smart"
			}
			addressingLLMTimeout := requestTimeout
			addressingConfidenceThreshold := configutil.FlagOrViperFloat64(cmd, "slack-addressing-confidence-threshold", "slack.addressing_confidence_threshold")
			if addressingConfidenceThreshold <= 0 {
				addressingConfidenceThreshold = 0.6
			}
			if addressingConfidenceThreshold > 1 {
				addressingConfidenceThreshold = 1
			}
			addressingInterjectThreshold := configutil.FlagOrViperFloat64(cmd, "slack-addressing-interject-threshold", "slack.addressing_interject_threshold")
			if addressingInterjectThreshold <= 0 {
				addressingInterjectThreshold = 0.6
			}
			if addressingInterjectThreshold > 1 {
				addressingInterjectThreshold = 1
			}

			healthListen := healthcheck.NormalizeListen(configutil.FlagOrViperString(cmd, "health-listen", "health.listen"))
			if healthListen != "" {
				healthServer, err := healthcheck.StartServer(cmd.Context(), logger, healthListen, "slack")
				if err != nil {
					logger.Warn("slack_health_server_start_error", "addr", healthListen, "error", err.Error())
				} else {
					defer func() {
						shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
						_ = healthServer.Shutdown(shutdownCtx)
						cancel()
					}()
				}
			}

			var (
				mu                  sync.Mutex
				history                          = make(map[string][]chathistory.ChatHistoryItem)
				stickySkillsByConv               = make(map[string][]string)
				workers                          = make(map[string]*slackConversationWorker)
				sharedGuard         *guard.Guard = guardFromViper(logger)
				enqueueSlackInbound func(context.Context, busruntime.BusMessage) error
			)

			getOrStartWorkerLocked := func(conversationKey string) *slackConversationWorker {
				if w, ok := workers[conversationKey]; ok && w != nil {
					return w
				}
				w := &slackConversationWorker{Jobs: make(chan slackJob, 16)}
				workers[conversationKey] = w
				go func(conversationKey string, w *slackConversationWorker) {
					for job := range w.Jobs {
						sem <- struct{}{}
						func() {
							defer func() { <-sem }()
							mu.Lock()
							h := append([]chathistory.ChatHistoryItem(nil), history[conversationKey]...)
							curVersion := w.Version
							sticky := append([]string(nil), stickySkillsByConv[conversationKey]...)
							mu.Unlock()
							if job.Version != curVersion {
								h = nil
							}
							ctx, cancel := context.WithTimeout(context.Background(), taskTimeout)
							final, _, loadedSkills, runErr := runSlackTask(
								ctx,
								logger,
								logOpts,
								client,
								reg,
								sharedGuard,
								cfg,
								llmModelFromViper(),
								job,
								h,
								sticky,
							)
							cancel()

							if runErr != nil {
								_, err := publishSlackBusOutbound(
									context.Background(),
									inprocBus,
									job.TeamID,
									job.ChannelID,
									"error: "+runErr.Error(),
									job.ThreadTS,
									fmt.Sprintf("slack:error:%s:%s", job.ChannelID, job.MessageTS),
								)
								if err != nil {
									logger.Warn("slack_bus_publish_error", "channel", busruntime.ChannelSlack, "channel_id", job.ChannelID, "bus_error_code", busErrorCodeString(err), "error", err.Error())
								}
								return
							}

							outText := strings.TrimSpace(formatFinalOutput(final))
							if outText != "" {
								_, err := publishSlackBusOutbound(
									context.Background(),
									inprocBus,
									job.TeamID,
									job.ChannelID,
									outText,
									job.ThreadTS,
									fmt.Sprintf("slack:message:%s:%s", job.ChannelID, job.MessageTS),
								)
								if err != nil {
									logger.Warn("slack_bus_publish_error", "channel", busruntime.ChannelSlack, "channel_id", job.ChannelID, "bus_error_code", busErrorCodeString(err), "error", err.Error())
								}
							}

							mu.Lock()
							if w.Version != curVersion {
								history[conversationKey] = nil
								stickySkillsByConv[conversationKey] = nil
							}
							if w.Version == curVersion && len(loadedSkills) > 0 {
								stickySkillsByConv[conversationKey] = capUniqueStrings(loadedSkills, slackStickySkillsCap)
							}
							cur := history[conversationKey]
							cur = append(cur, newSlackInboundHistoryItem(job))
							if outText != "" {
								cur = append(cur, newSlackOutboundAgentHistoryItem(job, outText, time.Now().UTC(), botUserID))
							}
							history[conversationKey] = trimChatHistoryItems(cur, slackHistoryCapForMode(groupTriggerMode))
							mu.Unlock()
						}()
					}
				}(conversationKey, w)
				return w
			}

			enqueueSlackInbound = func(ctx context.Context, msg busruntime.BusMessage) error {
				inbound, err := slackbus.InboundMessageFromBusMessage(msg)
				if err != nil {
					return err
				}
				text := strings.TrimSpace(inbound.Text)
				if text == "" {
					return fmt.Errorf("slack inbound text is required")
				}
				mu.Lock()
				w := getOrStartWorkerLocked(msg.ConversationKey)
				v := w.Version
				mu.Unlock()
				job := slackJob{
					ConversationKey: msg.ConversationKey,
					TeamID:          inbound.TeamID,
					ChannelID:       inbound.ChannelID,
					ChatType:        inbound.ChatType,
					MessageTS:       inbound.MessageTS,
					ThreadTS:        inbound.ThreadTS,
					UserID:          inbound.UserID,
					Text:            text,
					SentAt:          inbound.SentAt,
					Version:         v,
					MentionUsers:    append([]string(nil), inbound.MentionUsers...),
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case w.Jobs <- job:
					return nil
				}
			}

			busHandler := func(ctx context.Context, msg busruntime.BusMessage) error {
				switch msg.Direction {
				case busruntime.DirectionInbound:
					if msg.Channel != busruntime.ChannelSlack {
						return fmt.Errorf("unsupported inbound channel: %s", msg.Channel)
					}
					if enqueueSlackInbound == nil {
						return fmt.Errorf("slack inbound handler is not initialized")
					}
					return enqueueSlackInbound(ctx, msg)
				case busruntime.DirectionOutbound:
					if msg.Channel != busruntime.ChannelSlack {
						return fmt.Errorf("unsupported outbound channel: %s", msg.Channel)
					}
					_, _, err := slackDeliveryAdapter.Deliver(ctx, msg)
					return err
				default:
					return fmt.Errorf("unsupported direction: %s", msg.Direction)
				}
			}
			for _, topic := range busruntime.AllTopics() {
				if err := inprocBus.Subscribe(topic, busHandler); err != nil {
					return err
				}
			}

			appendIgnoredInboundHistory := func(event slackInboundEvent) {
				conversationKey, err := buildSlackConversationKey(event.TeamID, event.ChannelID)
				if err != nil {
					return
				}
				mu.Lock()
				cur := history[conversationKey]
				cur = append(cur, newSlackInboundHistoryItem(slackJob{
					ConversationKey: conversationKey,
					TeamID:          event.TeamID,
					ChannelID:       event.ChannelID,
					ChatType:        event.ChatType,
					MessageTS:       event.MessageTS,
					ThreadTS:        event.ThreadTS,
					UserID:          event.UserID,
					Text:            event.Text,
					SentAt:          event.SentAt,
					MentionUsers:    append([]string(nil), event.MentionUsers...),
				}))
				history[conversationKey] = trimChatHistoryItems(cur, slackHistoryCapForMode(groupTriggerMode))
				mu.Unlock()
			}

			logger.Info("slack_start",
				"bot_user_id", botUserID,
				"allowed_team_ids", len(allowedTeams),
				"allowed_channel_ids", len(allowedChannels),
				"task_timeout", taskTimeout.String(),
				"max_concurrency", maxConc,
				"group_trigger_mode", groupTriggerMode,
				"addressing_confidence_threshold", addressingConfidenceThreshold,
				"addressing_interject_threshold", addressingInterjectThreshold,
			)

			for {
				if cmd.Context().Err() != nil {
					logger.Info("slack_stop", "reason", "context_canceled")
					return nil
				}
				conn, err := api.connectSocket(cmd.Context())
				if err != nil {
					if cmd.Context().Err() != nil {
						logger.Info("slack_stop", "reason", "context_canceled")
						return nil
					}
					logger.Warn("slack_socket_connect_error", "error", err.Error())
					if err := sleepWithContext(cmd.Context(), 2*time.Second); err != nil {
						return nil
					}
					continue
				}
				logger.Info("slack_socket_connected")
				readErr := consumeSlackSocket(cmd.Context(), conn, func(envelope slackSocketEnvelope) error {
					event, ok, err := parseSlackInboundEvent(envelope, botUserID)
					if err != nil {
						return err
					}
					if !ok {
						return nil
					}
					if len(allowedTeams) > 0 && !allowedTeams[event.TeamID] {
						return nil
					}
					if len(allowedChannels) > 0 && !allowedChannels[event.ChannelID] {
						return nil
					}

					isGroup := isSlackGroupChat(event.ChatType)
					if isGroup {
						conversationKey, err := buildSlackConversationKey(event.TeamID, event.ChannelID)
						if err != nil {
							return err
						}
						mu.Lock()
						historySnapshot := append([]chathistory.ChatHistoryItem(nil), history[conversationKey]...)
						mu.Unlock()
						dec, accepted, err := decideSlackGroupTrigger(
							context.Background(),
							client,
							llmModelFromViper(),
							event,
							botUserID,
							groupTriggerMode,
							addressingLLMTimeout,
							addressingConfidenceThreshold,
							addressingInterjectThreshold,
							historySnapshot,
						)
						if err != nil {
							logger.Warn("slack_addressing_llm_error", "channel_id", event.ChannelID, "error", err.Error())
							return nil
						}
						if !accepted {
							if strings.EqualFold(groupTriggerMode, "talkative") {
								appendIgnoredInboundHistory(event)
							}
							return nil
						}
						event.ThreadTS = quoteReplyThreadTSForGroupTrigger(event, dec)
					}

					accepted, err := slackInboundAdapter.HandleInboundMessage(context.Background(), slackbus.InboundMessage{
						TeamID:       event.TeamID,
						ChannelID:    event.ChannelID,
						ChatType:     event.ChatType,
						MessageTS:    event.MessageTS,
						ThreadTS:     event.ThreadTS,
						UserID:       event.UserID,
						Text:         event.Text,
						SentAt:       event.SentAt,
						MentionUsers: append([]string(nil), event.MentionUsers...),
						EventID:      event.EventID,
					})
					if err != nil {
						logger.Warn("slack_bus_publish_error", "channel_id", event.ChannelID, "message_ts", event.MessageTS, "bus_error_code", busErrorCodeString(err), "error", err.Error())
						return nil
					}
					if !accepted {
						logger.Debug("slack_bus_inbound_deduped", "channel_id", event.ChannelID, "message_ts", event.MessageTS)
					}
					return nil
				})
				_ = conn.Close()
				if readErr != nil && !errors.Is(readErr, context.Canceled) && !errors.Is(readErr, context.DeadlineExceeded) {
					logger.Warn("slack_socket_read_error", "error", readErr.Error())
				}
			}
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

func consumeSlackSocket(ctx context.Context, conn *websocket.Conn, onEnvelope func(envelope slackSocketEnvelope) error) error {
	if conn == nil {
		return fmt.Errorf("slack websocket connection is nil")
	}
	for {
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var envelope slackSocketEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue
		}
		if strings.TrimSpace(envelope.EnvelopeID) != "" {
			if err := conn.WriteJSON(map[string]string{"envelope_id": envelope.EnvelopeID}); err != nil {
				return err
			}
		}
		if onEnvelope == nil {
			continue
		}
		if err := onEnvelope(envelope); err != nil {
			return err
		}
	}
}

func parseSlackInboundEvent(envelope slackSocketEnvelope, botUserID string) (slackInboundEvent, bool, error) {
	if strings.TrimSpace(envelope.Type) != "events_api" || len(envelope.Payload) == 0 {
		return slackInboundEvent{}, false, nil
	}
	var payload slackEventsAPIPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return slackInboundEvent{}, false, err
	}
	var event slackEvent
	if err := json.Unmarshal(payload.Event, &event); err != nil {
		return slackInboundEvent{}, false, err
	}
	eventType := strings.TrimSpace(event.Type)
	if eventType != "message" && eventType != "app_mention" {
		return slackInboundEvent{}, false, nil
	}
	subtype := strings.TrimSpace(event.Subtype)
	if subtype != "" {
		return slackInboundEvent{}, false, nil
	}
	if strings.TrimSpace(event.BotID) != "" {
		return slackInboundEvent{}, false, nil
	}
	userID := strings.TrimSpace(event.User)
	if userID == "" {
		return slackInboundEvent{}, false, nil
	}
	if userID == strings.TrimSpace(botUserID) {
		return slackInboundEvent{}, false, nil
	}
	channelID := strings.TrimSpace(event.Channel)
	if channelID == "" {
		return slackInboundEvent{}, false, nil
	}
	messageTS := strings.TrimSpace(event.TS)
	if messageTS == "" {
		return slackInboundEvent{}, false, nil
	}
	text := strings.TrimSpace(event.Text)
	if text == "" {
		return slackInboundEvent{}, false, nil
	}
	teamID := strings.TrimSpace(payload.TeamID)
	if teamID == "" {
		teamID = strings.TrimSpace(event.Team)
	}
	if teamID == "" && len(payload.Authorizations) > 0 {
		teamID = strings.TrimSpace(payload.Authorizations[0].TeamID)
	}
	if teamID == "" {
		return slackInboundEvent{}, false, fmt.Errorf("missing team_id in slack event")
	}
	chatType := normalizeSlackChatType(event.ChannelType, channelID)
	isAppMention := eventType == "app_mention"

	eventTime := payload.EventTime
	sentAt := time.Now().UTC()
	if eventTime > 0 {
		sentAt = time.Unix(eventTime, 0).UTC()
	}

	return slackInboundEvent{
		TeamID:          teamID,
		ChannelID:       channelID,
		ChatType:        chatType,
		MessageTS:       messageTS,
		ThreadTS:        strings.TrimSpace(event.ThreadTS),
		UserID:          userID,
		Text:            text,
		EventID:         strings.TrimSpace(payload.EventID),
		SentAt:          sentAt,
		MentionUsers:    collectSlackMentionUsers(text),
		IsAppMention:    isAppMention,
		IsThreadMessage: strings.TrimSpace(event.ThreadTS) != "",
	}, true, nil
}

func runSlackTask(
	ctx context.Context,
	logger *slog.Logger,
	logOpts agent.LogOptions,
	client llm.Client,
	baseReg *tools.Registry,
	sharedGuard *guard.Guard,
	cfg agent.Config,
	model string,
	job slackJob,
	history []chathistory.ChatHistoryItem,
	stickySkills []string,
) (*agent.Final, *agent.Context, []string, error) {
	task := strings.TrimSpace(job.Text)
	if task == "" {
		return nil, nil, nil, fmt.Errorf("empty slack task")
	}
	historyWithCurrent := append([]chathistory.ChatHistoryItem(nil), history...)
	historyWithCurrent = append(historyWithCurrent, newSlackInboundHistoryItem(job))
	historyRaw, err := json.MarshalIndent(map[string]any{
		"chat_history_messages": chathistory.BuildMessages(chathistory.ChannelSlack, historyWithCurrent),
	}, "", "  ")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("render slack history context: %w", err)
	}
	llmHistory := []llm.Message{{Role: "user", Content: string(historyRaw)}}

	if baseReg == nil {
		baseReg = registryFromViper()
		toolsutil.BindTodoUpdateToolLLM(baseReg, client, model)
	}
	reg := buildSlackRegistry(baseReg, job.ChatType)
	registerPlanTool(reg, client, model)
	toolsutil.BindTodoUpdateToolLLM(reg, client, model)
	toolsutil.SetTodoUpdateToolAddContext(reg, todoResolveContextForSlack(job))

	promptSpec, loadedSkills, skillAuthProfiles, err := promptSpecForSlack(ctx, logger, logOpts, task, client, model, stickySkills)
	if err != nil {
		return nil, nil, nil, err
	}
	promptprofile.ApplyPersonaIdentity(&promptSpec, logger)
	promptprofile.AppendLocalToolNotesBlock(&promptSpec, logger)
	promptprofile.AppendPlanCreateGuidanceBlock(&promptSpec, reg)

	engine := agent.New(
		client,
		reg,
		cfg,
		promptSpec,
		agent.WithLogger(logger),
		agent.WithLogOptions(logOpts),
		agent.WithSkillAuthProfiles(skillAuthProfiles, viper.GetBool("secrets.require_skill_profiles")),
		agent.WithGuard(sharedGuard),
	)

	meta := map[string]any{
		"trigger":            "slack",
		"slack_team_id":      job.TeamID,
		"slack_channel_id":   job.ChannelID,
		"slack_chat_type":    job.ChatType,
		"slack_message_ts":   job.MessageTS,
		"slack_thread_ts":    job.ThreadTS,
		"slack_from_user_id": job.UserID,
	}
	final, runCtx, err := engine.Run(ctx, task, agent.RunOptions{
		Model:           model,
		History:         llmHistory,
		Meta:            meta,
		SkipTaskMessage: true,
	})
	if err != nil {
		return final, runCtx, loadedSkills, err
	}
	return final, runCtx, loadedSkills, nil
}

func todoResolveContextForSlack(job slackJob) todo.AddResolveContext {
	user := strings.TrimSpace(job.UserID)
	if user != "" {
		user = "slack:" + user
	}
	mentions := normalizeMentionUsersForTodo(job.MentionUsers)
	return todo.AddResolveContext{
		Channel:          "slack",
		ChatType:         strings.ToLower(strings.TrimSpace(job.ChatType)),
		SpeakerUsername:  user,
		MentionUsernames: mentions,
		UserInputRaw:     job.Text,
	}
}

func normalizeMentionUsersForTodo(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		out = append(out, "slack:"+item)
	}
	return out
}

func newSlackInboundHistoryItem(job slackJob) chathistory.ChatHistoryItem {
	return chathistory.ChatHistoryItem{
		Channel:          chathistory.ChannelSlack,
		Kind:             chathistory.KindInboundUser,
		ChatID:           "slack:" + job.TeamID + ":" + job.ChannelID,
		ChatType:         strings.ToLower(strings.TrimSpace(job.ChatType)),
		MessageID:        strings.TrimSpace(job.MessageTS),
		ReplyToMessageID: strings.TrimSpace(job.ThreadTS),
		SentAt:           job.SentAt.UTC(),
		Sender:           slackSenderFromJob(job, false, ""),
		Text:             strings.TrimSpace(job.Text),
	}
}

func newSlackOutboundAgentHistoryItem(job slackJob, output string, sentAt time.Time, botUserID string) chathistory.ChatHistoryItem {
	return chathistory.ChatHistoryItem{
		Channel:          chathistory.ChannelSlack,
		Kind:             chathistory.KindOutboundAgent,
		ChatID:           "slack:" + job.TeamID + ":" + job.ChannelID,
		ChatType:         strings.ToLower(strings.TrimSpace(job.ChatType)),
		ReplyToMessageID: strings.TrimSpace(job.ThreadTS),
		SentAt:           sentAt.UTC(),
		Sender:           slackSenderFromJob(job, true, botUserID),
		Text:             strings.TrimSpace(output),
	}
}

func slackSenderFromJob(job slackJob, isBot bool, botUserID string) chathistory.ChatHistorySender {
	if isBot {
		return chathistory.ChatHistorySender{
			UserID:     strings.TrimSpace(botUserID),
			Username:   "slack-bot",
			Nickname:   "slack-bot",
			IsBot:      true,
			DisplayRef: "slack-bot",
		}
	}
	ref := strings.TrimSpace(job.UserID)
	if ref != "" {
		ref = "<@" + ref + ">"
	}
	return chathistory.ChatHistorySender{
		UserID:     strings.TrimSpace(job.UserID),
		Username:   strings.TrimSpace(job.UserID),
		Nickname:   ref,
		IsBot:      false,
		DisplayRef: ref,
	}
}

func slackHistoryCapForMode(mode string) int {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "talkative":
		return 16
	default:
		return 8
	}
}

func trimChatHistoryItems(items []chathistory.ChatHistoryItem, limit int) []chathistory.ChatHistoryItem {
	if limit <= 0 || len(items) <= limit {
		return append([]chathistory.ChatHistoryItem(nil), items...)
	}
	return append([]chathistory.ChatHistoryItem(nil), items[len(items)-limit:]...)
}

func capUniqueStrings(items []string, limit int) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func buildSlackConversationKey(teamID, channelID string) (string, error) {
	return busruntime.BuildSlackChannelConversationKey(strings.TrimSpace(teamID) + ":" + strings.TrimSpace(channelID))
}

func busErrorCodeString(err error) string {
	if err == nil {
		return ""
	}
	return string(busruntime.ErrorCodeOf(err))
}

func publishSlackBusOutbound(ctx context.Context, inprocBus *busruntime.Inproc, teamID, channelID, text, threadTS, correlationID string) (string, error) {
	if inprocBus == nil {
		return "", fmt.Errorf("bus is required")
	}
	if ctx == nil {
		return "", fmt.Errorf("context is required")
	}
	teamID = strings.TrimSpace(teamID)
	channelID = strings.TrimSpace(channelID)
	text = strings.TrimSpace(text)
	threadTS = strings.TrimSpace(threadTS)
	if teamID == "" {
		return "", fmt.Errorf("team_id is required")
	}
	if channelID == "" {
		return "", fmt.Errorf("channel_id is required")
	}
	if text == "" {
		return "", fmt.Errorf("text is required")
	}

	now := time.Now().UTC()
	messageID := "msg_" + uuid.NewString()
	sessionUUID, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	sessionID := sessionUUID.String()
	payloadBase64, err := busruntime.EncodeMessageEnvelope(busruntime.TopicChatMessage, busruntime.MessageEnvelope{
		MessageID: messageID,
		Text:      text,
		SentAt:    now.Format(time.RFC3339),
		SessionID: sessionID,
		ReplyTo:   threadTS,
	})
	if err != nil {
		return "", err
	}
	conversationKey, err := buildSlackConversationKey(teamID, channelID)
	if err != nil {
		return "", err
	}
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" {
		correlationID = "slack:" + messageID
	}
	outbound := busruntime.BusMessage{
		ID:              "bus_" + uuid.NewString(),
		Direction:       busruntime.DirectionOutbound,
		Channel:         busruntime.ChannelSlack,
		Topic:           busruntime.TopicChatMessage,
		ConversationKey: conversationKey,
		IdempotencyKey:  idempotency.MessageEnvelopeKey(messageID),
		CorrelationID:   correlationID,
		PayloadBase64:   payloadBase64,
		CreatedAt:       now,
		Extensions: busruntime.MessageExtensions{
			SessionID: sessionID,
			ReplyTo:   threadTS,
			ThreadTS:  threadTS,
			TeamID:    teamID,
			ChannelID: channelID,
		},
	}
	if err := inprocBus.PublishValidated(ctx, outbound); err != nil {
		return "", err
	}
	return messageID, nil
}

func toAllowlist(items []string) map[string]bool {
	out := make(map[string]bool)
	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		out[item] = true
	}
	return out
}

func isSlackGroupChat(chatType string) bool {
	switch strings.ToLower(strings.TrimSpace(chatType)) {
	case "channel", "private_channel", "mpim":
		return true
	default:
		return false
	}
}

func normalizeSlackChatType(channelType, channelID string) string {
	channelType = strings.ToLower(strings.TrimSpace(channelType))
	switch channelType {
	case "im", "mpim", "channel", "private_channel":
		return channelType
	}
	switch {
	case strings.HasPrefix(channelID, "D"):
		return "im"
	case strings.HasPrefix(channelID, "C"):
		return "channel"
	case strings.HasPrefix(channelID, "G"):
		return "private_channel"
	default:
		return "channel"
	}
}

func collectSlackMentionUsers(text string) []string {
	matches := slackMentionPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(matches))
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		userID := strings.TrimSpace(match[1])
		if userID == "" || seen[userID] {
			continue
		}
		seen[userID] = true
		out = append(out, userID)
	}
	return out
}

func quoteReplyThreadTSForGroupTrigger(event slackInboundEvent, dec slackGroupTriggerDecision) string {
	threadTS := strings.TrimSpace(event.ThreadTS)
	if threadTS != "" {
		return threadTS
	}
	if dec.AddressingImpulse > 0.8 {
		return strings.TrimSpace(event.MessageTS)
	}
	return ""
}

func decideSlackGroupTrigger(
	ctx context.Context,
	client llm.Client,
	model string,
	event slackInboundEvent,
	botUserID string,
	mode string,
	addressingLLMTimeout time.Duration,
	addressingConfidenceThreshold float64,
	addressingInterjectThreshold float64,
	history []chathistory.ChatHistoryItem,
) (slackGroupTriggerDecision, bool, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "smart"
	}
	if addressingConfidenceThreshold <= 0 {
		addressingConfidenceThreshold = 0.6
	}
	if addressingConfidenceThreshold > 1 {
		addressingConfidenceThreshold = 1
	}
	if addressingInterjectThreshold <= 0 {
		addressingInterjectThreshold = 0.6
	}
	if addressingInterjectThreshold > 1 {
		addressingInterjectThreshold = 1
	}
	if reason, ok := slackExplicitMentionReason(event, botUserID); ok {
		return slackGroupTriggerDecision{
			Reason:            reason,
			AddressingImpulse: 1,
		}, true, nil
	}

	runAddressingLLM := func(confidenceThreshold, interjectThreshold float64, requireAddressed bool, fallbackReason string) (slackGroupTriggerDecision, bool, error) {
		dec := slackGroupTriggerDecision{
			AddressingLLMAttempted: true,
			Reason:                 strings.TrimSpace(fallbackReason),
		}
		addrCtx := ctx
		if addrCtx == nil {
			addrCtx = context.Background()
		}
		cancel := func() {}
		if addressingLLMTimeout > 0 {
			addrCtx, cancel = context.WithTimeout(addrCtx, addressingLLMTimeout)
		}
		llmDec, llmOK, llmErr := slackAddressingDecisionViaLLM(addrCtx, client, model, event, history)
		cancel()
		if llmErr != nil {
			return dec, false, llmErr
		}
		dec.AddressingLLMOK = llmOK
		dec.AddressingLLMAddressed = llmDec.Addressed
		dec.AddressingLLMConfidence = llmDec.Confidence
		dec.AddressingLLMInterject = llmDec.Interject
		dec.AddressingImpulse = llmDec.Impulse
		if strings.TrimSpace(llmDec.Reason) != "" {
			dec.Reason = strings.TrimSpace(llmDec.Reason)
		}
		addressedOK := true
		if requireAddressed {
			addressedOK = llmDec.Addressed
		}
		if llmOK && addressedOK && llmDec.Confidence >= confidenceThreshold && llmDec.Interject > interjectThreshold {
			dec.UsedAddressingLLM = true
			return dec, true, nil
		}
		return dec, false, nil
	}

	switch mode {
	case "talkative":
		return runAddressingLLM(addressingConfidenceThreshold, addressingInterjectThreshold, false, mode)
	case "smart":
		return runAddressingLLM(addressingConfidenceThreshold, addressingInterjectThreshold, true, mode)
	default:
		return slackGroupTriggerDecision{}, false, nil
	}
}

func slackExplicitMentionReason(event slackInboundEvent, botUserID string) (string, bool) {
	if event.IsAppMention {
		return "app_mention", true
	}
	if strings.TrimSpace(botUserID) != "" && strings.Contains(event.Text, "<@"+strings.TrimSpace(botUserID)+">") {
		return "mention", true
	}
	if event.IsThreadMessage {
		return "thread_reply", true
	}
	return "", false
}

func slackAddressingDecisionViaLLM(ctx context.Context, client llm.Client, model string, event slackInboundEvent, history []chathistory.ChatHistoryItem) (slackAddressingLLMDecision, bool, error) {
	if ctx == nil || client == nil {
		return slackAddressingLLMDecision{}, false, nil
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return slackAddressingLLMDecision{}, false, fmt.Errorf("missing model for addressing_llm")
	}
	personaIdentity := loadAddressingPersonaIdentity()
	if personaIdentity == "" {
		personaIdentity = "You are MisterMorph, a general-purpose AI agent that can use tools to complete tasks."
	}
	historyMessages := chathistory.BuildMessages(chathistory.ChannelSlack, history)
	systemPrompt := strings.TrimSpace(strings.Join([]string{
		personaIdentity,
		"You are deciding whether the latest Slack group message should trigger an agent run.",
		"Return strict JSON with fields: addressed (bool), confidence (0..1), interject (0..1), impulse (0..1), reason (string).",
		"`addressed=true` means the user is clearly asking the bot or directly addressing the bot in context.",
	}, "\n"))
	userPayload, _ := json.Marshal(map[string]any{
		"current_message": map[string]any{
			"team_id":       event.TeamID,
			"channel_id":    event.ChannelID,
			"chat_type":     event.ChatType,
			"message_ts":    event.MessageTS,
			"thread_ts":     event.ThreadTS,
			"user_id":       event.UserID,
			"text":          event.Text,
			"mention_users": append([]string(nil), event.MentionUsers...),
		},
		"chat_history_messages": historyMessages,
	})
	res, err := client.Chat(llminspect.WithModelScene(ctx, "slack.addressing_decision"), llm.Request{
		Model:     model,
		ForceJSON: true,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: string(userPayload)},
		},
	})
	if err != nil {
		return slackAddressingLLMDecision{}, false, err
	}
	raw := strings.TrimSpace(res.Text)
	if raw == "" {
		return slackAddressingLLMDecision{}, false, fmt.Errorf("empty addressing_llm response")
	}
	var out slackAddressingLLMDecision
	if err := jsonutil.DecodeWithFallback(raw, &out); err != nil {
		return slackAddressingLLMDecision{}, false, fmt.Errorf("invalid addressing_llm json")
	}
	if out.Confidence < 0 {
		out.Confidence = 0
	}
	if out.Confidence > 1 {
		out.Confidence = 1
	}
	if out.Interject < 0 {
		out.Interject = 0
	}
	if out.Interject > 1 {
		out.Interject = 1
	}
	if out.Impulse < 0 {
		out.Impulse = 0
	}
	if out.Impulse > 1 {
		out.Impulse = 1
	}
	out.Reason = strings.TrimSpace(out.Reason)
	return out, true, nil
}

func loadAddressingPersonaIdentity() string {
	spec := agent.PromptSpec{}
	promptprofile.ApplyPersonaIdentity(&spec, slog.Default())
	return strings.TrimSpace(spec.Identity)
}

func buildSlackRegistry(baseReg *tools.Registry, chatType string) *tools.Registry {
	reg := tools.NewRegistry()
	if baseReg == nil {
		return reg
	}
	groupChat := isSlackGroupChat(chatType)
	for _, t := range baseReg.All() {
		name := strings.TrimSpace(t.Name())
		if groupChat && strings.EqualFold(name, "contacts_send") {
			continue
		}
		reg.Register(t)
	}
	return reg
}
