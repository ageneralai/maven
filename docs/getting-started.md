# Getting started

Maven is a personal AI assistant built on [ageneral-agents-go](https://github.com/ageneralai/ageneral-agents-go). This page covers the fastest path to a running binary; channel setup links are in the sidebar.

## Build and run

```bash
make build
make setup          # interactive config → ~/.maven/config.json

./maven agent -m "Hello"   # single message
make run                   # REPL
make gateway               # channels + cron + heartbeat
```

Smaller release binary: `make build-release`. Package for distribution: `make package` or `make package-all`.

## First-time workspace

```bash
make onboard
```

Creates default workspace files under your configured `agent.workspace` (skills dir, memory templates, etc.).

## Next steps

- **Telegram** — [telegram-setup.md](telegram-setup.md)
- **Feishu** — [feishu-setup.md](feishu-setup.md)
- **WeCom** — [wecom-setup.md](wecom-setup.md)
- **Architecture** — [architecture.md](architecture.md)
- **Subagents** — [subagents.md](subagents.md)
- **ACP delegation** — [acp.md](acp.md)
- **Voice (Web UI)** — [voice.md](voice.md)
- **Plugins** — [plugins.md](plugins.md)

Full Makefile targets, Docker, and configuration reference: [README](https://github.com/ageneralai/maven/blob/main/README.md) on GitHub.

To edit or publish this documentation site: [contributing.md](contributing.md).
