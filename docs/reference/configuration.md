# Configuration

Maven reads a single JSON file at `~/.maven/config.json` (mode `0600`). The schema is defined in `internal/kernel/config/config.go`. This page is the canonical field reference; for the example skeleton see [`config.example.json`](https://github.com/ageneralai/maven/blob/main/config.example.json) in the repository.

## Top-level

```json
{
  "agent":         { … },
  "provider":      { … },
  "channels":      { … },
  "tools":         { … },
  "skills":        { … },
  "mcp":           { … },
  "autoCompact":   { … },
  "memConsolidate":{ … },
  "shadowJournal": { … },
  "speech":        { … },
  "gateway":       { … }
}
```

## `agent`

Static settings for the agent runtime. Workspace cannot change across hot reload.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `workspace` | string | `~/.maven/workspace` | Directory Maven reads for `AGENTS.md`, `SOUL.md`, `HEARTBEAT.md`, `memory/`, `skills/`, channel state. Cannot be changed via hot reload. |
| `model` | string | `claude-sonnet-4-5-20250929` | Model identifier passed to the provider. |
| `maxTokens` | int | `8192` | Max output tokens per turn. |
| `maxToolIterations` | int | `20` | Max sequential tool-use iterations in one turn. |

## `provider`

LLM provider credentials. Validation requires `apiKey` non-empty; the gateway is meant to be run behind a vault like [OneCLI](../deployment/onecli.md) when you want real keys out of config.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `type` | string | `anthropic` | `anthropic` or `openai`. |
| `apiKey` | string | — | Required. Non-empty even if you front it with a credential-injecting proxy (use `"placeholder"`). |
| `baseUrl` | string | `""` | Override the provider's base URL (e.g. for a self-hosted gateway). |

## `channels`

| Sub-field | Type | Doc |
|-----------|------|-----|
| `telegram` | `TelegramConfig` | [Channel: Telegram](../channels/telegram.md) |
| `feishu` | `FeishuConfig` | [Channel: Feishu](../channels/feishu.md) |
| `wecom` | `WeComConfig` | [Channel: WeCom](../channels/wecom.md) |
| `whatsapp` | `WhatsAppConfig` | [Channel: WhatsApp](../channels/whatsapp.md) |
| `matrix` | `MatrixConfig` | [Channel: Matrix](../channels/matrix.md) |
| `web` | `WebConfig` | [Channel: Web UI](../channels/web.md) |

Each channel has an `enabled` boolean. Validation is per-channel and only runs when `enabled = true`.

### `channels.telegram`

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Master toggle. |
| `token` | — | Bot token. Must match `^\d+:[A-Za-z0-9_-]+$`. |
| `allowFrom` | `[]` | Numeric user IDs. Empty = all. |
| `proxy` | `""` | Per-channel proxy URL (http/https/socks5). |
| `rootDir` | `""` | Telegram assets directory. Empty = `<workspace>/.telegram`. |
| `feedback` | `"normal"` | `debug` / `normal` / `minimal` / `silent`. |
| `streaming` | `false` | Enable streaming output. |

### `channels.feishu`

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Master toggle. |
| `appId` | — | App ID. |
| `appSecret` | — | App secret. |
| `verificationToken` | `""` | Empty disables verification (dev only). |
| `encryptKey` | `""` | Event encryption key, if enabled in Feishu. |
| `port` | `9876` | Webhook HTTP port. |
| `allowFrom` | `[]` | Allowed `open_id` values. |
| `proxy` | `""` | Per-channel proxy URL. |

### `channels.wecom`

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Master toggle. |
| `token` | — | Callback signing token. |
| `encodingAESKey` | — | **Exactly 43 characters**. |
| `receiveId` | `""` | Optional strict receive-id check. |
| `port` | `9886` | Callback HTTP port. |
| `allowFrom` | `[]` | Allowed user IDs. |
| `proxy` | `""` | Per-channel proxy URL. |

### `channels.whatsapp`

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Master toggle. |
| `jid` | `""` | Default outbound JID. |
| `storePath` | `~/.maven/whatsapp-store.db` | SQLite session path. |
| `allowFrom` | `[]` | Allowed sender JIDs. |

### `channels.matrix`

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Master toggle. |
| `homeserver` | `"https://matrix.example.org"` | Full URL of homeserver. |
| `accessToken` | — | Long-lived access token. |
| `userId` | — | Bot MXID, starts with `@`. |
| `deviceId` | `""` | Auto-generated if empty. |
| `allowFrom` | `[]` | Allowed sender MXIDs. |
| `allowRooms` | `[]` | Allowed room IDs (`!…`). |

### `channels.web`

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Master toggle. |
| `allowFrom` | `[]` | Allowed internal client IDs. |
| `voice.enabled` | `false` | Enable `/ws/voice` voice transport. |

## `tools`

| Field | Default | Description |
|-------|---------|-------------|
| `execTimeout` | `60` | Reserved; tool execution timeout in seconds. |
| `restrictToWorkspace` | `true` | When true, sandbox-style tools and `DelegateTask`'s `cwd` / ACP FS hooks must stay under `agent.workspace`. |
| `task.enabled` | `false` | Enable the in-process `Task` tool ([Guides: Subagents](../guides/subagents.md)). |
| `acp.enabled` | `false` | Enable `DelegateTask` ([Guides: ACP delegation](../guides/acp-delegation.md)). |
| `acp.agents` | `{}` | Map of agent name → `{ command, args[], env[] }`. |

### `tools.acp.agents[name]`

| Field | Required | Description |
|-------|----------|-------------|
| `command` | yes | Executable on PATH. |
| `args` | no | Fixed arguments. |
| `env` | no | Extra `KEY=value` entries. |

## `skills`

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `true` | Master toggle. |
| `dir` | `""` | Custom skills directory. Empty = `<workspace>/skills`. |

See [Guides: Skills](../guides/skills.md).

## `mcp`

Reserved for MCP server URLs. Defaults to no servers.

| Field | Default | Description |
|-------|---------|-------------|
| `servers` | `[]` | URLs of MCP servers (currently passthrough to `agentsdk` `Options.MCPServers`). |

## `autoCompact`

Context rotation when the model nears its window. Off by default.

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Master toggle. |
| `threshold` | `0.8` | Trigger when usage crosses this fraction of the window. Must be `(0, 1]` when enabled. |
| `preserveCount` | `5` | Recent turns kept across a compact boundary. Must be `>= 0`. |

Validation only runs when `enabled = true`.

## `memConsolidate`

Background memory consolidation pass. See [Guides: Memory](../guides/memory.md).

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Master toggle. |
| `intervalHours` | `24` | Wall-clock interval between passes. |

## `shadowJournal`

Post-turn shadow journal pass. See [Guides: Memory — Shadow journaler](../guides/memory.md#shadow-journaler).

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Master toggle. When false, no post-turn shadow pass runs. |
| `model` | `""` | Override model for shadow turns. Empty inherits `agent.model`. Provider type, key, and base URL come from `provider.*`. |

## `speech`

Platform STT/TTS provider selection plus per-provider knobs. See [Guides: Voice](../guides/voice.md). For CLI `--voice`, `echoCancel` (default `pulse`) loads PulseAudio `module-echo-cancel` under internal device names and tears it down on exit — on Termux `webrtc` is unavailable but the `speex` fallback loads and runs; `off` skips the module and runs `capture`/`playback` verbatim (raw passthrough, e.g. headphones).

| Field | Default | Description |
|-------|---------|-------------|
| `sttProvider` | `deepgram` | Currently only `deepgram` is implemented. |
| `ttsProvider` | `openai` | `openai` / `deepgram` / `elevenlabs` / `cartesia`. |
| `echoCancel` | `pulse` | `pulse` loads `module-echo-cancel` (tries `webrtc` then `speex`) and routes through Maven's AEC devices; on Termux `webrtc` is unavailable but `speex` works. `off` disables echo cancellation entirely (raw passthrough, e.g. headphones); `capture`/`playback` run as configured with no forced `--device`. |
| `capture.command` | `parec` | Mic capture process for the CLI voice REPL. Must emit raw PCM s16le 16 kHz mono on stdout. With `echoCancel: pulse`, Maven appends the echo-cancel `--device`. |
| `capture.args` | `["--format=s16le","--rate=16000","--channels=1","--latency-msec=50"]` | Arguments for the capture command. Low latency keeps mic fragments small so VAD detects speech onset promptly (fast barge-in). |
| `playback.command` | `pacat` | Speaker playback process. Must accept raw PCM s16le 24 kHz mono on stdin. With `echoCancel: pulse`, Maven appends the echo-cancel `--device`. |
| `playback.args` | `["--format=s16le","--rate=24000","--channels=1","--latency-msec=100"]` | Arguments for the playback command. Low latency bounds the daemon buffer so barge-in cuts the speaker near-instantly. |
| `cartesia.voiceId` | `""` | Required when `cartesia` is selected. Or use `CARTESIA_VOICE_ID` env. |
| `cartesia.modelId` | `"sonic-2"` | TTS model. |
| `cartesia.apiVersion` | `"2025-04-16"` | API version header. |
| `cartesia.proxy` | `""` | Per-provider proxy URL. |
| `elevenlabs.voiceId` | `""` | Required when `elevenlabs` is selected. Or use `ELEVENLABS_VOICE_ID`. |
| `elevenlabs.proxy` | `""` | Per-provider proxy URL. |
| `deepgram.proxy` | `""` | Per-provider proxy URL. |
| `openai.proxy` | `""` | Per-provider proxy URL. |
| `wake.phrase` | `""` | CLI `--voice` only. When set, voice turns require this spoken wake phrase, which opens a conversation window. Empty = always listen. The `--wake-phrase` flag overrides this. |
| `wake.timeoutMs` | `8000` | Idle timeout before the wake conversation window re-arms to dormant, measured from when Maven finishes replying. Paused while a turn is in flight. |

Credentials come from environment variables; see [Reference: Environment](environment.md).

## `logging`

| Field | Default | Description |
|-------|---------|-------------|
| `level` | `info` | Process log verbosity: `debug`, `info`, `warn`, `error`. Hot reload applies changes when `gateway.hotReload` is enabled. Or set `MAVEN_LOG_LEVEL` at process start. |

## `gateway`

| Field | Default | Description |
|-------|---------|-------------|
| `host` | `0.0.0.0` | HTTP bind. Required non-empty. |
| `port` | `18790` | HTTP bind. `1..65535`. |
| `hotReload` | `false` | Watch `~/.maven/config.json` and re-`Apply` on change. |
| `reloadDebounceMs` | `800` (when omitted or `0`) | Debounce after fs events before reload fires. |
| `cron.maxConcurrentRuns` | `1` (when omitted or `0`) | Concurrent cron turns. Applied at start; restart required to change. |

## Validation summary

Aggregated errors surface from `Config.Validate()`:

- `provider.apiKey is required`
- `agent.workspace is required` / `agent.maxTokens must be positive` / `agent.maxToolIterations must be at least 1`
- `gateway.host is required` / `gateway.port must be 1..65535, got N` / `gateway.reloadDebounceMs must be non-negative` / `gateway.cron.maxConcurrentRuns must be >= 0`
- `channels.<name>.…` field-specific messages
- `autoCompact.threshold must be in (0,1] when autoCompact.enabled`
- `autoCompact.preserveCount must be non-negative`
- invalid `logging.level` (must be `debug`, `info`, `warn`, or `error`)

Errors join with `errors.Join` — a single failed `Validate()` returns every problem at once.

## File permissions

`SaveConfig` writes with mode `0600`. Don't loosen those bits — the file may carry API keys, bot tokens, and encryption keys. Use a credential vault ([OneCLI](../deployment/onecli.md)) or environment-only secrets when in doubt.
