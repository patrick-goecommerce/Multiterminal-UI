// Package config loads and provides application configuration.
//
// On first run, a default YAML config is written to ~/.multiterminal.yaml.
// Subsequent runs read and merge that file with built-in defaults.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all user-configurable settings.
type Config struct {
	DefaultShell          string         `yaml:"default_shell" json:"default_shell"`
	DefaultDir            string         `yaml:"default_dir" json:"default_dir"`
	// LastOpenedDir is updated automatically whenever the user opens or
	// creates a project folder. It takes precedence over DefaultDir when
	// resolving the working directory for a fresh app start (see
	// GetWorkingDir), so MTUI starts back up where the user left off.
	LastOpenedDir         string         `yaml:"last_opened_dir" json:"last_opened_dir"`
	Theme                 string         `yaml:"theme" json:"theme"`
	TerminalColor         string         `yaml:"terminal_color" json:"terminal_color"`
	MaxPanesPerTab        int            `yaml:"max_panes_per_tab" json:"max_panes_per_tab"`
	SidebarWidth          int            `yaml:"sidebar_width" json:"sidebar_width"`
	ClaudeCommand         string         `yaml:"claude_command" json:"claude_command"`
	ClaudeModels          []ModelEntry   `yaml:"claude_models" json:"claude_models"`
	ClaudeEnabled         *bool          `yaml:"claude_enabled" json:"claude_enabled"`
	CodexCommand          string         `yaml:"codex_command" json:"codex_command"`
	CodexModels           []ModelEntry   `yaml:"codex_models" json:"codex_models"`
	CodexEnabled          *bool          `yaml:"codex_enabled" json:"codex_enabled"`
	GeminiCommand         string         `yaml:"gemini_command" json:"gemini_command"`
	GeminiModels          []ModelEntry   `yaml:"gemini_models" json:"gemini_models"`
	GeminiEnabled         *bool          `yaml:"gemini_enabled" json:"gemini_enabled"`
	CommitReminderMinutes int            `yaml:"commit_reminder_minutes" json:"commit_reminder_minutes"`
	RestoreSession        *bool          `yaml:"restore_session" json:"restore_session"`
	LoggingEnabled        bool           `yaml:"logging_enabled" json:"logging_enabled"`
	// AutoBranchOnIssue: true (default) isolates every issue-launched pane in
	// its own deterministic worktree (CreateIssueWorktree) instead of leaving
	// it in the main repo's working directory. false disables automatic
	// branch/worktree handling entirely — the user manages branches manually.
	AutoBranchOnIssue     *bool          `yaml:"auto_branch_on_issue" json:"auto_branch_on_issue"`
	// ForceWorktrees: when true, Claude panes may only modify code inside a
	// worktree — the mtui-hook PreToolUse firewall denies Edit/Write/NotebookEdit
	// targeting the main checkout while no worktree is active, so the model has
	// to call EnterWorktree first (see cmd/mtui-hook/firewall.go). Documentation
	// and planning paths (.md, docs/, .mtui/, .claude/) stay exempt. Opt-in:
	// nil/false keeps the pre-existing advisory-only behaviour. A per-project
	// override lives in .mtui/config.json — resolve both via
	// AppService.EffectiveForceWorktrees, never this field alone.
	ForceWorktrees        *bool          `yaml:"force_worktrees" json:"force_worktrees"`
	IssueTracking         IssueTracking  `yaml:"issue_tracking" json:"issue_tracking"`
	Commands              []CommandEntry `yaml:"commands" json:"commands"`
	// FinishPrepPrompt overrides the built-in worktree-finish prep prompt
	// (see app_worktree_finish.go prepPromptTemplate) when non-empty. Supports
	// the same {{branch}}/{{targetBranch}}/{{worktreePath}} placeholders.
	FinishPrepPrompt      string         `yaml:"finish_prep_prompt" json:"finish_prep_prompt"`
	QuickActions          []QuickAction  `yaml:"quick_actions" json:"quick_actions"`
	Audio                 AudioSettings  `yaml:"audio" json:"audio"`
	KeepAlive             KeepAliveSettings `yaml:"keep_alive" json:"keep_alive"`
	StatusLine            StatusLineSettings `yaml:"status_line" json:"status_line"`
	LocalhostAutoOpen     string         `yaml:"localhost_auto_open" json:"localhost_auto_open"`
	SidebarPinned         bool           `yaml:"sidebar_pinned" json:"sidebar_pinned"`
	Favorites             map[string][]string `yaml:"favorites,omitempty" json:"favorites,omitempty"`
	FontFamily            string         `yaml:"font_family" json:"font_family"`
	FontSize              int            `yaml:"font_size"   json:"font_size"`
	BackgroundAgents      BackgroundAgents `yaml:"background_agents" json:"background_agents"`
	Orchestrator          OrchestratorSettings `yaml:"orchestrator" json:"orchestrator"`
	Language              string         `yaml:"language" json:"language"`
	SetupDone             bool           `yaml:"setup_done" json:"setup_done"`
	ChatStyle             string         `yaml:"chat_style" json:"chat_style"`
	STT                   STTSettings    `yaml:"stt" json:"stt"`
	AutoNaming            AutoNamingSettings `yaml:"auto_naming" json:"auto_naming"`
	MCPServer             MCPServerSettings `yaml:"mcp_server" json:"mcp_server"`
	// UpdateChannel selects which GitHub release track CheckForUpdates/ApplyUpdate
	// pull from: "stable" (release.yml, non-prerelease) or "alpha" (release-alpha.yml,
	// prerelease). Defaults to "stable" regardless of the running build variant.
	UpdateChannel         string         `yaml:"update_channel" json:"update_channel"`
	// AutoUpdateCheckMinutes: 0 (default) disables background update checks
	// entirely (fully opt-in, no automatic network calls). >0 enables a
	// periodic check every N minutes. Manual checks (Settings button) always
	// work regardless of this value.
	AutoUpdateCheckMinutes int           `yaml:"auto_update_check_minutes" json:"auto_update_check_minutes"`
	// TerminalScrollback is the xterm.js scrollback buffer size (lines kept
	// per pane). Must be one of validScrollbackSizes; defaults to 10000.
	TerminalScrollback    int           `yaml:"terminal_scrollback" json:"terminal_scrollback"`
}

