<p align="center">
  <img src="https://github.com/ageneralai/maven/blob/main/website/public/images/panorama_photosphere_120dp_000_FILL0_wght500_GRAD0_opsz48.png?raw=true" alt="Maven" width="120">
</p>

<h1 align="center">Maven</h1>

<p align="center">
  A personal AI assistant. One Go binary that runs locally or in a container, talks to your favorite chat apps, schedules its own work, and delegates coding tasks to external agents.
</p>

<p align="center">
  <a href="https://ageneralai.github.io/maven/"><strong>Documentation</strong></a> ·
  <a href="https://ageneralai.github.io/maven/getting-started/">Get started</a> ·
  <a href="https://ageneralai.github.io/maven/concepts/architecture/">Architecture</a> ·
  <a href="https://ageneralai.github.io/maven/reference/configuration/">Reference</a>
</p>

<p align="center">
  Built on <a href="https://github.com/ageneralai/ageneral-agents-go">ageneral-agents-go</a> · Licensed under <a href="LICENSE">MIT</a>
</p>

---

## Highlights

- **Chat in your apps.** Telegram, Feishu (Lark), WeCom, Matrix, WhatsApp, and a built-in Web UI all flow through one agent runtime.
- **Run on a schedule.** Persistent cron jobs and a periodic heartbeat call the same execution path the chat surfaces use.
- **Speak and listen.** Realtime browser voice via Deepgram STT and OpenAI / Deepgram / ElevenLabs / Cartesia TTS.
- **Delegate work.** A `Task` tool runs in-process subagents (explore, plan, general-purpose); a `DelegateTask` tool launches external [ACP](https://agentclientprotocol.com) coding agents (Claude Code, Gemini CLI, …) as subprocesses.
- **Remember.** Long-term `MEMORY.md` plus daily journals. A shadow journaler automatically records net-new facts from every conversation turn; a background pass promotes them to long-term memory.
- **Stay private.** Process-wide egress honors `HTTPS_PROXY`, `SSL_CERT_FILE`, and `NO_PROXY` so you can route everything through a vault like [OneCLI](https://github.com/onecli/onecli).
- **Hot reload.** Edit `~/.maven/config.json` and the gateway re-applies without restarting.

## Quick start

```bash
make build               # build the binary
make setup               # interactive ~/.maven/config.json (fills in a provider key)
make onboard             # initialize the workspace

./maven agent -m "Hello" # one-shot CLI
make run                 # interactive REPL
make gateway             # persistent gateway (channels + cron + heartbeat)
```

Full walkthrough: **[Get started](https://ageneralai.github.io/maven/getting-started/)**.

## Documentation

The full manual lives at **<https://ageneralai.github.io/maven/>**:

- [Concepts](https://ageneralai.github.io/maven/concepts/architecture/) — architecture, pipeline, plugins, sessions, streaming.
- [Guides](https://ageneralai.github.io/maven/guides/workspace/) — workspace, memory, skills, slash commands, cron, voice, subagents, ACP, hot reload.
- [Channels](https://ageneralai.github.io/maven/channels/) — Telegram, Feishu, WeCom, Matrix, WhatsApp, Web UI.
- [Deployment](https://ageneralai.github.io/maven/deployment/docker/) — Docker, proxy, OneCLI vault.
- [Reference](https://ageneralai.github.io/maven/reference/configuration/) — configuration schema, CLI, environment, HTTP API.

The doc source lives in [`docs/`](docs/) (MkDocs Material). To preview locally:

```bash
pip install -r requirements-docs.txt
mkdocs serve  # http://127.0.0.1:8000/
```

Pushes to `main` deploy to **`gh-pages`** via CI. See [contributing](https://ageneralai.github.io/maven/contributing/) for the workflow.

## Project layout

```text
cmd/maven/      CLI entry point
internal/
  gateway/      composition root (wire.go) and lifecycle
  kernel/       plugin-agnostic core (no plugin imports)
  plugins/      channel, tool, skill, voice, trigger, memory plugins
docs/           MkDocs source for the documentation site
scripts/        interactive setup
```

The architectural rule is one-way: `internal/kernel/` never imports `internal/plugins/`. Composition happens exactly once, in `internal/gateway/wire.go`. See [Architecture](https://ageneralai.github.io/maven/concepts/architecture/).

## Develop

```bash
make build         # binary
make test          # tests
make test-race     # race detector
make test-cover    # coverage report
make lint          # golangci-lint v2
make ci            # lint + vet + race tests
```

Docker:

```bash
make docker-up           # build and start
make docker-up-tunnel    # also start cloudflared tunnel for Feishu
make docker-down
```

Release binaries:

```bash
make build-release                                     # smaller, version-stamped binary
make package                                            # dist/maven-<os>-<arch>.gz
make package-all PLATFORMS="darwin/arm64 linux/amd64"   # multi-platform
```

## Security

- `~/.maven/config.json` is written with mode `0600`.
- `.gitignore` excludes `config.json`.
- For production, front the gateway with a reverse proxy and put credentials behind a vault — see [OneCLI](https://ageneralai.github.io/maven/deployment/onecli/).
- Never commit real API keys, bot tokens, or encryption keys to version control.

## License

[MIT](LICENSE).
