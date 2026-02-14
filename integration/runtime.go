package integration

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/quailyquaily/mistermorph/agent"
	"github.com/quailyquaily/mistermorph/internal/llmconfig"
	"github.com/quailyquaily/mistermorph/internal/llminspect"
	"github.com/quailyquaily/mistermorph/internal/llmutil"
	"github.com/quailyquaily/mistermorph/internal/logutil"
	"github.com/quailyquaily/mistermorph/internal/promptprofile"
	"github.com/quailyquaily/mistermorph/internal/skillsutil"
	"github.com/quailyquaily/mistermorph/internal/toolsutil"
	"github.com/quailyquaily/mistermorph/llm"
	"github.com/quailyquaily/mistermorph/tools"
	"github.com/spf13/viper"
)

// Runtime is the reusable wiring entrypoint for third-party embedding.
type Runtime struct {
	cfg Config
}

type PreparedRun struct {
	Engine  *agent.Engine
	Model   string
	Cleanup func() error
}

func New(cfg Config) (*Runtime, error) {
	cfg = normalizeConfig(cfg)
	applyViperDefaults()

	for k, v := range cfg.Overrides {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		viper.Set(key, v)
	}

	return &Runtime{cfg: cfg}, nil
}

func normalizeConfig(cfg Config) Config {
	if isZeroConfig(cfg) {
		return DefaultConfig()
	}
	if cfg.Overrides == nil {
		cfg.Overrides = map[string]any{}
	}
	if len(cfg.BuiltinToolNames) > 0 {
		cfg.BuiltinToolNames = normalizeToolNames(cfg.BuiltinToolNames)
	}
	return cfg
}

func isZeroConfig(cfg Config) bool {
	if len(cfg.Overrides) > 0 {
		return false
	}
	if len(cfg.BuiltinToolNames) > 0 {
		return false
	}
	if cfg.Features != (Features{}) {
		return false
	}
	if cfg.Inspect != (InspectOptions{}) {
		return false
	}
	return true
}

func normalizeToolNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil
	}
	slices.Sort(out)
	return out
}

func (rt *Runtime) NewRegistry() *tools.Registry {
	if rt == nil {
		return tools.NewRegistry()
	}
	return rt.buildRegistryFromViper()
}

func (rt *Runtime) NewRunEngine(ctx context.Context, task string) (*PreparedRun, error) {
	return rt.NewRunEngineWithRegistry(ctx, task, nil)
}

