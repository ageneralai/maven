# ACP delegation

Maven can delegate coding tasks to **external [ACP](https://agentclientprotocol.com)** processes via the `DelegateTask` tool. Protocol implementation: `github.com/coder/acp-go-sdk` (pinned in `go.mod`). Maven code: `internal/plugins/tool/acp`.

## Model

- **Subagent only.** The user talks to Maven; Maven's model calls `DelegateTask`; the ACP process is never a first-class chat peer.
- **One turn, one subprocess.** `exec.CommandContext` spans a single tool invocation; context cancellation or timeout kills the child. No cross-reload session pinning in Maven.
- **Progress vs answer.** ACP progress chunks stream into the channel's status card (where supported, e.g. Telegram); the model's main reply stays in the normal content stream.

## Configuration

```json
{
  "tools": {
    "restrictToWorkspace": true,
    "acp": {
      "enabled": true,
      "agents": {
        "claude": {
          "command": "npx",
          "args": ["-y", "@zed-industries/claude-code-acp@latest"]
        },
        "gemini": {
          "command": "gemini",
          "args": ["--experimental-acp"]
        }
      }
    }
  }
}
```

| Field     | Description                                                                      |
| --------- | -------------------------------------------------------------------------------- |
| `enabled` | When `true` and at least one valid agent entry exists, registers `DelegateTask`. |
| `agents`  | Map of logical name → `ACPAgent`. The model passes the logical name as `agent`.  |

**`ACPAgent`** (one per map entry):

| Field     | Type     | Description                                                                  |
| --------- | -------- | ---------------------------------------------------------------------------- |
| `command` | string   | Executable name or path on `PATH`. Required. **Never** taken from tool JSON. |
| `args`    | []string | Fixed arguments. Optional.                                                   |
| `env`     | []string | Extra `KEY=value` entries appended to the process env. Optional.             |

The agent binary and arguments come **only from config**. The model cannot supply arbitrary shell commands.

## Workspace restriction

When `tools.restrictToWorkspace` is true:

- The `cwd` parameter to `DelegateTask` must resolve inside `agent.workspace` (symlinks resolved before comparison).
- The ACP client's `ReadTextFile` / `WriteTextFile` hooks reject paths outside the workspace.

Untick this if you have a deliberate reason; defaults are safe.

## Tool: `DelegateTask`

```json
{
  "name": "DelegateTask",
  "input": {
    "agent": "claude",
    "prompt": "Refactor cron.Service.checkAndFire to extract due-job selection.",
    "cwd": "/home/me/.maven/workspace/project-x",
    "timeout": 180
  }
}
```

| Parameter | Required | Description                                                          |
| --------- | -------- | -------------------------------------------------------------------- |
| `agent`   | yes      | Map key from `tools.acp.agents`.                                     |
| `prompt`  | yes      | Task text sent as the ACP `Prompt`.                                  |
| `cwd`     | no       | Working directory for the subprocess. Defaults to `agent.workspace`. |
| `timeout` | no       | Seconds before the subprocess is killed. Default 120.                |

The tool is marked `IsDestructive: true` in `tool.Metadata` so SDK consumers can prompt for confirmation.

## Subprocess lifecycle

1. `exec.CommandContext(ctx, agent.Command, agent.Args...)` is started with `cmd.Dir = cwd` and the merged env.
2. ACP `Initialize` runs over the subprocess's stdio with file-system capabilities advertised.
3. `NewSession` then `Prompt`. Streamed events flow back into the channel's status card via the `emit` callback (this is the `DelegateTask` row that scrolls subprocess output).
4. On completion, the final assistant message is returned as the tool result.

`ctx` cancellation (chat ctx, gateway shutdown, or timeout) kills the process via `cmd.Process.Kill()` and the cleanup `defer cmd.Wait()`.

## Permissions

The `RequestPermission` hook auto-selects an "allow once" / "allow always" option when offered. If no allow option is present, the first option is selected. Cancellation is the default for empty option lists.

This is intentional: Maven delegates the user's intent through the tool; the ACP agent is acting on behalf of the user already inside a sandboxed workspace. Tune your ACP agent's permission policy if you need stricter prompts.

## Terminal support

Currently **not** supported. The ACP `CreateTerminal` / `TerminalOutput` / `KillTerminal` calls return `terminal not supported in gateway delegate mode`. Agents that rely on PTY tooling will fall back to non-terminal paths or fail.

## Related files

| File                                                           | Role                                         |
| -------------------------------------------------------------- | -------------------------------------------- |
| `internal/plugins/tool/acp/plugin.go`                          | Plugin axis adapter.                         |
| `internal/plugins/tool/acp/tool.go`                            | `DelegateTask` schema and execution.         |
| `internal/plugins/tool/acp/client.go`                          | ACP client, FS hooks, permission auto-allow. |
| `internal/plugins/tool/acp/sdk.go`                             | Build tools from config.                     |
| `internal/plugins/tool/acp/path.go`                            | Workspace-aware path resolver.               |
| `internal/plugins/channel/telegram/stream_state.go`, `card.go` | Status card streaming for tool output.       |
