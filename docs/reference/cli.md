# CLI

`maven` is a single binary with several subcommands. They all read the same config (`~/.maven/config.json`) and workspace.

```text
maven <command> [flags]
```

## Daemon mode

`-d` / `--daemon` is a persistent flag on any subcommand. The parent re-execs the same binary without `-d`, starts it in a new session (`setsid`), redirects stdout and stderr to a log file, writes a PID file, prints both paths, and exits. The child runs the command normally until `SIGINT` or `SIGTERM`.

| Path | Contents |
|------|----------|
| `~/.maven/<subcmd>.log` | stdout + stderr (append) |
| `~/.maven/<subcmd>.pid` | child PID |

`<subcmd>` is the first non-flag argument (`agent`, `gateway`, …). If none is given, files are named `maven.log` / `maven.pid`.

```bash
maven gateway -d
maven agent --voice -d

tail -f ~/.maven/gateway.log
kill $(cat ~/.maven/gateway.pid)
```

Environment variables are inherited from the shell that ran `-d` — export voice or provider keys before starting a detached agent.

## `maven agent`

Run the agent in CLI mode. Without flags, drops into an interactive REPL. With `-m`, runs a single message and exits. With `--voice`, the REPL also listens on the microphone and speaks replies aloud.

```bash
maven agent                       # interactive REPL (keyboard)
maven agent --voice               # keyboard + mic + speaker, one shared session
maven agent --voice -d            # detached; log ~/.maven/agent.log
maven agent -m "Summarize today"  # one-shot
make run                          # alias for `maven agent`
```

Replies stream to stdout as `maven ▸`; the next `you ▸` line appears when the reply finishes (type on it or, with `--voice`, speak to fill it). Errors go to stderr. With `--voice`, replies are also spoken; speaking or typing again preempts the in-flight reply (barge-in, always on). By default (`speech.echoCancel: "pulse"`) Maven loads `module-echo-cancel` on startup and routes mic/speaker I/O through echo-cancelled PulseAudio devices via `parec`/`pacat`; it fails fast with the exact PulseAudio diagnostic if the module can't initialize. The default `pulse` also works on Android/Termux via the `speex` fallback (`webrtc` is unavailable there); headphones are recommended for clean speaker-only STT — see [Guides: Voice](../guides/voice.md#android--termux).

## `maven gateway`

Start the persistent gateway: channels, cron, heartbeat, memory consolidation.

```bash
maven gateway
maven gateway -d                  # detached; log ~/.maven/gateway.log
make gateway
```

The process runs until `SIGINT` or `SIGTERM`. With `gateway.hotReload = true` it also reloads on config save.

Healthy startup log lines:

```text
INFO gateway channels started channels=[telegram web]
INFO cron started jobs=2
INFO heartbeat started interval=30m0s
INFO mem-consolidate started interval=24h0m0s
INFO gateway running host=0.0.0.0 port=18790
```

## `maven onboard`

Initialize `~/.maven/config.json` (if missing) and the workspace layout under `agent.workspace`. Safe to re-run — it does not overwrite existing files.

```bash
maven onboard
make onboard
```

Creates:

- `~/.maven/config.json` from defaults (with empty provider key — fill it in).
- `<workspace>/AGENTS.md`, `SOUL.md`, `HEARTBEAT.md` (defaults from the binary).
- `<workspace>/memory/MEMORY.md` (empty).
- `<workspace>/skills/` (directory).

## `maven status`

Print runtime status: config path, provider type, enabled channels, cron job count, heartbeat interval. Useful for verifying environment.

```bash
maven status
make status
```

## `maven skills`

Manage workspace skills. See [Guides: Skills](../guides/skills.md) for what skills are.

```bash
maven skills list                    # human-friendly listing
maven skills list --json             # stable JSON
maven skills info <name>             # one skill
maven skills info <name> --json
maven skills check                   # validate the directory
maven skills check --json
```

The `--json` outputs share `{schemaVersion: 1, command, ok}` and add per-command fields.

## Build / package

These are `Makefile` targets, not subcommands:

| Target | Description |
|--------|-------------|
| `make build` | Plain debug binary. |
| `make build-release` | Trimmed binary with ldflags-stamped version. |
| `make package` | Build + gzip to `dist/maven-<os>-<arch>.gz`. |
| `make package-all PLATFORMS="…"` | Multi-platform package. Default `darwin/arm64 linux/amd64 linux/arm64`. |
| `make tunnel` | cloudflared tunnel for Feishu webhook (`channels.feishu.port`, default 9876). |
| `make docker-up` / `docker-down` / `docker-up-tunnel` | Compose lifecycle. |
| `make test` / `test-race` / `test-cover` | Tests. |
| `make lint` | golangci-lint v2. |
| `make ci` | lint + vet + race tests. |
| `make setup` | Interactive `~/.maven/config.json` generator. |

## Exit codes

| Code | Reason |
|------|--------|
| `0` | Clean shutdown. |
| `1` | Initial config load / validate error, or unrecoverable startup error. |
| `2+` | Reserved for future use. |

Most operational failures (channel auth, slow LLM, missing files) log at `error` and continue. The process only exits on `Apply` failure at boot.

## Logging

Output is `slog` text on TTY, JSON otherwise (`internal/kernel/log/log.New`). Levels: `debug`, `info`, `warn`, `error`. Default is `info`.

Set `logging.level` in `~/.maven/config.json` or `MAVEN_LOG_LEVEL` in the environment. Env always wins — it is applied on every `LoadConfig` call, including hot reload. With `gateway.hotReload = true`, changing `logging.level` in config takes effect on reload only when `MAVEN_LOG_LEVEL` is not set.