// MCPServerSettings configures the local MCP server that lets an AI agent
// running inside an MTUI pane delegate work by opening, feeding, and closing
// other MTUI sessions (e.g. handing a task to Codex or Gemini). It listens on
// 127.0.0.1 only; there is no auth layer, since this is a single-user desktop app.
type MCPServerSettings struct {
	Enabled *bool `yaml:"enabled" json:"enabled"`
	Port    int   `yaml:"port" json:"port"`
}

// AutoNamingSettings controls automatic pane naming for Claude panes. When
// enabled, a fresh user prompt triggers a one-shot model call (Model) that
// summarizes the task into a short pane title.
type AutoNamingSettings struct {
	Enabled *bool  `yaml:"enabled" json:"enabled"`
	Model   string `yaml:"model" json:"model"`
}

// IssueTracking holds settings for automatic issue progress reporting.
type IssueTracking struct {
	AutoCommentOnStart  bool `yaml:"auto_comment_on_start" json:"auto_comment_on_start"`
	AutoCommentOnDone   bool `yaml:"auto_comment_on_done" json:"auto_comment_on_done"`
	AutoCommentOnClose  bool `yaml:"auto_comment_on_close" json:"auto_comment_on_close"`
	AutoCloseIssue      bool `yaml:"auto_close_issue" json:"auto_close_issue"`
	IncludeCostInReport bool `yaml:"include_cost_in_report" json:"include_cost_in_report"`
}

// ModelEntry represents a selectable Claude model in the launch dialog.
type ModelEntry struct {
	Label string `yaml:"label" json:"label"`
	ID    string `yaml:"id" json:"id"`
}

// CommandEntry represents a user-defined command in the command palette.
type CommandEntry struct {
	Name string `yaml:"name" json:"name"`
	Text string `yaml:"text" json:"text"`
}

// QuickAction represents a user-defined pane-titlebar button that sends a
// (placeholder-templated) prompt into the pane's prompt queue. Placeholders
// {{branch}}, {{targetBranch}}, {{worktreePath}} are substituted at send time.
type QuickAction struct {
	Label  string `yaml:"label" json:"label"`
	Prompt string `yaml:"prompt" json:"prompt"`
}

