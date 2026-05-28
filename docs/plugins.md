# Plugins

Maven is a plugin host. All integrations are plugins. The kernel (`internal/kernel/`) never imports a plugin. `internal/gateway/wire.go` is the only file that assembles them.

## Axis interfaces

Every plugin implements the base `Plugin` interface (`internal/kernel/plugin/plugin.go`):

```go
type Plugin interface {
    Name() string
    Start(ctx context.Context) error
    Stop() error
}
```

A plugin contributes to one or more **axes** by implementing additional interfaces:

| Interface | Method | Contributes |
|-----------|--------|-------------|
| `ChannelPlugin` | `Channels(cfg) []channel.Channel` | Chat transports |
| `ToolPlugin` | `Tools(cfg) []tool.Tool` | Agent tools (registered with the runtime) |
| `SkillPlugin` | `Skills(cfg) []api.SkillRegistration` | Prompt-time context injection |
| `TTSPlugin` | `TTSProvider(cfg) voice.TTS` | Text-to-speech provider |
| `STTPlugin` | `STTProvider(cfg) voice.STT` | Speech-to-text provider |
| `SlashPlugin` | `SlashCommands(cfg) []SlashCommand` | Pre-model `/commands` |
| `TriggerPlugin` | `Triggers(cfg) []Trigger` | Background execution (cron, heartbeat) |

A plugin implements only the axes it provides — no nil stubs required for the rest.

## Registry aggregation

`plugin.NewRegistry(plugins...)` fixes registration order. At `Apply` time the gateway calls each axis method on all plugins that implement it:

- `Channels`, `Tools`, `Skills` — results are concatenated in registration order.
- `TTSProvider`, `STTProvider` — first non-nil result wins.
- `SlashCommands` — results are concatenated; duplicates return an error.
- `Triggers` — each trigger's `Start(ctx, TurnExecutor, OutboundPublisher)` is called; all execution flows through the same pipeline the chat path uses.
- `Memory` — `Read` fans out to all plugins concurrently (500ms budget); `Write` routes to the single plugin where `Primary() == true`. Exactly one primary must be registered.

`Start` and `Stop` are called on every plugin in registration order regardless of which axes it implements.

## Registered plugins

All plugins are listed in `internal/gateway/wire.go`:

| Package | Axes |
|---------|------|
| `internal/plugins/channel/telegram` | Channel |
| `internal/plugins/channel/feishu` | Channel |
| `internal/plugins/channel/wecom` | Channel |
| `internal/plugins/channel/whatsapp` | Channel |
| `internal/plugins/channel/matrix` | Channel |
| `internal/plugins/channel/web` | Channel |
| `internal/plugins/trigger/cron` | Trigger + Tool + Slash |
| `internal/plugins/trigger/heartbeat` | Trigger |
| `internal/plugins/skill/file` | Skill |
| `internal/plugins/voice/deepgram` | STT + TTS |
| `internal/plugins/voice/openai` | TTS |
| `internal/plugins/voice/elevenlabs` | TTS |
| `internal/plugins/voice/cartesia` | TTS |
| `internal/plugins/tool/acp` | Tool |
| `internal/plugins/memory/file` | Memory (primary) + Tool (`remember`) |

## Adding a plugin

1. Create `internal/plugins/<axis>/<name>/`.
2. Define a type implementing `plugin.Plugin` plus the axis interfaces you need.
3. Export `NewPlugin(...)` returning the concrete type (or a `plugin.AxisPlugin` interface).
4. Add one line to `internal/gateway/wire.go` inside `plugin.NewRegistry(...)`.

Zero changes to any kernel package.

**Example: add a Discord channel plugin**

```go
// internal/plugins/channel/discord/plugin.go
package discord

type Plugin struct{ ... }

func NewPlugin(b *bus.MessageBus, lg *slog.Logger) plugin.ChannelPlugin { ... }
func (p *Plugin) Name() string                                          { return "discord" }
func (p *Plugin) Start(context.Context) error                           { return nil }
func (p *Plugin) Stop() error                                           { return nil }
func (p *Plugin) Channels(cfg *config.Config) []channel.Channel         { ... }
```

Then in `wire.go`:
```go
discord.NewPlugin(core.bus, core.logger),
```

That is the entire change required.

## Kernel wall

`internal/kernel/` must never import `internal/plugins/`. Enforced by `depguard` (`kernel_no_plugins` rule in `.golangci.yml`). Only `internal/gateway/wire.go` crosses the wall.
