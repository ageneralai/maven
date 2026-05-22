# Gateway plugins

Maven’s gateway loads optional integrations through **`pkg/plugin`**: a single **`Registry`** built at gateway startup, shared with the runtime (tools) and with channels that need speech providers (Web UI voice).

## Contract (`pkg/plugin`)

**`Plugin`** implementations supply zero or more capabilities:

| Method | Role |
|--------|------|
| **`Name()`** | Stable id for logs and start/stop errors. |
| **`Enabled(cfg)`** | If false, this plugin is skipped for aggregation on this config. |
| **`Tools(cfg)`** | Custom agent tools; merged when enabled. |
| **`Channels(cfg)`** | Extra **`channel.Channel`** instances; merged when enabled (non-nil slices only). |
| **`TTSProvider(cfg)`** / **`STTProvider(cfg)`** | Speech implementations (**`pkg/voice`** interfaces); see resolution below. |
| **`Start` / `Stop`** | Lifecycle hooks; run for **every** plugin in registration order (not gated by **`Enabled`**). |

**LLM / `ModelFactory`** is **not** part of **`Plugin`**; model wiring stays native in **`NewSDKRuntime`** and config.

## Registry aggregation

Registration order is fixed at **`plugin.NewRegistry(...)`** time.

- **`Tools(cfg)`** — For each plugin with **`Enabled(cfg)`**, append **`Tools(cfg)...`** (same pattern as a flat tool list).
- **`Channels(cfg)`** — For each enabled plugin, if **`Channels(cfg)`** is non-nil, append its elements. Nil slices are skipped.
- **`TTSProvider(cfg)`** / **`STTProvider(cfg)`** — For each enabled plugin, the first **non-nil** provider wins; remaining plugins are ignored for that resolution.

**Nil registry or nil `cfg`** — **`Tools`**, **`Channels`**, **`TTSProvider`**, **`STTProvider`** return nil.

**`Start` / `Stop`** — Fail-fast: first error stops the sequence and is returned (gateway does not come up partially).

## Composition today

**`internal/gateway/gateway.go`** builds:

1. **`acp.NewPlugin()`** — ACP **`DelegateTask`** when **`tools.acp`** yields tools (`pkg/acp`, including **`NewPlugin`**).
2. **`internal/voice.VoicePlugins()`** — Cartesia, Deepgram, ElevenLabs, OpenAI speech plugins (`pkg/cartesia`, `pkg/deepgram`, `pkg/elevenlabs`, `pkg/openai`).

The same **`Registry`** is passed to **`internal/channel/manager.NewChannelManager`** so Web UI voice can call **`internal/voice.NewSTT` / `NewTTS`** with that registry ( **`nil`** falls back to **`internal/voice.DefaultVoiceRegistry()`** in tests).

Runtime **`Apply`** pulls **`g.plugins.Tools(cfg)`** into the agent SDK runtime factory alongside cron and skills.

## Adding a plugin

1. Implement **`plugin.Plugin`** (often a zero-sized **`type Plugin struct{}`** plus **`NewPlugin() plugin.Plugin`**).
2. Return **nil** for surfaces you do not implement (e.g. voice-only plugins return nil **`Tools`**, **`Channels`**, and the unused speech side).
3. Use **`Enabled`** to avoid work when your integration is off (ACP gates on config; voice vendor plugins gate on **`channels.web.voice.enabled`**).
4. Register the constructor in **`internal/gateway/gateway.go`** (or a small helper next to it) so production and tests stay aligned.

For speech providers, prefer **`pkg/<vendor>/plugin.go`** next to the HTTP/WebSocket implementation, and add **`NewPlugin()`** to **`internal/voice.VoicePlugins()`** so one list stays the single composition point for voice.

## Related paths

- **`pkg/plugin/plugin.go`** — **`Plugin`** interface
- **`pkg/plugin/registry.go`** — **`Registry`** aggregation and lifecycle
- **`pkg/plugin/registry_test.go`** — aggregation tests
- **`internal/gateway/gateway.go`** — default registry construction
- **`internal/voice/factory.go`** — **`NewSTT` / `NewTTS`** + fallback errors when plugins return nil
- **`pkg/voice/keys.go`** — **`MergeKeys`** for provider credentials (shared with plugins)