// AudioSettings holds audio feedback configuration.
type AudioSettings struct {
	Enabled     *bool  `yaml:"enabled" json:"enabled"`
	Volume      int    `yaml:"volume" json:"volume"`
	WhenFocused *bool  `yaml:"when_focused" json:"when_focused"`
	DoneSound   string `yaml:"done_sound" json:"done_sound"`
	InputSound  string `yaml:"input_sound" json:"input_sound"`
	ErrorSound  string `yaml:"error_sound" json:"error_sound"`
}

// KeepAliveSettings controls the automatic Claude session keep-alive feature.
type KeepAliveSettings struct {
	Enabled         *bool  `yaml:"enabled" json:"enabled"`
	IntervalMinutes int    `yaml:"interval_minutes" json:"interval_minutes"`
	Message         string `yaml:"message" json:"message"`
}

// StatusLineSettings controls the Claude Code statusline written to ~/.claude/settings.json.
type StatusLineSettings struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	Template      string `yaml:"template" json:"template"` // "minimal", "standard", "extended"
	ShowModel     bool   `yaml:"show_model" json:"show_model"`
	ShowContext   bool   `yaml:"show_context" json:"show_context"`
	ShowCost      bool   `yaml:"show_cost" json:"show_cost"`
	ShowGitBranch bool   `yaml:"show_git_branch" json:"show_git_branch"`
	ShowDuration  bool   `yaml:"show_duration" json:"show_duration"`
}

// BackgroundAgents holds settings for automatic background review and test agents.
type BackgroundAgents struct {
	ReviewEnabled *bool  `yaml:"review_enabled" json:"review_enabled"`
	ReviewTool    string `yaml:"review_tool" json:"review_tool"`
	ReviewModel   string `yaml:"review_model" json:"review_model"`
	ReviewPrompt  string `yaml:"review_prompt" json:"review_prompt"`
	TestEnabled   *bool  `yaml:"test_enabled" json:"test_enabled"`
	TestCommand   string `yaml:"test_command" json:"test_command"`
}

// OrchestratorSettings holds agent orchestration configuration.
type OrchestratorSettings struct {
	MaxParallelAgents    int    `yaml:"max_parallel_agents" json:"max_parallel_agents"`
	DefaultAutoMerge     bool   `yaml:"default_auto_merge" json:"default_auto_merge"`
	DefaultAutoStart     bool   `yaml:"default_auto_start" json:"default_auto_start"`
	MaxRetries           int    `yaml:"max_retries" json:"max_retries"`
	ReviewCommand        string `yaml:"review_command" json:"review_command"`
	SyncSubtasksToGitHub bool   `yaml:"sync_subtasks_to_github" json:"sync_subtasks_to_github"`
}

// STTSettings configures speech-to-text voice input.
type STTSettings struct {
	Provider string           `yaml:"provider" json:"provider"` // cloud-whisper | whisper-cpp | parakeet
	Language string           `yaml:"language" json:"language"` // ISO code or "auto"
	Cloud    STTCloudSettings `yaml:"cloud" json:"cloud"`
}

// STTCloudSettings configures the cloud Whisper-compatible endpoint.
type STTCloudSettings struct {
	BaseURL string `yaml:"base_url" json:"base_url"` // empty = OpenAI default
	Model   string `yaml:"model" json:"model"`       // default whisper-1
	APIKey  string `yaml:"api_key" json:"api_key"`   // empty = $OPENAI_API_KEY
}

// normalizeSTT applies defaults/validation to STT settings.
func normalizeSTT(c *Config) {
	valid := map[string]bool{"cloud-whisper": true, "whisper-cpp": true, "parakeet": true}
	if !valid[c.STT.Provider] {
		c.STT.Provider = "cloud-whisper"
	}
	if c.STT.Language == "" {
		c.STT.Language = "de"
	}
	if c.STT.Cloud.Model == "" {
		c.STT.Cloud.Model = "whisper-1"
	}
}

// DefaultConfig returns the built-in defaults.
// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool { return &b }

