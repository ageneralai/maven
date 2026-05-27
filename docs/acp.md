# ACP delegation

Maven can delegate coding work to **external ACP (Agent Client Protocol)** processes via the **`DelegateTask`** custom tool. Protocol reference: [agentclientprotocol.com](https://agentclientprotocol.com). Go client: **`github.com/coder/acp-go-sdk`** (version pinned in `go.mod`). Code: `internal/plugins/tool/acp` (client, tool, `plugin.ToolPlugin` adapter), `internal/gateway/wire.go` (registry), `internal/kernel/config/config.go` (`tools.acp`), `internal/plugins/channel/telegram/stream_state.go` + `card.go` (status card streaming).

## Model

- **Subagent only**: the user talks to Maven; Maven's model chooses **`DelegateTask`**; the ACP process is never a first-class chat peer.
- **One turn, one subprocess**: `exec.CommandContext` spans a single tool invocation; context cancel / timeout kills the child. No cross-reload session pinning in Maven.
- **Progress vs answer**: ACP updates are **`emit` → `tool.StreamingTool` → `api.EventToolExecutionOutput`** (payload in **`event.Output`**, asserted as **`string`** in the Telegram handler). The Telegram **status card** shows those chunks under the **`DelegateTask`** row; **main model text** stays in the normal content stream.

## Configuration

In `~/.maven/config.json` under **`tools.acp`** (see also `config.example.json`):

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | When true and at least one valid agent exists, registers **`DelegateTask`**. |
| `agents` | map | Keys are logical names the model passes as **`agent`**. Values are **`ACPAgent`** rows (below). |

**`ACPAgent`** (per map entry):

| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable name or path on **`PATH`** (required). Never taken from tool JSON—only from config. |
| `args` | []string | Fixed arguments (optional). |
| `env` | []string | Extra **`KEY=value`** entries appended to the process env (optional). |

**Workspace**: Tool parameter **`cwd`** is resolved and, when **`tools.restrictToWorkspace`** is true, must stay inside **`agent.workspace`**. Client FS hooks (**`ReadTextFile`** / **`WriteTextFile`**) enforce the same rule for paths the ACP agent requests.

Example:

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

## Tool: `DelegateTask`

| Parameter | Required | Description |
|-----------|----------|-------------|
| `agent` | yes | Must match a key under **`tools.acp.agents`**. |
| `prompt` | yes | Task text sent as the ACP **`Prompt`**. |
| `cwd` | no | Working directory for the subprocess (default: `agent.workspace`). |

**Result**: the tool result is the ACP process's final answer. Streaming progress chunks appear in the Telegram status card during execution.

## Related files

- `internal/plugins/tool/acp/plugin.go` — `plugin.ToolPlugin` adapter
- `internal/plugins/tool/acp/tool.go` — `DelegateTask` schema and execution
- `internal/plugins/tool/acp/client.go` — ACP protocol client and FS hooks
- `internal/plugins/tool/acp/sdk.go` — builds tools from config
- `internal/kernel/config/config.go` — `ACPToolConfig`, `ACPAgent`
- `internal/gateway/wire.go` — registry registration
- `internal/plugins/channel/telegram/stream_state.go`, `card.go` — status card streaming
