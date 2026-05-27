package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var telegramTokenPattern = regexp.MustCompile(`^\d+:[A-Za-z0-9_-]+$`)

const (
	DefaultModel             = "claude-sonnet-4-5-20250929"
	DefaultMaxTokens         = 8192
	DefaultTemperature       = 0.7
	DefaultMaxToolIterations = 20
	DefaultExecTimeout       = 60
	DefaultHost              = "0.0.0.0"
	DefaultPort              = 18790
	DefaultBufSize           = 100
)

type Config struct {
	Agent         AgentConfig         `json:"agent"`
	Channels      ChannelsConfig      `json:"channels"`
	Provider      ProviderConfig      `json:"provider"`
	Tools         ToolsConfig         `json:"tools"`
	Skills        SkillsConfig        `json:"skills"`
	MCP           MCPConfig           `json:"mcp"`
	AutoCompact   AutoCompactConfig   `json:"autoCompact"`
	TokenTracking TokenTrackingConfig `json:"tokenTracking"`
	Gateway       GatewayConfig       `json:"gateway"`
	Speech        SpeechConfig        `json:"speech,omitempty"`
}

type AgentConfig struct {
	Workspace         string  `json:"workspace"`
	Model             string  `json:"model"`
	MaxTokens         int     `json:"maxTokens"`
	Temperature       float64 `json:"temperature"`
	MaxToolIterations int     `json:"maxToolIterations"`
}

type ProviderConfig struct {
	Type    string `json:"type,omitempty"` // "anthropic" (default) or "openai"
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl,omitempty"`
}

type ChannelsConfig struct {
	Telegram TelegramConfig `json:"telegram"`
	Feishu   FeishuConfig   `json:"feishu"`
	WeCom    WeComConfig    `json:"wecom"`
	WhatsApp WhatsAppConfig `json:"whatsapp"`
	Matrix   MatrixConfig   `json:"matrix"`
	Web      WebConfig      `json:"web"`
}

type TelegramConfig struct {
	Enabled   bool     `json:"enabled"`
	Token     string   `json:"token"`
	AllowFrom []string `json:"allowFrom"`
	Proxy     string   `json:"proxy,omitempty"`
	RootDir   string   `json:"rootDir,omitempty"`   // default: <agent.workspace>/.telegram
	Feedback  string   `json:"feedback,omitempty"`  // "debug", "normal" (default), "minimal", "silent"
	Streaming bool     `json:"streaming,omitempty"` // enable streaming output via message editing
}

type FeishuConfig struct {
	Enabled           bool     `json:"enabled"`
	AppID             string   `json:"appId"`
	AppSecret         string   `json:"appSecret"`
	VerificationToken string   `json:"verificationToken"`
	EncryptKey        string   `json:"encryptKey,omitempty"`
	Port              int      `json:"port,omitempty"`
	AllowFrom         []string `json:"allowFrom"`
	Proxy             string   `json:"proxy,omitempty"`
}

type WeComConfig struct {
	Enabled        bool     `json:"enabled"`
	Token          string   `json:"token"`
	EncodingAESKey string   `json:"encodingAESKey"`
	ReceiveID      string   `json:"receiveId,omitempty"`
	Port           int      `json:"port,omitempty"`
	AllowFrom      []string `json:"allowFrom"`
	Proxy          string   `json:"proxy,omitempty"`
}

type ToolsConfig struct {
	ExecTimeout         int           `json:"execTimeout"`
	RestrictToWorkspace bool          `json:"restrictToWorkspace"`
	ACP                 ACPToolConfig  `json:"acp,omitempty"`
	Task                TaskToolConfig `json:"task,omitempty"`
}

// ACPToolConfig registers subprocess ACP agents invoked only via the DelegateTask tool (commands never come from model params).
type ACPToolConfig struct {
	Enabled bool                `json:"enabled"`
	Agents  map[string]ACPAgent `json:"agents,omitempty"`
}

type ACPAgent struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
}

// TaskToolConfig enables the in-process Task tool for SDK subagent delegation.
type TaskToolConfig struct {
	Enabled bool `json:"enabled"`
}

type GatewayConfig struct {
	Host             string            `json:"host"`
	Port             int               `json:"port"`
	HotReload        bool              `json:"hotReload"`
	ReloadDebounceMs int               `json:"reloadDebounceMs,omitempty"`
	Cron             GatewayCronConfig `json:"cron,omitempty"`
}