func DefaultConfig() Config {
	return Config{
		DefaultShell:          "",
		DefaultDir:            "",
		Theme:                 "konzept",
		TerminalColor:         "#39ff14",
		MaxPanesPerTab:        9,
		SidebarWidth:          30,
		ClaudeCommand:         "claude",
		ClaudeEnabled:         boolPtr(true),
		CodexCommand: "codex",
		CodexModels: []ModelEntry{
			{Label: "Default", ID: ""},
			{Label: "o4-mini", ID: "o4-mini"},
			{Label: "o3", ID: "o3"},
			{Label: "GPT-4.1", ID: "gpt-4.1"},
		},
		CodexEnabled:  boolPtr(false),
		GeminiCommand: "gemini",
		GeminiModels: []ModelEntry{
			{Label: "Default", ID: ""},
			{Label: "Gemini 2.5 Pro", ID: "gemini-2.5-pro"},
			{Label: "Gemini 2.5 Flash", ID: "gemini-2.5-flash"},
		},
		GeminiEnabled: boolPtr(false),
		CommitReminderMinutes: 30,
		RestoreSession:        boolPtr(true),
		AutoBranchOnIssue:     boolPtr(true),
		ForceWorktrees:        boolPtr(false),
		IssueTracking: IssueTracking{
			AutoCommentOnStart:  true,
			AutoCommentOnDone:   true,
			AutoCommentOnClose:  true,
			AutoCloseIssue:      false,
			IncludeCostInReport: true,
		},
		// "opus"/"sonnet"/"fable"/"haiku" are the claude CLI's own aliases and
		// always resolve to the latest model in that tier (per `claude --help`
		// and confirmed in practice for haiku too), so this list never needs
		// updating as Anthropic ships new versions.
		ClaudeModels: []ModelEntry{
			{Label: "Default", ID: ""},
			{Label: "Opus (latest)", ID: "opus"},
			{Label: "Sonnet (latest)", ID: "sonnet"},
			{Label: "Fable (latest)", ID: "fable"},
			{Label: "Haiku (latest)", ID: "haiku"},
		},
		Commands: []CommandEntry{
			{Name: "Commit & Push", Text: "git add -A && git commit -m 'update' && git push"},
		},
		Audio: AudioSettings{
			Enabled:     boolPtr(true),
			Volume:      50,
			WhenFocused: boolPtr(true),
		},
		KeepAlive: KeepAliveSettings{
			Enabled:         boolPtr(true),
			IntervalMinutes: 60,
			Message:         "Hi!",
		},
		StatusLine: StatusLineSettings{
			Enabled:     false,
			Template:    "standard",
			ShowModel:   true,
			ShowContext: true,
			ShowCost:    true,
		},
		BackgroundAgents: BackgroundAgents{
			ReviewEnabled: boolPtr(false),
			ReviewTool:    "claude",
			ReviewModel:   "claude-haiku-4-5-20251001",
			ReviewPrompt:  "Review the following commit diff. Flag bugs, security issues, and code quality problems:\n\n{diff}",
			TestEnabled:   boolPtr(false),
			TestCommand:   "",
		},
		LocalhostAutoOpen: "notify",
		FontFamily:        "",
		FontSize:          10,
		Orchestrator: OrchestratorSettings{
			MaxParallelAgents:    3,
			DefaultAutoMerge:     false,
			DefaultAutoStart:     false,
			MaxRetries:           2,
			ReviewCommand:        "go test ./... && go vet ./...",
			SyncSubtasksToGitHub: false,
		},
		Language:  "de",
		SetupDone: false,
		ChatStyle: "claude-code",
		STT: STTSettings{Provider: "cloud-whisper", Language: "de", Cloud: STTCloudSettings{Model: "whisper-1"}},
		AutoNaming: AutoNamingSettings{
			Enabled: boolPtr(true),
			Model:   "claude-haiku-4-5",
		},
		MCPServer: MCPServerSettings{
			Enabled: boolPtr(true),
			Port:    51533,
		},
		UpdateChannel:          "stable",
		AutoUpdateCheckMinutes: 0,
		TerminalScrollback:     10000,
	}
}

// ShouldRestoreSession returns whether the session should be restored.
func (c Config) ShouldRestoreSession() bool {
	if c.RestoreSession == nil {
		return true
	}
	return *c.RestoreSession
}

// ShouldAutoBranch returns whether to auto-isolate issue-launched panes into
// their own worktree (see AutoBranchOnIssue).
func (c Config) ShouldAutoBranch() bool {
	if c.AutoBranchOnIssue == nil {
		return true
	}
	return *c.AutoBranchOnIssue
}

