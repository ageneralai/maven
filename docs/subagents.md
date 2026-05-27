# Subagents

Maven can delegate scoped research, exploration, or planning work to **in-process SDK subagents** via the **`Task`** custom tool. This is separate from [ACP delegation](acp.md): `Task` runs built-in subagents inside Maven's SDK runtime; `DelegateTask` launches external ACP subprocesses.

## Model

- **In-process**: `Task` calls the current `ageneral-agents-go` runtime with `TargetSubagent` set to a built-in subagent.
- **Scoped child session**: child sessions are derived as `{parent}:task:{uuid}` so delegated work stays traceable to the parent turn.
- **No nesting**: a task running inside a `:task:` session cannot call `Task` again.
- **Tool-limited**: each built-in subagent receives the tool whitelist from its subagent definition.
- **External CLIs stay ACP**: use `DelegateTask` for configured coding CLIs under `tools.acp.agents`.

## Configuration

Enable the tool under `tools.task`:

```json
{
  "tools": {
    "task": {
      "enabled": true
    }
  }
}
```

When disabled, Maven does not register the `Task` tool.

## Tool: `Task`

| Parameter | Required | Description |
| --------- | -------- | ----------- |
| `name`    | yes      | Built-in subagent type: `general-purpose`, `explore`, or `plan`. |
| `goal`    | yes      | Specific instruction sent to the subagent. |
| `model`   | no       | Optional model tier override: `haiku` / `low`, `sonnet` / `mid` / `medium`, or `opus` / `high`. |

**Result**: the tool result is the subagent's final assistant text. If the subagent completes without text, Maven returns `(subagent completed with no text output)`.

## When to use

- Use **`explore`** for read-only repo inspection and codebase mapping.
- Use **`general-purpose`** for broader research or multi-step analysis inside Maven's runtime.
- Use **`plan`** for implementation planning before code changes.
- Use **`DelegateTask`** instead when the target is an external ACP-compatible agent process.

## Related files

- `internal/kernel/task/tool.go` — `Task` schema, validation, runtime request
- `internal/kernel/task/sdk.go` — registers `Task` from `tools.task`
- `internal/kernel/task/session.go` — child session IDs and nested task guard
- `internal/kernel/task/holder.go` — binds the tool to the SDK runtime after initialization
- `internal/kernel/agent/sdk_runtime.go` — attaches `Task` tools to runtime options
- `internal/kernel/config/config.go` — `TaskToolConfig`
