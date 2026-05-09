# ACP delegation (subagent)

Maven can delegate coding work to **external ACP (Agent Client Protocol)** processes via the **`DelegateTask`** custom tool. Protocol reference: [agentclientprotocol.com](https://agentclientprotocol.com). Go client: **`github.com/coder/acp-go-sdk`** (version pinned in the module; see `go.mod` / `go.sum`). Code: `pkg/acp` (client + tool), `pkg/plugin/acp` (gateway plugin wrapper), `internal/gateway/gateway.go` (registry), `internal/config/config.go` (`tools.acp`), `internal/channel/telegram/stream_state.go` + `card.go` (status card streaming).

## Model

- **Subagent only**: the user talks to Maven; Maven’s model chooses **`DelegateTask`**; the ACP process is never a first-class chat peer.
- **One turn, one subprocess**: `exec.CommandContext` spans a single tool invocation; context cancel / timeout kills the child. No cross-reload session pinning in Maven.
- **Progress vs answer**: ACP updates are **`emit` → `tool.StreamingTool` → `api.EventToolExecutionOutput`** (payload in **`event.Output`**, asserted as **`string`** in the Telegram handler). The Telegram **status card** shows those chunks under the **`DelegateTask`** row; **main model text** stays in the normal content stream (**not** mixed into `textBuf` for tool output).

## Configuration

In `~/.maven/config.json` under **`tools.acp`** (see also **`config.example.json`** → **`tools.acp`**):

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | When true and at least one valid agent exists, registers **`DelegateTask`**. |
| `agents` | map | Keys are logical names the model passes as **`agent`** (e.g. `codex`, `claude`). Values are **`ACPAgent`** rows (below). |

**`ACPAgent`** (per map entry):

| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable name or path on **`PATH`** (required). Never taken from tool JSON—only from config. |
| `args` | []string | Fixed arguments (optional). |
| `env` | []string | Extra **`KEY=value`** entries appended to the process env (optional). |

**Workspace**: Tool parameter **`cwd`** is resolved and, when **`tools.restrictToWorkspace`** is true, must stay inside **`agent.workspace`** (same idea as other sandboxed tools). Client FS hooks (**`ReadTextFile`** / **`WriteTextFile`**) enforce the same rule for paths the ACP agent requests.

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

Adjust commands to whatever your host installs (`codex-acp`, local binaries, etc.).

## Tool: `DelegateTask`

| Parameter | Required | Description |
|-----------|----------|-------------|
| `agent` | yes | Must match a key under **`tools.acp.agents`**. |
| `prompt` | yes | Task text sent as the ACP **`Prompt`**. |
| `cwd` | no | Working directory; default **`agent.workspace`**. |
| `timeout` | no | Seconds for the whole subprocess turn (default **120**). |

**Result**: Maven’s tool result **`Output`** is the delegated agent’s **final accumulated assistant text** (not the streamed card lines). On failure, errors include captured **stderr** from the subprocess when present (`acp subprocess stderr:`).

## Telegram UX

Status card visibility follows existing **`channels.telegram.feedback`**: **`normal`** and **`debug`** show the card (and thus streamed **`DelegateTask`** output); **`minimal`** / **`silent`** hide it. **`debug`** still logs non-noisy stream types as today.

## Related files

- `pkg/acp/client.go` — stdio **`ClientSideConnection`**, **`Initialize` → `NewSession` → `Prompt`**, permission auto-select, stderr capture on errors
- `pkg/acp/tool.go` — **`tool.StreamingTool`** implementation and JSON schema
- `pkg/acp/path.go` — **`resolveWorkspacePath`** for **`cwd`** and FS paths
- `pkg/plugin/acp/plugin.go` — **`plugin.Plugin`** wrapping **`pkg/acp.Tools`**
- `github.com/ageneralai/ageneral-agents-go/pkg/api/runtime_tool_executor.go` — **`StreamSink`** → **`EventToolExecutionOutput`** (**`ToolUseID`** matches **`EventToolExecutionStart`**)