// ShouldForceWorktrees returns the GLOBAL worktree-mandatory setting (see
// ForceWorktrees). The nil check lives here rather than only in Load because
// SaveConfig assigns AppService.cfg directly and bypasses Load entirely, so
// the field can legitimately be nil at runtime.
//
// Callers in the backend should prefer AppService.EffectiveForceWorktrees,
// which layers the per-project override on top of this.
func (c Config) ShouldForceWorktrees() bool {
	if c.ForceWorktrees == nil {
		return false
	}
	return *c.ForceWorktrees
}

// ShouldAutoName returns whether automatic pane naming is enabled.
func (c Config) ShouldAutoName() bool {
	if c.AutoNaming.Enabled == nil {
		return true
	}
	return *c.AutoNaming.Enabled
}

// ShouldKeepAlive returns whether the keep-alive feature is enabled.
func (c Config) ShouldKeepAlive() bool {
	if c.KeepAlive.Enabled == nil {
		return true
	}
	return *c.KeepAlive.Enabled
}

// ShouldRunMCPServer returns whether the local agent-control MCP server
// should be started.
func (c Config) ShouldRunMCPServer() bool {
	if c.MCPServer.Enabled == nil {
		return true
	}
	return *c.MCPServer.Enabled
}

// configPath returns the path to ~/.multiterminal.yaml.
func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".multiterminal.yaml")
}

