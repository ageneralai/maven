# Sample workspace

This tree **mirrors** a real **`agent.workspace`** (default **`~/.maven/workspace`**). The **`maven` binary does not read this path** — copy pieces into your own workspace if you want these files without editing them by hand after **`maven onboard`**.

| Path | Role |
|------|------|
| **`AGENTS.md`** / **`SOUL.md`** | Same default wording as **`maven onboard`** (canonical source is still the literals in **`cmd/maven/main.go`**). |
| **`HEARTBEAT.md`** | **`maven onboard`** seeds an empty file (heartbeat skips until there is non-empty prompt text). This sample contains a minimal prompt including the **`HEARTBEAT_OK`** convention. |
| **`memory/MEMORY.md`** | Empty, matching onboard seed. Memory store and journals live under **`memory/`**. |
| **`skills/`** | Optional skills (`SKILL.md` per subdirectory); see **`README.md`** Skills section. |
| **`.telegram/slashes/`** | Optional Telegram slash defs from the earlier sample workspace: **`compact`** (pipeline), **`help`** and **`status`** (local); see **`docs/telegram-setup.md`**. |

Layout overview: **`docs/workspace.md`**.
