# Maven Architecture

Maven is a single-process Go application: CLI agent and persistent gateway for personal AI assistance. Chat transports, scheduled jobs, and health checks share one execution model built on `ageneral-agents-go`.

## Design constraints

1. **Single execution surface** — Chat, cron, and heartbeat all flow through the same pipeline and agent runtime.
2. **Single mutation path** — `Gateway.Apply` is the only way to change active system state.
3. **Kernel wall** — Core logic never imports plugins; composition happens in `gateway/wire.go`.

## Topology

```mermaid
flowchart LR
    subgraph External
        Users[Users via chat apps]
        LLM[LLM APIs]
    end
    subgraph "Maven Process"
        GW[Gateway]
        PLG[Plugin Registry]
        BUS[Message Bus]
        PIPE[Pipeline]
        RT[Agent Runtime]
    end
    Users <--> PLG
    PLG --> BUS
    BUS --> PIPE
    PLG -->|triggers| PIPE
    PIPE --> RT
    RT --> LLM
    GW --> PLG
    GW --> PIPE
```

| Plane | Responsibility |
|-------|----------------|
| **Ingress** | Channels and triggers normalize inbound stimuli |
| **Execution** | Pipeline coordinates turns, sessions, tools, model calls |
| **Egress** | Bus dispatches outbound messages to channels |

## Kernel packages

All packages under `kernel/` are plugin-agnostic core logic:

| Package | Role |
|---------|------|
| `kernel/bus` | Inbound/outbound message routing |
| `kernel/pipeline` | Turn coordinator; implements `TurnExecutor` |
| `kernel/agent` | SDK runtime wrapper |
| `kernel/session`, `kernel/sessionid` | Session routing and persistence |
| `kernel/scheduling` | Turn admission control |
| `kernel/health` | Liveness signals |
| `kernel/events` | Internal event bus |
| `kernel/turnctx` | Per-turn context |
| `kernel/executor` | `TurnExecutor` / `StreamRunner` contracts |
| `kernel/stringutil`, `kernel/log` | Shared utilities |
| `kernel/memory`, `kernel/prompt` | Workspace memory and system prompt |
| `kernel/slash`, `kernel/slashkind` | Slash command registry and dispatch |
| `kernel/config` | Config load, watch, hot reload |
| `kernel/voice` | TTS/STT provider interfaces |
| `kernel/channels` | Channel interface and manager |
| `kernel/task` | Background task tooling |
| `kernel/plugin` | Plugin axis interfaces and registry |

## Plugin axes

Defined in `kernel/plugin/plugin.go`. Each axis is an optional interface on `Plugin`:

| Interface | Contributes |
|-----------|-------------|
| `ChannelPlugin` | Chat transports (`channels.Channel`) |
| `ToolPlugin` | Agent tools (`tool.Tool`) |
| `SkillPlugin` | Prompt-time skills (`api.SkillRegistration`) |
| `TTSPlugin` / `STTPlugin` | Voice providers |
| `SlashPlugin` | Pre-model `/commands` |
| `TriggerPlugin` | Background triggers (cron, heartbeat) |

The registry (`kernel/plugin/registry.go`) collects contributions by axis at runtime.

## Plugin implementations

| Path | Axis |
|------|------|
| `plugins/channels/telegram`, `feishu`, `wecom`, `whatsapp`, `matrix`, `web` | Channel |
| `plugins/triggers/cron` | Trigger + Slash + Tool |
| `plugins/triggers/heartbeat` | Trigger |
| `plugins/skills/file` | Skill |
| `plugins/voice/cartesia`, `deepgram`, `elevenlabs`, `openai` | TTS/STT |
| `plugins/tools/acp` | Tool |

## Gateway as plugin host

The gateway wires kernel subsystems and hosts all plugins:

| File | Responsibility |
|------|----------------|
| `gateway/gateway.go` | `Gateway` struct, `Options`, `New` / `NewWithOptions` |
| `gateway/apply.go` | Single mutation path: `Apply`, runtime rebuild, channel reload |
| `gateway/lifecycle.go` | `Run`, `Shutdown`, signal handling, hot reload |
| `gateway/wire.go` | Composition root: plugin manifest + `Wire()` entry point |
| `gateway/triggers.go` | Trigger start/stop helpers |

### Apply loop

`Apply` is idempotent desired-state reconciliation:

1. Validate reload constraints (workspace immutability)
2. Stop background triggers
3. Load skills, build system prompt, register slash commands
4. Build fresh agent runtime from factory + plugin tools
5. Reload pipeline (swap runtime, re-apply channels)
6. Start triggers

`Run` calls `Apply` once at startup, then blocks on signals or config hot-reload (each reload re-enters `Apply`).

### Wire manifest pattern

**To see everything the binary does, read `wire.go`.** It is the single composition root:

- Instantiates every plugin (channels, cron, heartbeat, skills, voice, ACP)
- Registers them in `plugin.NewRegistry`
- Wires cross-plugin dependencies (e.g. web channel ↔ registry, cron ↔ pipeline)
- Exposes `Wire(cfg, logger)` as the production entry point

No other file should import `plugins/…` for side-effect registration.

## Kernel wall

`kernel/` must never import `github.com/ageneralai/maven/plugins/…`. Enforced by:

- Architectural rule: plugins depend on kernel, not vice versa
- `depguard` linter rule `kernel_no_plugins` in `.golangci.yml`

Only `gateway/wire.go` (and tests) cross the wall.

## Execution flow

```mermaid
sequenceDiagram
    participant CH as Channel Plugin
    participant BUS as Bus
    participant PIPE as Pipeline
    participant RT as Runtime
    participant LLM as LLM API
    CH->>BUS: inbound message
    BUS->>PIPE: dequeue
    PIPE->>PIPE: session resolve, slash dispatch
    PIPE->>RT: turn execution
    RT->>LLM: model call
    LLM-->>RT: response
    RT-->>PIPE: result
    PIPE->>BUS: outbound
    BUS->>CH: deliver
```

Background triggers (`cron`, `heartbeat`) call the same `TurnExecutor` the pipeline implements — identical tool and model behavior regardless of entry point.

## Configuration

Single config file (`~/.maven/config.json`). Gateway hot-reload watches the file and re-applies via `Apply`. Workspace files under `agent.workspace` supply memory context and skills.

## CLI entry

`cmd/maven` loads config, calls `gateway.Wire`, and runs the gateway lifecycle. Tests inject custom `Options.RuntimeFactory` via `NewWithOptions`.
