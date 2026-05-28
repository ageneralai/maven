# Subagents

A subagent, the `Task` tool, delegates a scoped goal to an **in-process SDK subagent**. It runs inside Maven's runtime (same model client, same workspace) and is separate from [ACP delegation](acp-delegation.md), which spawns external CLIs as subprocesses.

## When to use Task vs DelegateTask

| Goal                                                     | Use                                                  |
| -------------------------------------------------------- | ---------------------------------------------------- |
| Read-only repo inspection, codebase mapping              | `Task` with `name: "explore"`                        |
| Multi-step research inside Maven's runtime               | `Task` with `name: "general-purpose"`                |
| Implementation planning before code changes              | `Task` with `name: "plan"`                           |
| Coding work in an external CLI (Claude Code, Gemini CLI) | `DelegateTask` (configured under `tools.acp.agents`) |

The built-in subagent types come from `ageneral-agents-go/pkg/runtime/subagents`. Each carries its own tool whitelist.

## Enable

```json
{
  "tools": {
    "task": {
      "enabled": true
    }
  }
}
```

When disabled, the `Task` tool is not registered.

## Tool schema

```json
{
  "name": "Task",
  "input": {
    "name": "explore",
    "goal": "Find every place we call the cron service from outside its package.",
    "model": "sonnet"
  }
}
```

| Parameter | Required | Description                                                                   |
| --------- | -------- | ----------------------------------------------------------------------------- |
| `name`    | yes      | `general-purpose`, `explore`, or `plan`.                                      |
| `goal`    | yes      | Specific instruction for the subagent.                                        |
| `model`   | no       | Tier override: `haiku` / `low`, `sonnet` / `mid` / `medium`, `opus` / `high`. |

**Result:** the subagent's final assistant text. If the subagent emits no text, Maven returns `(subagent completed with no text output)`.

## Sessions

- Each `Task` invocation derives a fresh `task:{uuid}` session ID (`internal/kernel/sessionid.KindTask`).
- The runtime request includes `Metadata.parent_session` so the SDK can correlate work back to the parent turn.
- **Nesting is rejected.** If the parent turn is already a `task:` session (i.e. you're already inside a subagent), the tool errors with "nested task delegation is not supported."

## Tool whitelisting

Each built-in subagent ships with its own `BaseContext.ToolWhitelist`. The Task tool copies that list into the `api.Request.ToolWhitelist`, so the subagent only sees the tools its type is meant to use (e.g. `explore` cannot edit files).

## Plumbing

| File                                   | Role                                                    |
| -------------------------------------- | ------------------------------------------------------- |
| `internal/kernel/task/tool.go`         | `Task` schema, validation, runtime request.             |
| `internal/kernel/task/sdk.go`          | Registers the tool from `tools.task` config.            |
| `internal/kernel/task/session.go`      | Child session IDs and nested-task guard.                |
| `internal/kernel/task/holder.go`       | Binds the tool to the live SDK runtime after `api.New`. |
| `internal/kernel/agent/sdk_runtime.go` | Attaches the Task tool to runtime options.              |

## Common patterns

**Investigate before fixing:**

```text
Use Task(name=explore, goal="map where InvoiceService is used in api/ and worker/") and summarize before proposing changes.
```

**Plan, then code:**

```text
Use Task(name=plan, goal="Outline the steps to migrate cron.Service to a new admission lane.") and present the plan.
After the user approves, edit the files.
```

**Cheap research:**

```text
Use Task(name=general-purpose, model=low, goal="Find vendored libraries that have moved to /maintenance.").
```
