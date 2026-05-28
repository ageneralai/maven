# Heartbeat

Heartbeat is a periodic trigger that runs an agent turn from `HEARTBEAT.md` on a fixed interval. Use it for proactive checks ("anything I should surface?") that aren't reactive to user input.

## How it fires

```mermaid
flowchart LR
    T[Ticker every N] --> P[Pulse SignalHeartbeatTick]
    P --> A{Try acquire<br/>1-slot lane}
    A -- busy --> S[Log skipped]
    A -- ok --> R[Read HEARTBEAT.md]
    R --> E{Empty?}
    E -- yes --> Done[No-op]
    E -- no --> X[RunTurn with heartbeat:{uuid}]
    X --> OK{Output contains<br/>HEARTBEAT_OK?}
    OK -- yes --> Debug[debug log]
    OK -- no --> Info[info log: result]
```

## Configuration

There is no `heartbeat` section in config today. The default interval is **30 minutes** (`internal/plugins/trigger/heartbeat`). To change it, modify the plugin instantiation in `internal/gateway/wire.go`:

```go
hbPlug := heartbeat.NewPlugin(15*time.Minute, core.logger, heartbeat.WithHealthReporter(core.liveness))
```

A future config field is the obvious place for this; until then, the default applies.

## Prompt: `HEARTBEAT.md`

`internal/plugins/trigger/heartbeat.FileTrigger` reads `<workspace>/HEARTBEAT.md` on every tick. The file content (trimmed) becomes the agent prompt. Empty or missing files skip the tick entirely.

`maven onboard` seeds an empty file. The sample workspace ships a minimal prompt:

```markdown
On each heartbeat tick, check whether anything needs the user's attention based on earlier context.

Reply with HEARTBEAT_OK when there is nothing to surface; otherwise give a concise status the gateway can log.
```

The `HEARTBEAT_OK` convention is a marker the trigger uses to log at `debug` rather than `info`. The trigger does not enforce it — anything else is logged with the truncated result.

## Sessions

Each tick gets a fresh session ID: `heartbeat:{uuid}` (`sessionid.New(KindHeartbeat, "")`). Two ticks never share history.

## Concurrency

Heartbeat has its own try-once weight-1 admission lane (`kernel/scheduling.Lane` with capacity 1). If a previous tick is still running when the next fire is due:

- The new tick is skipped with `heartbeat skipped: previous tick still running` at debug.
- The next tick attempts again on the following interval.

Heartbeat does **not** share an admission lane with cron. They can run at the same time.

## Liveness

The trigger calls `health.Pulse(SignalHeartbeatTick)` at the start of every fire — before the lane try. External liveness probes can use this to verify the ticker is alive.

## Disabling

Heartbeat runs unconditionally when the trigger is registered (`wire.go`). To disable in practice:

- Leave `HEARTBEAT.md` empty (recommended). Ticks fire but immediately no-op.
- Or remove the plugin from `wire.go` (requires rebuild).

## Extending: custom triggers

`heartbeat.WithTrigger(yourTrigger)` lets tests or extensions replace the file-based prompt source:

```go
heartbeat.WithTrigger(heartbeat.StaticTrigger("Check inbox and flag urgent items."))
```

The `Trigger` interface is `Prompt() string`; an empty return skips the tick.

## Related

- [Guides: Memory](memory.md) — heartbeat can use `memory_search` to keep context across days.
- [Concepts: Sessions](../concepts/sessions.md) — heartbeat sessions are isolated from chat history.
