package main

import (
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/quailyquaily/mistermorph/internal/pathutil"
	"github.com/quailyquaily/mistermorph/internal/statepaths"
	"github.com/quailyquaily/mistermorph/secrets"
	"github.com/quailyquaily/mistermorph/tools"
	"github.com/quailyquaily/mistermorph/tools/builtin"
	"github.com/spf13/viper"
)

func registryFromViper() *tools.Registry {
	return buildRegistryFromConfig(loadRegistryConfigFromViper(), slog.Default())
}

type registryConfig struct {
	UserAgent                     string
	SecretsEnabled                bool
	SecretsRequireSkillProfiles   bool
	SecretsAllowProfiles          []string
	SecretsAliases                map[string]string
	AuthProfiles                  map[string]secrets.AuthProfile
	FileCacheDir                  string
	FileStateDir                  string
	ToolsReadFileMaxBytes         int64
	ToolsReadFileDenyPaths        []string
	ToolsWriteFileEnabled         bool
	ToolsWriteFileMaxBytes        int
	ToolsBashEnabled              bool
	ToolsBashTimeout              time.Duration
	ToolsBashMaxOutputBytes       int
	ToolsBashDenyPaths            []string
	ToolsURLFetchEnabled          bool
	ToolsURLFetchTimeout          time.Duration
	ToolsURLFetchMaxBytes         int64
	ToolsURLFetchMaxBytesDownload int64
	ToolsWebSearchEnabled         bool
	ToolsWebSearchTimeout         time.Duration
	ToolsWebSearchMaxResults      int
	ToolsWebSearchBaseURL         string
	ToolsContactsEnabled          bool
	ToolsTodoUpdateEnabled        bool
	TODOPathWIP                   string
	TODOPathDone                  string
	ContactsDir                   string
	TelegramBotToken              string
	TelegramBaseURL               string
	SlackBotToken                 string
	SlackBaseURL                  string
	ContactsFailureCooldown       time.Duration
}

func applyRegistryViperDefaults() {
	viper.SetDefault("tools.read_file.max_bytes", 256*1024)
	viper.SetDefault("tools.read_file.deny_paths", []string{"config.yaml"})

	viper.SetDefault("tools.write_file.enabled", true)
	viper.SetDefault("tools.write_file.max_bytes", 512*1024)

	viper.SetDefault("tools.bash.enabled", true)
	viper.SetDefault("tools.bash.timeout", 30*time.Second)
	viper.SetDefault("tools.bash.max_output_bytes", 256*1024)
	viper.SetDefault("tools.bash.deny_paths", []string{"config.yaml"})

	viper.SetDefault("tools.url_fetch.enabled", true)
	viper.SetDefault("tools.url_fetch.timeout", 30*time.Second)
	viper.SetDefault("tools.url_fetch.max_bytes", int64(512*1024))
	viper.SetDefault("tools.url_fetch.max_bytes_download", int64(100*1024*1024))
	viper.SetDefault("tools.web_search.enabled", true)
	viper.SetDefault("tools.web_search.timeout", 20*time.Second)
	viper.SetDefault("tools.web_search.max_results", 5)
	viper.SetDefault("tools.web_search.base_url", "https://duckduckgo.com/html/")
	viper.SetDefault("tools.contacts.enabled", true)
	viper.SetDefault("tools.todo_update.enabled", true)
}

