package config

import "time"

// Config is the root configuration for Herald.
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Auth          AuthConfig          `yaml:"auth"`
	Database      DatabaseConfig      `yaml:"database"`
	Execution     ExecutionConfig     `yaml:"execution"`
	Notifications NotificationsConfig `yaml:"notifications"`
	Projects      map[string]Project  `yaml:"projects"`
	RateLimit     RateLimitConfig     `yaml:"rate_limit"`
	Dashboard     DashboardConfig     `yaml:"dashboard"`
}

type ServerConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	PublicURL string `yaml:"public_url"`
	LogLevel  string `yaml:"log_level"`
	LogFile   string `yaml:"log_file"`
}

type AuthConfig struct {
	ClientID        string        `yaml:"client_id"`
	ClientSecret    string        `yaml:"client_secret"`
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
	RedirectURIs    []string      `yaml:"redirect_uris"`
}

type DatabaseConfig struct {
	Path          string `yaml:"path"`
	RetentionDays int    `yaml:"retention_days"`
}

type ExecutionConfig struct {
	ClaudePath     string            `yaml:"claude_path"`
	DefaultTimeout time.Duration     `yaml:"default_timeout"`
	MaxTimeout     time.Duration     `yaml:"max_timeout"`
	WorkDir        string            `yaml:"work_dir"`
	MaxConcurrent  int               `yaml:"max_concurrent"`
	MaxPromptSize  int               `yaml:"max_prompt_size"`
	MaxOutputSize  int               `yaml:"max_output_size"`
	Env            map[string]string `yaml:"env"`
}

type NotificationsConfig struct {
	// MCP server notifications are always enabled (push via SSE).
	// No external notification backends (ntfy, webhooks) — Herald uses
	// the MCP notification protocol to push updates to Claude Chat directly.
}

type Project struct {
	Path               string    `yaml:"path"`
	Description        string    `yaml:"description"`
	Default            bool      `yaml:"default"`
	AllowedTools       []string  `yaml:"allowed_tools"`
	MaxConcurrentTasks int       `yaml:"max_concurrent_tasks"`
	Git                GitConfig `yaml:"git"`
}

type GitConfig struct {
	AutoBranch   bool   `yaml:"auto_branch"`
	AutoStash    bool   `yaml:"auto_stash"`
	AutoCommit   bool   `yaml:"auto_commit"`
	BranchPrefix string `yaml:"branch_prefix"`
}

type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
	Burst             int `yaml:"burst"`
}

type DashboardConfig struct {
	Enabled bool `yaml:"enabled"`
}

// Defaults returns a Config with sensible default values.
func Defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Host:     "127.0.0.1",
			Port:     8420,
			LogLevel: "info",
		},
		Auth: AuthConfig{
			ClientID:        "herald-claude-chat",
			AccessTokenTTL:  1 * time.Hour,
			RefreshTokenTTL: 30 * 24 * time.Hour,
		},
		Database: DatabaseConfig{
			Path:          "~/.config/herald/herald.db",
			RetentionDays: 90,
		},
		Execution: ExecutionConfig{
			ClaudePath:     "claude",
			DefaultTimeout: 30 * time.Minute,
			MaxTimeout:     2 * time.Hour,
			WorkDir:        "~/.config/herald/work",
			MaxConcurrent:  3,
			MaxPromptSize:  102400,  // 100KB
			MaxOutputSize:  1048576, // 1MB
			Env: map[string]string{
				"CLAUDE_CODE_ENTRYPOINT":          "herald",
				"CLAUDE_CODE_DISABLE_AUTO_UPDATE": "1",
			},
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: 200,
			Burst:             100,
		},
		Dashboard: DashboardConfig{
			Enabled: true,
		},
	}
}
