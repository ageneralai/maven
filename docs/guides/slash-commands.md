# Slash commands

A slash command is text that starts with `/` and triggers a registered handler before (or instead of) the model. Maven supports three sources:

1. **Built-in slash commands** in `kernel/slash` (e.g. `/compact`) and gateway-registered commands (e.g. `/reload`, `/status`).
2. **Plugin-contributed slash commands** via the `SlashPlugin` axis (e.g. `/cron-add`, `/cron-list`, `/cron-remove`).
3. **Workspace-defined Telegram slash commands** loaded from `<workspace>/.telegram/slashes/*.md`.

A single inbound supports **one** slash invocation per turn. Multiple `/` lines return an error.

## Parsing

The lexer mirrors `agentsdk-go/pkg/runtime/commands`:

- Quotes: `"`, `'`. Backslash escapes one character.
- Long flags: `--key=value` or `--key value` (space-separated when the next token does not start with `-`).
- Bare arguments after the command name accumulate into `Args`.

```text
/cron-add --name reminder --in 1h --message "ping back" --deliver --channel telegram --to 42
```

## Dispatch

`slash.PreTurn(ctx, registry, input)` parses, looks up the handler, and runs it:

- **Empty `Result.Output`** ⇒ the turn continues to the model. `Result.Metadata` is merged into the model request (enriched with `slash.turn.channel` and `slash.turn.chat_id`). `Result.PostAction` is queued for post-response handling.
- **Non-empty `Result.Output`** ⇒ skipped model run; the trimmed output is the user reply.

The dispatcher applies an optional **expected slash name** filter: channels that parsed `/command` themselves (like Telegram's workspace slashes) set `ExpectedSlashName` so PreTurn only fires when the parsed name matches; otherwise it falls through to the model.

On Telegram, kernel and plugin slashes are registered in the BotFather menu automatically on gateway start (merged with workspace defs under `.telegram/slashes/`; workspace overrides description and handling when names collide).

## Built-in: `/reload`

`/reload` re-runs `Gateway.Apply` without restarting the process. Use it after editing `AGENTS.md`, `SOUL.md`, `memory/MEMORY.md`, or skills under `skills/` — the config watcher only watches `config.json`.

The handler replies `Reloading…` immediately; the reload runs on the gateway main loop (same path as hot reload). Works with or without `gateway.hotReload`.

## Built-in: `/status`

`/status` prints cron job counts and `MEMORY.md` size. No model turn.

## Built-in: `/compact`

`/compact [free-form focus text]` compresses the current conversation into a continuation summary and rotates the chat session. Implementation: `internal/kernel/slash/builtin_compact.go`.

Flow:

1. The handler emits a `Metadata["api.prepend_prompt"]` instructing the model to produce a continuation summary (no user-facing fluff).
2. The model runs and returns the summary.
3. The post-action handler (`postaction.HandlePostResponse`) sees a `CompactRotateAction`:
   - Optional pre-compact flush hook fires (the gateway wires `remember` here so important context lands in long-term memory before the rotation).
   - The router rotates the session.
   - The summary is seeded as a `system` message into the new session's history file (`<workspace>/.maven/history/<sessionID>.json`).
4. Reply is either the summary or a fixed ack ("Conversation compacted and continued in a fresh session.") depending on response mode.

## Built-in: `/new`

Routing hint, not a registered slash. Channels emit `bus.RoutingHints.BuiltinCommand = "new"`. The pipeline calls `Router.Rotate(routeKey)` and replies "Started a fresh session." — the model is never invoked. Telegram emits this from a top-level handler so users can run `/new` even without a workspace slash def.

## Plugin slash: cron

The cron plugin contributes three slash commands:

| Command | Description |
|---------|-------------|
| `/cron-add` | Schedule a persisted job. Exactly one of `--expr`, `--in`, or `--at-ms`. With `--deliver true` plus `--channel` / `--to`, sends the job result to a chat. |
| `/cron-list` | Print all persisted jobs (id, schedule, delivery). |
| `/cron-remove --id <id>` | Remove a job by id. |

Examples:

```text
/cron-add --name water --in 30m --message "Remind me to drink water"
/cron-add --name standup --expr "0 30 9 * * MON-FRI" --message "Daily standup time" --deliver true --channel telegram --to 42
/cron-list
/cron-remove --id 7d1a0c…
```

See [Guides: Cron jobs](cron.md) for the field reference.

## Workspace Telegram slashes

Drop a markdown file under `<workspace>/.telegram/slashes/`:

```markdown
---
command: compact
description: Compress conversation history
type: pipeline
passthrough: true
streaming: false
---
```

| Frontmatter field | Type | Default | Meaning |
|-------------------|------|---------|---------|
| `command` | string | required | Slash name without leading `/`. |
| `description` | string | `""` | Shown in Telegram's BotFather command list (truncated to 256 chars). |
| `type` | enum | `agent` | `local` (script handler), `agent` (model run with prompt or args), `pipeline` (model run via the kernel slash dispatch). |
| `session` | enum | `current` | `current` or `isolated`. Local commands must use `current`. |
| `handler` | string | `""` | Script name under `.telegram/handlers/` (local type only). |
| `passthrough` | bool | `false` | Pipeline-only. When true, sends `/cmd args` straight to the dispatcher instead of substituting a prompt body. |
| `streaming` | bool | `true` | Whether the gateway uses the streaming pipeline when this slash fires. |

The body becomes the agent prompt (when `passthrough: false`). Use this for "prompt presets":

```markdown
---
command: brief
description: Summarize today's events
type: agent
session: current
---

Summarize what happened today based on my memory and notes. Keep it under 100 words.
```

For `local`, Maven shells out to `.telegram/handlers/<handler>` with `(session_id, args)` and `WORKSPACE` / `SESSION_ID` env vars. Stdout becomes the reply.

## Adding a slash command in a plugin

```go
// internal/plugins/yourplugin/slash.go
func (p *Plugin) SlashCommands(*config.Config) []plugin.SlashCommand {
    return []plugin.SlashCommand{
        {
            Definition: plugin.SlashDefinition{
                Name:        "ping",
                Description: "Replies with pong.",
            },
            Handler: handlerFunc(func(ctx context.Context, inv plugin.SlashInvocation) (plugin.SlashResult, error) {
                return plugin.SlashResult{Command: "ping", Output: "pong"}, nil
            }),
        },
    }
}
```

Then implement `SlashPlugin` on the plugin struct and add it to the registry in `wire.go`. The gateway merges these with built-ins on every `Apply`.

## Errors

- **Parse error** (unclosed quote, dangling escape, invalid name) → user sees the command-error template.
- **Handler error** → same template, logged at error level.
- **Multiple commands in one inbound** → error.
- **Duplicate registration** (two plugins claim the same name) → fatal at gateway start.
