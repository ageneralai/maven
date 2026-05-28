# Get started

This page takes you from clone to a running gateway in under five minutes.

## Prerequisites

- **Go 1.25+** (build only — release binaries have no runtime Go dependency)
- An **Anthropic** or **OpenAI** API key
- Optional: tokens for any channel you want to enable (see [Channels](channels/index.md))

## 1. Get the binary

=== "Build from source"

    ```bash
    git clone https://github.com/ageneralai/maven.git
    cd maven
    make build
    ```

    Output: `./maven`.

=== "Release archive"

    Pre-built artifacts for `darwin/arm64`, `linux/amd64`, `linux/arm64` are produced by `make package-all` and attached to GitHub Releases:

    ```bash
    curl -L https://github.com/ageneralai/maven/releases/latest/download/maven-linux-amd64.gz \
      | gunzip > maven
    chmod +x maven
    ```

=== "Docker"

    ```bash
    docker build -t maven .
    docker run -d -v maven-data:/root/.maven -p 18790:18790 maven
    ```

    See [Deployment: Docker](deployment/docker.md).

## 2. Configure

Run the interactive setup, which writes `~/.maven/config.json` (mode `0600`):

```bash
make setup
```

Or copy [`config.example.json`](https://github.com/ageneralai/maven/blob/main/config.example.json) to `~/.maven/config.json` and edit by hand. The minimal config needs a provider key:

```json
{
  "provider": {
    "type": "anthropic",
    "apiKey": "sk-ant-..."
  },
  "agent": {
    "workspace": "~/.maven/workspace",
    "model": "claude-sonnet-4-5-20250929",
    "maxTokens": 8192,
    "maxToolIterations": 20
  }
}
```

Full schema: [Reference: Configuration](reference/configuration.md).

## 3. Initialize the workspace

```bash
make onboard
```

This creates the default workspace at `~/.maven/workspace` with `AGENTS.md`, `SOUL.md`, `HEARTBEAT.md`, an empty `memory/MEMORY.md`, and a `skills/` directory. See [Guides: Workspace](guides/workspace.md).

## 4. Try the CLI

One-shot:

```bash
./maven agent -m "What can you do?"
```

Interactive REPL:

```bash
make run
```

## 5. Start the gateway

Run channels, cron, heartbeat, and memory consolidation in one process:

```bash
make gateway
```

Healthy startup logs look like:

```text
INFO gateway running host=0.0.0.0 port=18790
INFO gateway channels started channels=[web]
INFO cron started jobs=0
INFO heartbeat started interval=30m0s
```

The default Web UI is reachable at `http://localhost:18790` (enable it under `channels.web` in config).

## Next steps

- **Wire a channel.** Pick one and follow its setup guide: [Telegram](channels/telegram.md), [Feishu](channels/feishu.md), [WeCom](channels/wecom.md), [Matrix](channels/matrix.md), [WhatsApp](channels/whatsapp.md), [Web UI](channels/web.md).
- **Schedule a job.** Use the `cron-schedule` agent tool or `/cron-add` slash command — see [Guides: Cron jobs](guides/cron.md).
- **Understand the model.** Read [Concepts: Architecture](concepts/architecture.md) and [Concepts: Pipeline](concepts/pipeline.md).
- **Run on a server.** [Deployment: Docker](deployment/docker.md), [Deployment: Proxy](deployment/proxy.md).
