# Plugins

Maven is a plugin host. All integrations are plugins. The kernel (`internal/kernel/`) never imports a plugin. `internal/gateway/wire.go` is the only file that assembles them.

## Base interface

Every plugin implements the minimal lifecycle (`internal/kernel/plugin/plugin.go`):

```go
type Plugin interface {
    Name() string
    Start(ctx context.Context) error
    Stop() error
}
```

`Start` and `Stop` are called on every registered plugin in registration order, regardless of which axes it contributes.

## Axis interfaces

A plugin contributes to one or more **axes** by implementing additional interfaces. None are required; nil contributions are fine.

| Interface | Method | Contributes |
|-----------|--------|-------------|
| `ChannelPlugin` | `Channels(cfg) []channels.Channel` | Chat transports. |
| `ToolPlugin` | `Tools(cfg) []tool.Tool` | Agent tools registered with the runtime. |
| `SkillPlugin` | `Skills(cfg) []api.SkillRegistration` | Prompt-time context injection. |
| `TTSPlugin` | `TTSProvider(cfg) voice.TTS` | Text-to-speech provider. |
| `STTPlugin` | `STTProvider(cfg) voice.STT` | Speech-to-text provider. |
| `SlashPlugin` | `SlashCommands(cfg) []SlashCommand` | Pre-model `/commands`. |
| `TriggerPlugin` | `Triggers(cfg) []Trigger` | Background execution. |
| `MemoryPlugin` | `Read(ctx, cfg, q) ([]MemoryEntry, error)` plus `Primary()` semantics | Long-term memory. |

## Registry aggregation

`plugin.NewRegistry(plugins...)` fixes registration order. At `Apply` time the gateway calls each axis method on plugins that implement it:

| Axis | Aggregation |
|------|-------------|
| Channels, Tools, Skills | Concatenated in registration order. |
| TTS, STT | First non-nil result wins. |
| SlashCommands | Concatenated; duplicate names error during registry build. |
| Triggers | Each trigger's `Start(ctx, TurnExecutor, OutboundPublisher)` is called; all execution flows through the same pipeline the chat path uses. |
| Memory | `Read` fans out concurrently with a 500 ms budget; `Write` routes to the single plugin where `Primary() == true`. Exactly one primary required. |

## Trigger contract

```go
type Trigger interface {
    Name() string
    Start(ctx context.Context, exec executor.TurnExecutor, pub OutboundPublisher) error
    Stop() error
}
```

Triggers receive the pipeline as a `TurnExecutor`. Cron also receives the outbound publisher so it can deliver job output to channels.

## Outbound publisher

```go
type OutboundPublisher interface {
    PublishOutbound(ctx context.Context, channel, chatID, content string) error
}
```

Narrow surface over `bus.MessageBus.PublishOutbound`. Triggers never see the bus directly.

## Registered plugins

All plugins are wired in `internal/gateway/wire.go`:

| Package | Axes |
|---------|------|
| `plugins/channel/telegram` | Channel |
| `plugins/channel/feishu` | Channel |
| `plugins/channel/wecom` | Channel |
| `plugins/channel/whatsapp` | Channel |
| `plugins/channel/matrix` | Channel |
| `plugins/channel/web` | Channel |
| `plugins/trigger/cron` | Trigger + Tool + Slash |
| `plugins/trigger/heartbeat` | Trigger |
| `plugins/trigger/memconsolidate` | Trigger |
| `plugins/skill/file` | Skill |
| `plugins/voice/deepgram` | STT + TTS |
| `plugins/voice/openai` | TTS |
| `plugins/voice/elevenlabs` | TTS |
| `plugins/voice/cartesia` | TTS |
| `plugins/tool/acp` | Tool |
| `plugins/memory/file` | Memory (primary) + Tool (`remember`, `memory_search`, `memory_get`) |

## Adding a plugin

1. Create `internal/plugins/<axis>/<name>/`.
2. Define a type implementing `plugin.Plugin` plus the axis interfaces you need.
3. Export `NewPlugin(...)` returning the concrete type (or a `plugin.AxisPlugin` interface).
4. Add one line to `internal/gateway/wire.go` inside `plugin.NewRegistry(...)`.

Zero changes to any kernel package.

**Example: a Discord channel plugin**

```go
// internal/plugins/channel/discord/plugin.go
package discord

type Plugin struct{ /* … */ }

func NewPlugin(b *bus.MessageBus, lg *slog.Logger) plugin.ChannelPlugin { /* … */ }

func (p *Plugin) Name() string                                  { return "discord" }
func (p *Plugin) Start(context.Context) error                   { return nil }
func (p *Plugin) Stop() error                                   { return nil }
func (p *Plugin) Channels(cfg *config.Config) []channels.Channel { /* … */ }
```

Then in `wire.go`:

```go
plugs := []plugin.Plugin{
    // …existing entries…
    discord.NewPlugin(core.bus, core.logger),
}
```

That is the entire change required.

## Kernel wall enforcement

`internal/kernel/` must never import `internal/plugins/`. Enforced by:

- Architectural rule: plugins depend on kernel, never the reverse.
- `depguard` linter rule `kernel_no_plugins` in `.golangci.yml`:

```yaml
depguard:
  rules:
    kernel_no_plugins:
      files:
        - "**/internal/kernel/**"
      deny:
        - pkg: "github.com/ageneralai/maven/internal/plugins/"
          desc: "kernel must not import plugins"
```

Only `internal/gateway/wire.go` (and tests) cross the wall.