// Load reads the config file, falling back to defaults for missing fields.
func Load() Config {
	cfg := DefaultConfig()
	// Saved before Unmarshal, which overwrites these wholesale (even with an
	// empty list) if the key is present in the YAML at all — the fallback
	// below needs the real defaults, not whatever Unmarshal left behind.
	defaultClaudeModels := cfg.ClaudeModels
	defaultCodexModels := cfg.CodexModels
	defaultGeminiModels := cfg.GeminiModels

	p := configPath()
	if p == "" {
		return cfg
	}

	data, err := os.ReadFile(p)
	if err != nil {
		// No config file yet – write defaults for future editing
		_ = writeDefaults(p, cfg)
		return cfg
	}

	_ = yaml.Unmarshal(data, &cfg)

	// An empty (as opposed to absent) claude/codex/gemini_models list in the
	// YAML — e.g. from a config file predating one of these fields, or any
	// past save that round-tripped an empty array — would otherwise persist
	// forever: yaml.Unmarshal only fills in defaults for missing keys, not
	// present-but-empty ones. Self-heal it back to the built-in model list.
	if len(cfg.ClaudeModels) == 0 {
		cfg.ClaudeModels = defaultClaudeModels
	}
	if len(cfg.CodexModels) == 0 {
		cfg.CodexModels = defaultCodexModels
	}
	if len(cfg.GeminiModels) == 0 {
		cfg.GeminiModels = defaultGeminiModels
	}

	// Apply sensible bounds
	if cfg.MaxPanesPerTab < 1 {
		cfg.MaxPanesPerTab = 1
	}
	if cfg.MaxPanesPerTab > 9 {
		cfg.MaxPanesPerTab = 9
	}
	if cfg.SidebarWidth < 15 {
		cfg.SidebarWidth = 15
	}
	if cfg.SidebarWidth > 60 {
		cfg.SidebarWidth = 60
	}

	// Validate theme name
	validThemes := map[string]bool{"konzept": true, "dark": true, "light": true, "dracula": true, "nord": true, "solarized": true}
	if !validThemes[cfg.Theme] {
		cfg.Theme = "konzept"
	}

	if cfg.CommitReminderMinutes < 0 {
		cfg.CommitReminderMinutes = 0
	}

	if cfg.Audio.Volume < 0 {
		cfg.Audio.Volume = 0
	}
	if cfg.Audio.Volume > 100 {
		cfg.Audio.Volume = 100
	}
	if cfg.Audio.Enabled == nil {
		cfg.Audio.Enabled = boolPtr(true)
	}
	if cfg.Audio.WhenFocused == nil {
		cfg.Audio.WhenFocused = boolPtr(true)
	}

	if cfg.KeepAlive.Enabled == nil {
		cfg.KeepAlive.Enabled = boolPtr(true)
	}
	if cfg.KeepAlive.IntervalMinutes <= 0 {
		cfg.KeepAlive.IntervalMinutes = 60
	}
	if cfg.KeepAlive.Message == "" {
		cfg.KeepAlive.Message = "Hi!"
	}

	if cfg.RestoreSession == nil {
		cfg.RestoreSession = boolPtr(true)
	}

	if cfg.ForceWorktrees == nil {
		cfg.ForceWorktrees = boolPtr(false)
	}

	// Validate localhost_auto_open
	validAutoOpen := map[string]bool{"auto": true, "notify": true, "off": true}
	if !validAutoOpen[cfg.LocalhostAutoOpen] {
		cfg.LocalhostAutoOpen = "notify"
	}

	validFontSizes := map[int]bool{8: true, 10: true, 12: true, 14: true, 16: true, 18: true, 20: true}
	if !validFontSizes[cfg.FontSize] {
		cfg.FontSize = 10
	}

	validScrollbackSizes := map[int]bool{1000: true, 2500: true, 5000: true, 10000: true, 25000: true, 50000: true, 100000: true}
	if !validScrollbackSizes[cfg.TerminalScrollback] {
		cfg.TerminalScrollback = 10000
	}

	if cfg.Favorites == nil {
		cfg.Favorites = make(map[string][]string)
	}

	ba := &cfg.BackgroundAgents
	if ba.ReviewEnabled == nil { ba.ReviewEnabled = boolPtr(false) }
	validTools := map[string]bool{"claude": true, "codex": true, "gemini": true}
	if !validTools[ba.ReviewTool] { ba.ReviewTool = "claude" }
	if ba.ReviewModel == "" { ba.ReviewModel = "claude-haiku-4-5-20251001" }
	if ba.ReviewPrompt == "" { ba.ReviewPrompt = "Review the following commit diff. Flag bugs, security issues, and code quality problems:\n\n{diff}" }
	if ba.TestEnabled == nil { ba.TestEnabled = boolPtr(false) }

	// Validate orchestrator settings
	if cfg.Orchestrator.MaxParallelAgents < 1 {
		cfg.Orchestrator.MaxParallelAgents = 1
	}
	if cfg.Orchestrator.MaxParallelAgents > 8 {
		cfg.Orchestrator.MaxParallelAgents = 8
	}
	if cfg.Orchestrator.MaxRetries < 0 {
		cfg.Orchestrator.MaxRetries = 0
	}
	if cfg.Orchestrator.MaxRetries > 5 {
		cfg.Orchestrator.MaxRetries = 5
	}

	// Validate language
	validLangs := map[string]bool{"de": true, "en": true, "it": true, "es": true, "fr": true}
	if !validLangs[cfg.Language] {
		cfg.Language = "de"
	}

	// Validate chat_style
	validChatStyles := map[string]bool{"claude-code": true, "telegram": true}
	if !validChatStyles[cfg.ChatStyle] {
		cfg.ChatStyle = "claude-code"
	}

	normalizeSTT(&cfg)

	if cfg.AutoNaming.Enabled == nil {
		cfg.AutoNaming.Enabled = boolPtr(true)
	}
	if cfg.AutoNaming.Model == "" {
		cfg.AutoNaming.Model = "claude-haiku-4-5"
	}

	if cfg.MCPServer.Enabled == nil {
		cfg.MCPServer.Enabled = boolPtr(true)
	}
	if cfg.MCPServer.Port <= 0 || cfg.MCPServer.Port > 65535 {
		cfg.MCPServer.Port = 51533
	}

	// Validate update_channel
	validUpdateChannels := map[string]bool{"stable": true, "alpha": true}
	if !validUpdateChannels[cfg.UpdateChannel] {
		cfg.UpdateChannel = "stable"
	}
	if cfg.AutoUpdateCheckMinutes < 0 {
		cfg.AutoUpdateCheckMinutes = 0
	}

	return cfg
}

// Save writes the given config to the YAML file.
func Save(cfg Config) error {
	p := configPath()
	if p == "" {
		return nil
	}
	return writeDefaults(p, cfg)
}

// writeDefaults persists the configuration to disk.
func writeDefaults(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	header := []byte("# Multiterminal UI configuration\n# Edit this file to customise defaults.\n\n")
	return os.WriteFile(path, append(header, data...), 0644)
}
