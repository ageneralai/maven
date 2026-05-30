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
	"time"
)

var telegramTokenPattern = regexp.MustCompile(`^\d+:[A-Za-z0-9_-]+$`)

const (
	DefaultModel             = "claude-sonnet-4-5-20250929"
	DefaultMaxTokens         = 8192
	DefaultMaxToolIterations = 20
	DefaultExecTimeout       = 60
	DefaultHost              = "0.0.0.0"
	DefaultPort              = 18790
	DefaultBufSize           = 100
)

type Config struct {
	Agent          AgentConfig          `json:"agent"`
	Channels       ChannelsConfig       `json:"channels"`
	Provider       ProviderConfig       `json:"provider"`
	Tools          ToolsConfig          `json:"tools"`
	Skills         SkillsConfig         `json:"skills"`
	MCP            MCPConfig            `json:"mcp"`
	AutoCompact    AutoCompactConfig    `json:"autoCompact"`
	MemConsolidate MemConsolidateConfig `json:"memConsolidate,omitempty"`
	ShadowJournal  ShadowJournalConfig  `json:"shadowJournal,omitempty"`
	Gateway        GatewayConfig        `json:"gateway"`
	Speech         SpeechConfig         `json:"speech,omitempty"`
	Logging        LoggingConfig        `json:"logging,omitempty"`
}

type LoggingConfig struct {
	Level string `json:"level,omitempty"` // debug, info, warn, error (default: info)
}

type AgentConfig struct {
	Workspace         string `json:"workspace"`
	Model             string `json:"model"`
	MaxTokens         int    `json:"maxTokens"`
	MaxToolIterations int    `json:"maxToolIterations"`
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
	ExecTimeout         int            `json:"execTimeout"`
	RestrictToWorkspace bool           `json:"restrictToWorkspace"`
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
	Enabled     bool     `json:"enabled"`
	Homeserver  string   `json:"homeserver"`
	AccessToken string   `json:"accessToken"`
	UserID      string   `json:"userId"`
	DeviceID    string   `json:"deviceId,omitempty"`
	AllowFrom   []string `json:"allowFrom"`
	AllowRooms  []string `json:"allowRooms"`
}

type WebConfig struct {
	Enabled   bool           `json:"enabled"`
	AllowFrom []string       `json:"allowFrom,omitempty"`
	Voice     WebVoiceConfig `json:"voice"`
}

// SpeechConfig selects platform STT/TTS providers.
// sttProvider: deepgram (default). ttsProvider: openai (default) | deepgram | elevenlabs | cartesia.
type SpeechConfig struct {
	STTProvider string             `json:"sttProvider,omitempty"`
	TTSProvider string             `json:"ttsProvider,omitempty"`
	EchoCancel  string             `json:"echoCancel,omitempty"`
	Capture     SpeechExecConfig   `json:"capture,omitempty"`
	Playback    SpeechExecConfig   `json:"playback,omitempty"`
	Cartesia    CartesiaConfig     `json:"cartesia,omitempty"`
	ElevenLabs  ElevenLabsConfig   `json:"elevenlabs,omitempty"`
	Deepgram    DeepgramConfig     `json:"deepgram,omitempty"`
	OpenAI      OpenAISpeechConfig `json:"openai,omitempty"`
	Wake        WakeConfig         `json:"wake,omitempty"`
}

// EchoCancelDisabled reports whether CLI voice should skip PulseAudio
// module-echo-cancel and run capture/playback as configured (Android).
func (s SpeechConfig) EchoCancelDisabled() bool {
	return strings.EqualFold(strings.TrimSpace(s.EchoCancel), "off")
}

// Validate checks the echoCancel mode. Empty/"pulse" load module-echo-cancel;
// "off" runs configured I/O directly.
func (s SpeechConfig) Validate() error {
	switch strings.ToLower(strings.TrimSpace(s.EchoCancel)) {
	case "", "pulse", "off":
	default:
		return fmt.Errorf("speech.echoCancel must be \"pulse\" or \"off\", got %q", s.EchoCancel)
	}
	if s.Wake.TimeoutMs < 0 {
		return fmt.Errorf("speech.wake.timeoutMs must be non-negative, got %d", s.Wake.TimeoutMs)
	}
	return nil
}