func loadRegistryConfigFromViper() registryConfig {
	applyRegistryViperDefaults()

	authProfiles := map[string]secrets.AuthProfile{}
	_ = viper.UnmarshalKey("auth_profiles", &authProfiles)
	for id, profile := range authProfiles {
		profile.ID = id
		authProfiles[id] = profile
	}

	secretsAliases := map[string]string{}
	_ = viper.UnmarshalKey("secrets.aliases", &secretsAliases)

	fileStateDir := strings.TrimSpace(viper.GetString("file_state_dir"))

	return registryConfig{
		UserAgent:                     strings.TrimSpace(viper.GetString("user_agent")),
		SecretsEnabled:                viper.GetBool("secrets.enabled"),
		SecretsRequireSkillProfiles:   viper.GetBool("secrets.require_skill_profiles"),
		SecretsAllowProfiles:          append([]string(nil), viper.GetStringSlice("secrets.allow_profiles")...),
		SecretsAliases:                copyStringMap(secretsAliases),
		AuthProfiles:                  copyAuthProfilesMap(authProfiles),
		FileCacheDir:                  strings.TrimSpace(viper.GetString("file_cache_dir")),
		FileStateDir:                  fileStateDir,
		ToolsReadFileMaxBytes:         int64(viper.GetInt("tools.read_file.max_bytes")),
		ToolsReadFileDenyPaths:        append([]string(nil), viper.GetStringSlice("tools.read_file.deny_paths")...),
		ToolsWriteFileEnabled:         viper.GetBool("tools.write_file.enabled"),
		ToolsWriteFileMaxBytes:        viper.GetInt("tools.write_file.max_bytes"),
		ToolsBashEnabled:              viper.GetBool("tools.bash.enabled"),
		ToolsBashTimeout:              viper.GetDuration("tools.bash.timeout"),
		ToolsBashMaxOutputBytes:       viper.GetInt("tools.bash.max_output_bytes"),
		ToolsBashDenyPaths:            append([]string(nil), viper.GetStringSlice("tools.bash.deny_paths")...),
		ToolsURLFetchEnabled:          viper.GetBool("tools.url_fetch.enabled"),
		ToolsURLFetchTimeout:          viper.GetDuration("tools.url_fetch.timeout"),
		ToolsURLFetchMaxBytes:         viper.GetInt64("tools.url_fetch.max_bytes"),
		ToolsURLFetchMaxBytesDownload: viper.GetInt64("tools.url_fetch.max_bytes_download"),
		ToolsWebSearchEnabled:         viper.GetBool("tools.web_search.enabled"),
		ToolsWebSearchTimeout:         viper.GetDuration("tools.web_search.timeout"),
		ToolsWebSearchMaxResults:      viper.GetInt("tools.web_search.max_results"),
		ToolsWebSearchBaseURL:         strings.TrimSpace(viper.GetString("tools.web_search.base_url")),
		ToolsContactsEnabled:          viper.GetBool("tools.contacts.enabled"),
		ToolsTodoUpdateEnabled:        viper.GetBool("tools.todo_update.enabled"),
		TODOPathWIP:                   pathutil.ResolveStateFile(fileStateDir, statepaths.TODOWIPFilename),
		TODOPathDone:                  pathutil.ResolveStateFile(fileStateDir, statepaths.TODODONEFilename),
		ContactsDir:                   pathutil.ResolveStateChildDir(fileStateDir, strings.TrimSpace(viper.GetString("contacts.dir_name")), "contacts"),
		TelegramBotToken:              strings.TrimSpace(viper.GetString("telegram.bot_token")),
		TelegramBaseURL:               "https://api.telegram.org",
		SlackBotToken:                 strings.TrimSpace(viper.GetString("slack.bot_token")),
		SlackBaseURL:                  strings.TrimSpace(viper.GetString("slack.base_url")),
		ContactsFailureCooldown:       contactsFailureCooldownFromViper(),
	}
}

