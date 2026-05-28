# Workspace

The workspace is the directory Maven reads for persona, memory, skills, and per-channel data. `maven onboard` creates it with a default layout you can edit by hand.

## Default location

Set by `agent.workspace` in config; defaults to `~/.maven/workspace`. Change it in `~/.maven/config.json` if you prefer another path. Workspace is **not** hot-reloadable — changing it requires a restart.

## Layout after `onboard`

```text
~/.maven/
  config.json
  sessions/                  # agentsdk per-session history (JSONL)
  workspace/
    AGENTS.md                # system prompt persona (editable)
    SOUL.md                  # identity / continuity (editable)
    HEARTBEAT.md             # heartbeat task text (starts empty)
    memory/
      MEMORY.md              # long-term facts (curated by mem-consolidate)
      2026-05-27.md          # daily journal (appended by `remember`)
    skills/
      <skill-name>/
        SKILL.md             # optional skill definitions
    .maven/
      session-router.json    # route-key → current session ID
      history/               # compact-seed system messages
    .telegram/               # optional Telegram assets
      slashes/               # custom /commands (markdown + frontmatter)
      handlers/              # handler scripts referenced by local slash defs
    .matrix/                 # Matrix sync state
      state.json
```

## Files Maven reads at startup

| Path | Purpose | Required |
|------|---------|----------|
| `AGENTS.md` | First block of the system prompt template. | No (empty when missing). |
| `SOUL.md` | Second block of the system prompt template. | No (empty when missing). |
| `HEARTBEAT.md` | Task text for each heartbeat tick. Empty file → tick is a no-op. | No. |
| `memory/MEMORY.md` | Long-term memory injected after the prompt template. | No. |
| `memory/YYYY-MM-DD.md` | Daily journal files for `memory_search` / `memory_get`. | No. |
| `skills/*/SKILL.md` | Skill definitions (frontmatter + body). | No. |
| `.telegram/slashes/*.md` | Custom Telegram slash command defs. | No. |

The system prompt assembly happens in `internal/kernel/prompt.BuildTemplate`. Memory is appended via the registry in `apply.go`.

## Editing AGENTS.md and SOUL.md

These define personality. Maven re-reads them on `Apply` (start, hot reload, or `/reload`). Send `/reload` after editing, or enable `gateway.hotReload` and save `config.json` to trigger the same path.

The defaults seeded by `onboard`:

- **`AGENTS.md`** instructs the model to use `memory_search` / `memory_get` before answering from guesswork and `remember` to record facts. It also sets a "voice mode" rule for short, spoken inputs.
- **`SOUL.md`** sets persona ("direct and efficient, technical when needed, proactive about using tools").

## Heartbeat task

`HEARTBEAT.md` is read on every heartbeat tick. Non-empty content becomes the agent prompt for that tick. The default body says "reply HEARTBEAT_OK when nothing to surface" so silent ticks log at debug only.

See [Guides: Heartbeat](heartbeat.md).

## Memory files

Memory is split:

- **`memory/MEMORY.md`** — long-term, curated. Promoted to the system prompt at every turn. Edited by the agent (via the file system) under the supervision of the `mem-consolidate` background pass.
- **`memory/YYYY-MM-DD.md`** — daily journals. `remember(content)` appends a line. The agent retrieves these on demand with `memory_search` / `memory_get`.

Full details in [Guides: Memory](memory.md).

## Skills

A skill is a folder under `skills/` containing a `SKILL.md` with YAML frontmatter and a body. The body becomes the system prompt injected when one of the skill's keywords matches the user's input. See [Guides: Skills](skills.md).

## Telegram assets

The Telegram channel optionally reads from `.telegram/`:

- `slashes/<name>.md` — slash command definitions (built-in `/cron-add`-style live in plugins; these are user-defined).
- `handlers/<name>` — executable scripts invoked by `type: local` slash commands.

Channel root can be overridden via `channels.telegram.rootDir`. See [Channels: Telegram](../channels/telegram.md) and [Guides: Slash commands](slash-commands.md).

## Matrix state

`.matrix/state.json` holds Matrix sync state (`nextBatch`, `filterId`, `deviceId`). Delete it to force a full re-sync.

## Sample workspace in the repo

A working reference workspace lives at [`docs/workspace/`](https://github.com/ageneralai/maven/tree/main/docs/workspace) in the repository. It mirrors `onboard` defaults plus example Telegram slashes and one example skill. The `maven` binary does **not** read this path — copy pieces into your own workspace if you want them.