// GatewayCronConfig controls gateway-executed cron scheduling (not job definitions).
type GatewayCronConfig struct {
	// MaxConcurrentRuns caps concurrent cron agent turns in process. Omitted or 0 means 1.
	// Applied at gateway start only; changing requires restart.
	MaxConcurrentRuns int `json:"maxConcurrentRuns,omitempty"`
}

type SkillsConfig struct {
	Enabled bool   `json:"enabled"`
	Dir     string `json:"dir,omitempty"` // 默认 workspace/skills
}

type MCPConfig struct {
	Servers []string `json:"servers,omitempty"`
}

type WhatsAppConfig struct {
	Enabled   bool     `json:"enabled"`
	JID       string   `json:"jid,omitempty"`
	StorePath string   `json:"storePath,omitempty"`
	AllowFrom []string `json:"allowFrom,omitempty"`
}

type MatrixConfig struct {
	Enabled      bool     `json:"enabled"`
	Homeserver   string   `json:"homeserver"`
	AccessToken  string   `json:"accessToken"`
	UserID       string   `json:"userId"`
	DeviceID     string   `json:"deviceId,omitempty"`
	AllowFrom    []string `json:"allowFrom"`
	AllowRooms   []string `json:"allowRooms"`
}

type WebConfig struct {
	Enabled   bool           `json:"enabled"`
	AllowFrom []string       `json:"allowFrom,omitempty"`
	Voice     WebVoiceConfig `json:"voice"`
}

// SpeechConfig selects platform STT/TTS providers (credentials via env; see kernel/voice.MergeKeys).
// sttProvider: deepgram (default). ttsProvider: openai (default) | deepgram | elevenlabs | cartesia.
type SpeechConfig struct {
	STTProvider string           `json:"sttProvider,omitempty"`
	TTSProvider string           `json:"ttsProvider,omitempty"`
	Cartesia    CartesiaConfig   `json:"cartesia,omitempty"`
	ElevenLabs  ElevenLabsConfig `json:"elevenlabs,omitempty"`
	Deepgram    DeepgramConfig   `json:"deepgram,omitempty"`
	OpenAI      OpenAISpeechConfig `json:"openai,omitempty"`
}

type CartesiaConfig struct {
	VoiceID    string `json:"voiceId,omitempty"`
	ModelID    string `json:"modelId,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
	Proxy      string `json:"proxy,omitempty"`
}

type ElevenLabsConfig struct {
	VoiceID string `json:"voiceId,omitempty"`
	Proxy   string `json:"proxy,omitempty"`
}

type DeepgramConfig struct {
	Proxy string `json:"proxy,omitempty"`
}

type OpenAISpeechConfig struct {
	Proxy string `json:"proxy,omitempty"`
}

// WebVoiceConfig enables browser realtime voice on the Web UI (/ws/voice).
type WebVoiceConfig struct {
	Enabled bool `json:"enabled"`
}

type AutoCompactConfig struct {
	Enabled       bool    `json:"enabled"`
	Threshold     float64 `json:"threshold,omitempty"`
	PreserveCount int     `json:"preserveCount,omitempty"`
}

type TokenTrackingConfig struct {
	Enabled bool `json:"enabled"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Agent: AgentConfig{
			Workspace:         filepath.Join(home, ".maven", "workspace"),
			Model:             DefaultModel,
			MaxTokens:         DefaultMaxTokens,
			Temperature:       DefaultTemperature,
			MaxToolIterations: DefaultMaxToolIterations,
		},
		Provider: ProviderConfig{},
		Channels: ChannelsConfig{},
		Tools: ToolsConfig{
			ExecTimeout:         DefaultExecTimeout,
			RestrictToWorkspace: true,
		},
		Skills: SkillsConfig{
			Enabled: true,
		},
		AutoCompact: AutoCompactConfig{
			Enabled:       false,
			Threshold:     0.8,
			PreserveCount: 5,
		},
		Gateway: GatewayConfig{
			Host: DefaultHost,
			Port: DefaultPort,
		},
	}
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config: nil")
	}
	return errors.Join(
		c.Provider.Validate(),
		c.Agent.Validate(),
		c.Gateway.Validate(),
		c.Channels.Validate(),
		c.AutoCompact.Validate(),
	)
}

func (c ProviderConfig) Validate() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return errors.New("provider.apiKey is required")
	}
	return nil
}