func (rt *Runtime) NewRunEngineWithRegistry(ctx context.Context, task string, baseReg *tools.Registry) (*PreparedRun, error) {
	if rt == nil {
		return nil, fmt.Errorf("runtime is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	task = strings.TrimSpace(task)

	logger, err := logutil.LoggerFromViper()
	if err != nil {
		return nil, err
	}
	slog.SetDefault(logger)
	logOpts := logutil.LogOptionsFromViper()

	provider := llmutil.ProviderFromViper()
	endpoint := llmutil.EndpointForProvider(provider)
	apiKey := llmutil.APIKeyForProvider(provider)
	model := llmutil.ModelForProvider(provider)
	requestTimeout := viper.GetDuration("llm.request_timeout")

	client, err := llmutil.ClientFromConfig(llmconfig.ClientConfig{
		Provider:       provider,
		Endpoint:       endpoint,
		APIKey:         apiKey,
		Model:          model,
		RequestTimeout: requestTimeout,
	})
	if err != nil {
		return nil, err
	}

	client, inspectCleanup, err := rt.wrapClientWithInspect(client, task, rt.cfg.Inspect)
	if err != nil {
		return nil, err
	}

	reg := cloneRegistry(baseReg)
	if reg == nil {
		reg = rt.NewRegistry()
	}

	if rt.cfg.Features.PlanTool {
		toolsutil.RegisterPlanTool(reg, client, model)
	}
	toolsutil.BindTodoUpdateToolLLM(reg, client, model)

	skillAuthProfiles := []string{}
	promptSpec := agent.DefaultPromptSpec()
	if rt.cfg.Features.Skills {
		spec, _, authProfiles, err := skillsutil.PromptSpecWithSkills(ctx, logger, logOpts, task, client, model, skillsutil.SkillsConfigFromViper())
		if err != nil {
			_ = inspectCleanup()
			return nil, err
		}
		promptSpec = spec
		skillAuthProfiles = authProfiles
	}
	promptprofile.ApplyPersonaIdentity(&promptSpec, logger)
	promptprofile.AppendLocalToolNotesBlock(&promptSpec, logger)
	if rt.cfg.Features.PlanTool {
		promptprofile.AppendPlanCreateGuidanceBlock(&promptSpec, reg)
	}

	opts := []agent.Option{
		agent.WithLogger(logger),
		agent.WithLogOptions(logOpts),
		agent.WithSkillAuthProfiles(skillAuthProfiles, viper.GetBool("secrets.require_skill_profiles")),
	}
	if g := rt.buildGuardFromViper(logger); g != nil {
		opts = append(opts, agent.WithGuard(g))
	}

	engine := agent.New(
		client,
		reg,
		agent.Config{
			MaxSteps:       viper.GetInt("max_steps"),
			ParseRetries:   viper.GetInt("parse_retries"),
			MaxTokenBudget: viper.GetInt("max_token_budget"),
		},
		promptSpec,
		opts...,
	)

	return &PreparedRun{
		Engine: engine,
		Model:  model,
		Cleanup: func() error {
			return inspectCleanup()
		},
	}, nil
}

func (rt *Runtime) RunTask(ctx context.Context, task string, opts agent.RunOptions) (*agent.Final, *agent.Context, error) {
	prepared, err := rt.NewRunEngine(ctx, task)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = prepared.Cleanup()
	}()

	if strings.TrimSpace(opts.Model) == "" {
		opts.Model = prepared.Model
	}
	return prepared.Engine.Run(ctx, task, opts)
}

func cloneRegistry(base *tools.Registry) *tools.Registry {
	if base == nil {
		return nil
	}
	out := tools.NewRegistry()
	for _, t := range base.All() {
		out.Register(t)
	}
	return out
}

func (rt *Runtime) wrapClientWithInspect(client llm.Client, task string, inspect InspectOptions) (llm.Client, func() error, error) {
	if client == nil {
		return nil, func() error { return nil }, fmt.Errorf("llm client is nil")
	}

	closers := make([]func() error, 0, 2)
	cleanup := func() error {
		var firstErr error
		for i := len(closers) - 1; i >= 0; i-- {
			if err := closers[i](); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	if inspect.Request {
		inspector, err := llminspect.NewRequestInspector(llminspect.Options{
			Mode:            strings.TrimSpace(inspect.Mode),
			Task:            strings.TrimSpace(task),
			TimestampFormat: strings.TrimSpace(inspect.TimestampFormat),
			DumpDir:         strings.TrimSpace(inspect.DumpDir),
		})
		if err != nil {
			return nil, cleanup, err
		}
		closers = append(closers, inspector.Close)
		if err := llminspect.SetDebugHook(client, inspector.Dump); err != nil {
			_ = cleanup()
			return nil, cleanup, err
		}
	}

	if inspect.Prompt {
		inspector, err := llminspect.NewPromptInspector(llminspect.Options{
			Mode:            strings.TrimSpace(inspect.Mode),
			Task:            strings.TrimSpace(task),
			TimestampFormat: strings.TrimSpace(inspect.TimestampFormat),
			DumpDir:         strings.TrimSpace(inspect.DumpDir),
		})
		if err != nil {
			_ = cleanup()
			return nil, cleanup, err
		}
		closers = append(closers, inspector.Close)
		client = &llminspect.PromptClient{Base: client, Inspector: inspector}
	}

	return client, cleanup, nil
}

func (rt *Runtime) RequestTimeout() time.Duration {
	if rt == nil {
		return 0
	}
	return viper.GetDuration("llm.request_timeout")
}