func buildRegistryFromConfig(cfg registryConfig, log *slog.Logger) *tools.Registry {
	r := tools.NewRegistry()
	if log == nil {
		log = slog.Default()
	}

	userAgent := strings.TrimSpace(cfg.UserAgent)
	secretsEnabled := cfg.SecretsEnabled
	secretsRequireSkillProfiles := cfg.SecretsRequireSkillProfiles

	allowProfiles := make(map[string]bool)
	for _, id := range cfg.SecretsAllowProfiles {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		allowProfiles[id] = true
	}

	authProfiles := copyAuthProfilesMap(cfg.AuthProfiles)

	for _, p := range authProfiles {
		if err := p.Validate(); err != nil {
			log.Warn("auth_profile_invalid", "profile", p.ID, "err", err)
			delete(authProfiles, p.ID)
		}
	}

	if secretsEnabled {
		log.Info("secrets_enabled",
			"require_skill_profiles", secretsRequireSkillProfiles,
			"allow_profiles", keysSorted(allowProfiles),
			"auth_profiles", len(authProfiles),
		)
	} else {
		if len(allowProfiles) > 0 || len(authProfiles) > 0 {
			log.Warn("secrets_disabled_but_configured",
				"allow_profiles", keysSorted(allowProfiles),
				"auth_profiles", len(authProfiles),
			)
		}
	}

	secretsAliases := copyStringMap(cfg.SecretsAliases)
	resolver := &secrets.EnvResolver{Aliases: secretsAliases}
	profileStore := secrets.NewProfileStore(authProfiles)

	r.Register(builtin.NewReadFileToolWithDenyPaths(
		cfg.ToolsReadFileMaxBytes,
		append([]string(nil), cfg.ToolsReadFileDenyPaths...),
		strings.TrimSpace(cfg.FileCacheDir),
		strings.TrimSpace(cfg.FileStateDir),
	))

	if cfg.ToolsWriteFileEnabled {
		r.Register(builtin.NewWriteFileTool(
			true,
			cfg.ToolsWriteFileMaxBytes,
			strings.TrimSpace(cfg.FileCacheDir),
			strings.TrimSpace(cfg.FileStateDir),
		))
	}

	if cfg.ToolsBashEnabled {
		bt := builtin.NewBashTool(
			true,
			cfg.ToolsBashTimeout,
			cfg.ToolsBashMaxOutputBytes,
			strings.TrimSpace(cfg.FileCacheDir),
			strings.TrimSpace(cfg.FileStateDir),
		)
		bt.DenyPaths = append([]string(nil), cfg.ToolsBashDenyPaths...)
		if secretsEnabled {
			// Safety default: allow bash for local automation, but deny curl to avoid "bash + curl" carrying auth.
			bt.DenyTokens = append(bt.DenyTokens, "curl")
		}
		r.Register(bt)
	}

	if cfg.ToolsURLFetchEnabled {
		r.Register(builtin.NewURLFetchToolWithAuthLimits(
			true,
			cfg.ToolsURLFetchTimeout,
			cfg.ToolsURLFetchMaxBytes,
			cfg.ToolsURLFetchMaxBytesDownload,
			userAgent,
			strings.TrimSpace(cfg.FileCacheDir),
			&builtin.URLFetchAuth{
				Enabled:       secretsEnabled,
				AllowProfiles: allowProfiles,
				Profiles:      profileStore,
				Resolver:      resolver,
			},
		))
	}

	if cfg.ToolsWebSearchEnabled {
		r.Register(builtin.NewWebSearchTool(
			true,
			cfg.ToolsWebSearchBaseURL,
			cfg.ToolsWebSearchTimeout,
			cfg.ToolsWebSearchMaxResults,
			userAgent,
		))
	}

	if cfg.ToolsTodoUpdateEnabled {
		r.Register(builtin.NewTodoUpdateTool(
			true,
			cfg.TODOPathWIP,
			cfg.TODOPathDone,
			cfg.ContactsDir,
		))
	}

	if cfg.ToolsContactsEnabled {
		r.Register(builtin.NewContactsSendTool(builtin.ContactsSendToolOptions{
			Enabled:          true,
			ContactsDir:      cfg.ContactsDir,
			TelegramBotToken: strings.TrimSpace(cfg.TelegramBotToken),
			TelegramBaseURL:  strings.TrimSpace(cfg.TelegramBaseURL),
			SlackBotToken:    strings.TrimSpace(cfg.SlackBotToken),
			SlackBaseURL:     strings.TrimSpace(cfg.SlackBaseURL),
			FailureCooldown:  cfg.ContactsFailureCooldown,
		}))
	}

	return r
}

func contactsFailureCooldownFromViper() time.Duration {
	if viper.IsSet("contacts.proactive.failure_cooldown") {
		if v := viper.GetDuration("contacts.proactive.failure_cooldown"); v > 0 {
			return v
		}
	}
	return 72 * time.Hour
}

func keysSorted(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyAuthProfilesMap(in map[string]secrets.AuthProfile) map[string]secrets.AuthProfile {
	if len(in) == 0 {
		return map[string]secrets.AuthProfile{}
	}
	out := make(map[string]secrets.AuthProfile, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