func (c AgentConfig) Validate() error {
	var errs []error
	if strings.TrimSpace(c.Workspace) == "" {
		errs = append(errs, errors.New("agent.workspace is required"))
	}
	if c.MaxTokens <= 0 {
		errs = append(errs, errors.New("agent.maxTokens must be positive"))
	}
	if c.MaxToolIterations < 1 {
		errs = append(errs, errors.New("agent.maxToolIterations must be at least 1"))
	}
	if c.Temperature < 0 || c.Temperature > 2 {
		errs = append(errs, errors.New("agent.temperature must be between 0 and 2"))
	}
	return errors.Join(errs...)
}

func (c GatewayConfig) Validate() error {
	var errs []error
	if strings.TrimSpace(c.Host) == "" {
		errs = append(errs, errors.New("gateway.host is required"))
	}
	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("gateway.port must be 1..65535, got %d", c.Port))
	}
	if c.ReloadDebounceMs < 0 {
		errs = append(errs, errors.New("gateway.reloadDebounceMs must be non-negative"))
	}
	if c.Cron.MaxConcurrentRuns < 0 {
		errs = append(errs, errors.New("gateway.cron.maxConcurrentRuns must be >= 0 (0 means default 1)"))
	}
	return errors.Join(errs...)
}

func (c ChannelsConfig) Validate() error {
	return errors.Join(
		c.Telegram.Validate(),
		c.Feishu.Validate(),
		c.WeCom.Validate(),
		c.Matrix.Validate(),
	)
}

func (c TelegramConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	token := strings.TrimSpace(c.Token)
	if token == "" {
		return errors.New("channels.telegram.token is required when telegram is enabled")
	}
	if !telegramTokenPattern.MatchString(token) {
		return errors.New("channels.telegram.token must match \\d+:[A-Za-z0-9_-]+ when telegram is enabled")
	}
	return nil
}

func (c FeishuConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.AppID) == "" {
		return errors.New("channels.feishu.appId is required when feishu is enabled")
	}
	if strings.TrimSpace(c.AppSecret) == "" {
		return errors.New("channels.feishu.appSecret is required when feishu is enabled")
	}
	return nil
}

func (c WeComConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.Token) == "" {
		return errors.New("channels.wecom.token is required when wecom is enabled")
	}
	key := strings.TrimSpace(c.EncodingAESKey)
	if key == "" {
		return errors.New("channels.wecom.encodingAESKey is required when wecom is enabled")
	}
	if len(key) != 43 {
		return fmt.Errorf("channels.wecom.encodingAESKey must be 43 characters when wecom is enabled, got %d", len(key))
	}
	return nil
}

func (c MatrixConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	homeserver := strings.TrimSpace(c.Homeserver)
	if homeserver == "" {
		return errors.New("channels.matrix.homeserver is required when matrix is enabled")
	}
	u, err := url.Parse(homeserver)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("channels.matrix.homeserver must be a valid URL when matrix is enabled")
	}
	if strings.TrimSpace(c.AccessToken) == "" {
		return errors.New("channels.matrix.accessToken is required when matrix is enabled")
	}
	userID := strings.TrimSpace(c.UserID)
	if userID == "" {
		return errors.New("channels.matrix.userId is required when matrix is enabled")
	}
	if !strings.HasPrefix(userID, "@") {
		return errors.New("channels.matrix.userId must start with @ when matrix is enabled")
	}
	return nil
}

func (c AutoCompactConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	var errs []error
	if c.Threshold <= 0 || c.Threshold > 1 {
		errs = append(errs, errors.New("autoCompact.threshold must be in (0,1] when autoCompact.enabled"))
	}
	if c.PreserveCount < 0 {
		errs = append(errs, errors.New("autoCompact.preserveCount must be non-negative"))
	}
	return errors.Join(errs...)
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".maven")
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.json")
}

func LoadConfig() (*Config, error) {
	return LoadConfigFromPath(ConfigPath())
}

// LoadConfigFromPath reads and merges JSON at path with the same defaults as LoadConfig.
func LoadConfigFromPath(path string) (*Config, error) {
	cfg := DefaultConfig()
	// #nosec G304 -- path is app-controlled/config path, not user-supplied arbitrary include
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}
	if cfg.Agent.Workspace == "" {
		cfg.Agent.Workspace = DefaultConfig().Agent.Workspace
	}
	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(ConfigPath(), data, 0600)
}
