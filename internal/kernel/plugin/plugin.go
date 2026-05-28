package plugin

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/channel"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/executor"
	"github.com/ageneralai/maven/internal/kernel/hook"
	"github.com/ageneralai/maven/internal/kernel/voice"
)

// Plugin is the minimum. Every plugin has identity and a lifecycle.
type Plugin interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
}

// ChannelPlugin contributes chat transports.
type ChannelPlugin interface {
	Plugin
	Channels(cfg *config.Config) []channels.Channel
}

// ToolPlugin contributes agent tools.
type ToolPlugin interface {
	Plugin
	Tools(cfg *config.Config) []tool.Tool
}

// SkillPlugin contributes prompt-time skills.
type SkillPlugin interface {
	Plugin
	Skills(cfg *config.Config) []api.SkillRegistration
}

// TTSPlugin contributes text-to-speech.
type TTSPlugin interface {
	Plugin
	TTSProvider(cfg *config.Config) voice.TTSProvider
}

// STTPlugin contributes speech-to-text.
type STTPlugin interface {
	Plugin
	STTProvider(cfg *config.Config) voice.STTProvider
}

// SlashCommand is one slash handler registered before model execution.
type SlashCommand struct {
	Definition SlashDefinition
	Handler    SlashHandler
}

// SlashDefinition names a /slash command.
type SlashDefinition struct {
	Name        string
	Description string
}

// SlashHandler runs when a registered slash command matches.
type SlashHandler interface {
	Handle(ctx context.Context, inv SlashInvocation) (SlashResult, error)
}

// SlashInvocation is one parsed /command from user text.
type SlashInvocation struct {
	Name     string
	Args     []string
	Flags    map[string]string
	Raw      string
	Position int
}

// SlashResult is a handler outcome.
type SlashResult struct {
	Command    string
	Output     string
	Metadata   map[string]any
	PostAction string
}

// SlashPlugin contributes slash commands.
type SlashPlugin interface {
	Plugin
	SlashCommands(cfg *config.Config) []SlashCommand
}

// OutboundPublisher enqueues outbound messages (narrow bus surface for triggers).
type OutboundPublisher interface {
	PublishOutbound(ctx context.Context, channel, chatID, content string) error
}

// Trigger fires turns without a user message (cron, heartbeat, webhooks).
type Trigger interface {
	Name() string
	Start(ctx context.Context, exec executor.TurnExecutor, pub OutboundPublisher) error
	Stop() error
}

// TriggerPlugin contributes background triggers.
type TriggerPlugin interface {
	Plugin
	Triggers(cfg *config.Config) []Trigger
}

// PostTurnPlugin optionally contributes a handler called by the pipeline after each real user
// conversation turn. The pipeline fires it in a goroutine; returning nil disables the hook.
type PostTurnPlugin interface {
	Plugin
	PostTurnHandler(cfg *config.Config) hook.PostTurnHandler
}