// SpeechExecConfig runs an external process for raw PCM I/O (parec/pacat on PulseAudio).
type SpeechExecConfig struct {
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// CaptureCommand returns the mic capture command (default parec s16le 16 kHz mono).
// Low --latency-msec keeps mic frames small so VAD detects speech onset promptly (fast barge-in).
func (s SpeechConfig) CaptureCommand() (string, []string) {
	if cmd := strings.TrimSpace(s.Capture.Command); cmd != "" {
		return cmd, append([]string(nil), s.Capture.Args...)
	}
	return "parec", []string{"--format=s16le", "--rate=16000", "--channels=1", "--latency-msec=50"}
}

// PlaybackCommand returns the speaker playback command (default pacat s16le 24 kHz mono).
// Low --latency-msec bounds the daemon buffer so killing playback on barge-in cuts near-instantly.
func (s SpeechConfig) PlaybackCommand() (string, []string) {
	if cmd := strings.TrimSpace(s.Playback.Command); cmd != "" {
		return cmd, append([]string(nil), s.Playback.Args...)
	}
	return "pacat", []string{"--format=s16le", "--rate=24000", "--channels=1", "--latency-msec=100"}
}

// DefaultWakeTimeoutMs is the idle window (ms) before a wake conversation re-arms.
const DefaultWakeTimeoutMs = 8000

// WakeConfig gates CLI voice turns behind a spoken wake phrase (empty = always listen).
type WakeConfig struct {
	Phrase    string `json:"phrase,omitempty"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
}

// WakeWindow is the idle timeout before a wake conversation window re-arms to dormant.
func (s SpeechConfig) WakeWindow() time.Duration {
	if s.Wake.TimeoutMs <= 0 {
		return DefaultWakeTimeoutMs * time.Millisecond
	}
	return time.Duration(s.Wake.TimeoutMs) * time.Millisecond
}

type CartesiaConfig struct {
	APIKey     string `json:"apiKey,omitempty"`
	VoiceID    string `json:"voiceId,omitempty"`
	ModelID    string `json:"modelId,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
	Proxy      string `json:"proxy,omitempty"`
}

type ElevenLabsConfig struct {
	APIKey  string `json:"apiKey,omitempty"`
	VoiceID string `json:"voiceId,omitempty"`
	Proxy   string `json:"proxy,omitempty"`
}

type DeepgramConfig struct {
	APIKey string `json:"apiKey,omitempty"`
	Proxy  string `json:"proxy,omitempty"`
}

type OpenAISpeechConfig struct {
	APIKey string `json:"apiKey,omitempty"`
	Proxy  string `json:"proxy,omitempty"`
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

// MemConsolidateConfig controls the background memory consolidation pass.
type MemConsolidateConfig struct {
	Enabled       bool `json:"enabled"`
	IntervalHours int  `json:"intervalHours,omitempty"` // default 24 when enabled
}

// ShadowJournalConfig controls the post-turn shadow journal pass.
type ShadowJournalConfig struct {
	Enabled bool   `json:"enabled"`
	Model   string `json:"model,omitempty"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Agent: AgentConfig{
			Workspace:         filepath.Join(home, ".maven", "workspace"),
			Model:             DefaultModel,
			MaxTokens:         DefaultMaxTokens,
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
		MemConsolidate: MemConsolidateConfig{
			Enabled:       false,
			IntervalHours: 24,
		},
		ShadowJournal: ShadowJournalConfig{
			Enabled: false,
			Model:   "",
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
		c.Logging.Validate(),
		c.Speech.Validate(),
	)
}

func (c LoggingConfig) Validate() error {
	if strings.TrimSpace(c.Level) == "" {
		return nil
	}
	_, err := ParseLogLevel(c.Level)
	return err
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
	if p := strings.TrimSpace(os.Getenv("MAVEN_CONFIG")); p != "" {
		return p
	}
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
	applyEnv(cfg)
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
