# Cron jobs

Cron is a persisted, on-disk job scheduler that runs the same agent runtime your chat channels do. Jobs survive restarts, deduplicate fire-while-busy, and optionally deliver their output to a chat.

## Configuration

```json
{
  "gateway": {
    "cron": {
      "maxConcurrentRuns": 1
    }
  }
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `gateway.cron.maxConcurrentRuns` | `1` (when omitted) | Max concurrent cron turns. Applied at gateway start only. Changing requires **restart** (not hot reload). |

Job definitions are persistent in `~/.maven/data/cron/jobs.json` (created on first add). Don't edit by hand — use the tools, slash commands, or restart.

## Schedule types

Exactly one of these per job:

| Type | Field | Example | Behavior |
|------|-------|---------|----------|
| Six-field cron | `expr` | `"0 30 9 * * MON-FRI"` | Repeating. Seconds + minutes + hours + day-of-month + month + day-of-week. Parser: `gronx`. |
| Duration from now | `in` | `"1h30m"` | One-shot. Resolves to `now + duration` at add time. |
| Unix milliseconds | `at_ms` | `1746000000000` | One-shot. Absolute target. |

One-shot jobs (`in` or `at_ms`) auto-disable after firing. Repeating (`expr`) jobs reschedule from `LastRunAtMs`.

## Payload

| Field | Required | Description |
|-------|----------|-------------|
| `message` | yes | Prompt the agent will run when the job fires. |
| `deliver` | no | When `true`, publish the agent's output to a channel. |
| `channel` | when delivering | Outbound channel name (e.g. `telegram`). |
| `to` | when delivering | Recipient ID in that channel. |

Validation rules (`Payload.Validate`):

- `deliver = false` requires `channel` and `to` to be empty.
- `deliver = true` requires both non-empty.
- Reserved literal `"deliver_to_incoming_chat"` in `to` is rejected (it's a tool sugar, not a recipient).

## Agent tools

The cron plugin contributes three tools to the runtime:

### `cron-schedule`

```json
{
  "name": "cron-schedule",
  "input": {
    "name": "standup",
    "message": "It's standup time. Summarize yesterday and queue today's plan.",
    "expr": "0 30 9 * * MON-FRI",
    "deliver_to_incoming_chat": true
  }
}
```

| Parameter | Description |
|-----------|-------------|
| `name` | Label. |
| `message` | Agent prompt when the job runs. |
| `expr` / `in` / `at_ms` | Exactly one. |
| `deliver` | Send output to `channel + to`. Mutually exclusive with `deliver_to_incoming_chat`. |
| `deliver_to_incoming_chat` | Send to the current inbound chat. Channel + to come from `turnctx`. |
| `channel`, `to` | Explicit recipient for `deliver: true`. Omit when using `deliver_to_incoming_chat`. |

**Inference behavior:** if you omit all delivery fields while running inside an active chat, the tool sets `deliver_to_incoming_chat = true` automatically. Explicitly passing `deliver: false` disables that inference.

### `cron-list`

No parameters. Returns one job per line: `id name="…" enabled=on|off [schedule] msg="…" [→channel:to]`.

### `cron-remove`

```json
{ "name": "cron-remove", "input": { "id": "7d1a0c…" } }
```

## Slash commands

The same operations are available as slash commands so users can drive them directly:

```text
/cron-add --name remind --in 1h --message "Walk the dog"
/cron-add --name standup --expr "0 30 9 * * MON-FRI" --message "Standup" --deliver true --channel telegram --to 42
/cron-list
/cron-remove --id 7d1a0c…
```

The `--deliver true` flag requires non-empty `--channel` and `--to`. There is no `--deliver-to-incoming-chat` flag in slash; use the agent tool from within a chat if you want that sugar.

## Execution semantics

```mermaid
flowchart LR
    T[Ticker / wake notify] --> S[checkAndFire]
    S --> D[Find due jobs<br/>under mutex]
    D --> Q[Disable one-shots,<br/>clear nextRunAtMs,<br/>save jobs.json]
    Q --> A[Acquire admission<br/>weighted semaphore]
    A --> R[RunTurn with<br/>cron:{id}:{uuid} session]
    R --> P[Persist last run / error]
    R --> O{Deliver?}
    O -- yes --> Pub[OutboundPublisher]
    O -- no --> End[Done]
    Pub --> Cap{ReactiveOnly channel?}
    Cap -- yes --> Skip[Log skip]
    Cap -- no --> Send[Publish to bus]
```

Key properties:

- **At-most-once-per-tick.** A job that is due is taken off the next-run schedule before its goroutine acquires the semaphore. Two ticks of the same job cannot both fire from the same `NextRunAtMs`.
- **Crash-safe.** `jobs.json` is rewritten atomically (`os.Rename` from temp file). Disabling one-shots happens before fire, so a crash mid-run does not re-fire on restart.
- **Per-run isolation.** Each fire derives `sessionid.New(KindCron, jobID)` → `cron:{jobID}:{uuid}`. Different runs of the same job never share history.
- **Admission lane.** All cron fires share a weighted semaphore (`gateway.cron.maxConcurrentRuns`). Heartbeat has its own one-slot lane. Chat is uncapped.

## Delivery

When `deliver: true`:

- The publisher hits the message bus (`bus.OutboundPublisher.PublishOutbound`).
- Before publishing, the manager checks the channel's capabilities. If `ReactiveOnly` is true (e.g. WeCom, which only allows passive replies via expiring `response_url`), the deliver is skipped and logged. Use another channel for proactive delivery from WeCom contexts.

## Errors

Per job, the persisted state carries `lastStatus` (`ok` / `error`) and `lastError` (string). Subsequent ticks log at info; errors do not auto-disable a repeating job.
