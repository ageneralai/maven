<p>
  <img src="https://github.com/ageneralai/maven/blob/main/website/public/images/panorama_photosphere_120dp_000_FILL0_wght500_GRAD0_opsz48.png?raw=true" alt="Maven" width="120">
</p>

# Maven

Personal AI assistant built on [ageneral-agents-go](https://github.com/ageneralai/ageneral-agents-go).

## Features

- **CLI Agent** - Single message or interactive REPL mode
- **Gateway** - Full orchestration: channels + cron + heartbeat
- **Telegram Channel** - Receive and send messages via Telegram bot (text + image + document); optional **streaming** in private chats (Bot API `sendMessageDraft`) when `channels.telegram.streaming` is true and the model streams tokens
- **Feishu Channel** - Receive and send messages via Feishu (Lark) bot
- **WeCom Channel** - Receive inbound messages and send markdown replies via WeCom intelligent bot API mode
- **WhatsApp Channel** - Receive and send messages via WhatsApp (QR code login)
- **Web UI** - Browser-based chat interface with WebSocket (responsive, PC + mobile)
- **Multi-Provider** - Support for Anthropic and OpenAI models
- **Multimodal** - Image recognition and document processing
- **Cron Jobs** - Scheduled tasks with JSON persistence
- **ACP delegation** - Optional **`DelegateTask`** tool: Maven spawns configured ACP coding agents (stdio); see [docs/acp.md](docs/acp.md)
- **Heartbeat** - Periodic tasks from HEARTBEAT.md
- **Memory** - Long-term (MEMORY.md) + daily memories
- **Skills** - Custom skill loading from workspace
- **Gateway config hot reload** - Optional watch on `~/.maven/config.json` (`gateway.hotReload`) to reload channels and runtime without restarting the process

## Quick Start

```bash
# Build
make build

# Build smaller release binary
make build-release

# Interactive config setup
make setup

# Or initialize config and workspace manually
make onboard

# Set your API key
export MAVEN_API_KEY=your-api-key

# Run agent (single message)
./maven agent -m "Hello"

# Run agent (REPL mode)
make run

# Start gateway (channels + cron + heartbeat)
make gateway
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build binary |
| `make build-release` | Build optimized binary with `-trimpath -ldflags="-s -w"` |
| `make package` | Package optimized binary to `dist/maven-<os>-<arch>.gz` |
| `make package-all` | Package optimized binaries for `darwin/arm64 linux/amd64 linux/arm64` |
| `make run` | Run agent REPL |
| `make gateway` | Start gateway (channels + cron + heartbeat) |
| `make onboard` | Initialize config and workspace |
| `make status` | Show maven status |
| `make setup` | Interactive config setup (generates `~/.maven/config.json`) |
| `make tunnel` | Start cloudflared tunnel for Feishu webhook |
| `make test` | Run tests |
| `make test-race` | Run tests with race detection |
| `make test-cover` | Run tests with coverage report |
| `make docker-up` | Docker build and start |
| `make docker-up-tunnel` | Docker start with cloudflared tunnel |
| `make docker-down` | Docker stop |
| `make lint` | Run golangci-lint |

## Binary Packaging

```bash
# Smaller binary for release
make build-release

# Create compressed package (.gz)
make package

# Build and package default multi-platform artifacts
make package-all

# Or customize target platforms
make package-all PLATFORMS="linux/amd64 linux/arm64"
```

`make package` creates a single archive `dist/maven-<os>-<arch>.gz`.
`make package-all` creates multiple archives under `dist/`, suitable for release distribution and low-bandwidth deployment.

## Configuration

Run `make setup` for interactive config, or copy `config.example.json` to `~/.maven/config.json`:

```json
{
  "provider": {
    "type": "anthropic",
    "apiKey": "your-api-key",
    "baseUrl": ""
  },
  "agent": {
    "model": "claude-sonnet-4-5-20250929"
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "your-bot-token",
      "allowFrom": ["123456789"],
      "streaming": false
    },
    "feishu": {
      "enabled": true,
      "appId": "cli_xxx",
      "appSecret": "your-app-secret",
      "verificationToken": "your-verification-token",
      "port": 9876,
      "allowFrom": []
    },
    "wecom": {
      "enabled": true,
      "token": "your-token",
      "encodingAESKey": "your-43-char-encoding-aes-key",
      "receiveId": "",
      "port": 9886,
      "allowFrom": ["zhangsan"]
    },
    "whatsapp": {
      "enabled": true,
      "allowFrom": []
    },
    "web": {
      "enabled": true,
      "allowFrom": [],
      "voice": {
        "enabled": false,
        "sttProvider": "deepgram",
        "ttsProvider": "openai"
      }
    }
  },
  "tools": {
    "restrictToWorkspace": true,
    "acp": {
      "enabled": false,
      "agents": {
        "claude": {
          "command": "npx",
          "args": ["-y", "@zed-industries/claude-code-acp@latest"]
        }
      }
    }
  },
  "skills": {
    "enabled": true,
    "dir": ""
  },
  "autoCompact": {
    "enabled": false,
    "threshold": 0.8,
    "preserveCount": 5
  },
  "gateway": {
    "host": "0.0.0.0",
    "port": 18790,
    "hotReload": false,
    "reloadDebounceMs": 800,
    "cron": {
      "maxConcurrentRuns": 1
    }
  }
}
```

### Gateway (`host`, `port`, hot reload, cron queue)

- **`gateway.host`** / **`gateway.port`**: HTTP bind for Web UI and channel webhooks (defaults align with `config.example.json`).
- **`gateway.hotReload`**: when `true`, edits to `~/.maven/config.json` (after a short debounce) trigger a reload; the log line `[gateway] reloaded; …` confirms success. **`agent.workspace` cannot change** on reload (restart required).
- **`gateway.reloadDebounceMs`**: debounce in milliseconds before reload runs after the file changes; `0` uses an internal default (800ms).
- **`gateway.cron.maxConcurrentRuns`**: max concurrent **cron** agent turns in the gateway process (default **1** if omitted). Heartbeat uses its own **one-slot** try-once queue. Changing **`maxConcurrentRuns`** requires a **gateway restart** (not hot reload). See `internal/gateway/doc.go`.

See `config.example.json` for the full schema.

### Tools / ACP (`tools`, `tools.acp`)

- **`tools.restrictToWorkspace`**: when true, sandbox-style tools (and **`DelegateTask`** `cwd` / ACP FS hooks) must stay under **`agent.workspace`**.
- **`tools.acp`**: optional; when **`enabled`** is true and **`agents`** has valid entries (`command` non-empty per key), the gateway registers **`DelegateTask`**. Agent binaries and args come **only from config**, never from model-supplied shell commands.

Full detail and examples: [docs/acp.md](docs/acp.md).

### Auto-compact (`autoCompact`)

Context rotation when the model nears its window is **off by default** (`enabled: false`). Set **`enabled`** to **`true`** to opt in.

- **`threshold`**: trigger when estimated context usage crosses this fraction of the window (greater than `0`, up to `1`). Default in generated defaults is **`0.8`** when you enable auto-compact.
- **`preserveCount`**: how many recent turns to keep across a compact boundary (default **`5`** in generated defaults).

Validation rules are enforced in `config.Validate()` when `enabled` is true.

### Telegram (`channels.telegram`)

- **`streaming`** (optional, default `false`): when `true`, the gateway uses the streaming pipeline and Telegram shows progressive output. **Private** DMs use Bot API **`sendMessageDraft`** (then a final `sendMessage`). **Groups/supergroups** use placeholder + `editMessageText` (draft API is private-chat only). For visible streaming, the LLM provider must actually return streamed chunks (`stream: true` / SSE); otherwise you still get one burst when the model finishes.

### Provider Types

| Type | Config | Env Vars |
|------|--------|----------|
| `anthropic` (default) | `"type": "anthropic"` | `MAVEN_API_KEY`, `ANTHROPIC_API_KEY` |
| `openai` | `"type": "openai"` | `OPENAI_API_KEY` |

When using OpenAI, set the model to an OpenAI model name (e.g., `gpt-4o`).

### Environment Variables

| Variable | Description |
|----------|-------------|
| `MAVEN_API_KEY` | API key (any provider) |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OPENAI_API_KEY` | OpenAI API key (auto-sets type to openai) |
| `MAVEN_BASE_URL` | Custom API base URL |
| `MAVEN_TELEGRAM_TOKEN` | Telegram bot token |
| `MAVEN_FEISHU_APP_ID` | Feishu app ID |
| `MAVEN_FEISHU_APP_SECRET` | Feishu app secret |
| `MAVEN_WECOM_TOKEN` | WeCom intelligent bot callback token |
| `MAVEN_WECOM_ENCODING_AES_KEY` | WeCom intelligent bot callback EncodingAESKey |
| `MAVEN_WECOM_RECEIVE_ID` | Optional receive ID for strict decrypt validation |

> Prefer environment variables over config files for sensitive values like API keys.

### Skills

`maven` supports local skills loaded from `SKILL.md` files.

- `skills.enabled`: enable or disable skills (default `true`)
- `skills.dir`: custom skills directory; empty means `<agent.workspace>/skills`
- `maven onboard` automatically creates the default skills directory

Skill layout:

```text
<workspace>/skills/<skill-name>/SKILL.md
```

Minimal `SKILL.md` example:

```markdown
---
name: writer
description: writing helper
keywords: [write, draft]
---
# Writer
Use this skill for writing tasks.
```

After changing skills, either enable **`gateway.hotReload`** and save `~/.maven/config.json` so the gateway reloads skill registration, or restart **`maven gateway`**.

Skill diagnostics:

```bash
./maven skills list
./maven skills info writer
./maven skills check
./maven skills list --json
```

JSON contract (stable):

- Common fields for all `--json` outputs:
  - `schemaVersion` (int, currently `1`)
  - `command` (`skills.list` | `skills.info` | `skills.check`)
  - `ok` (bool)
- `skills list --json`:
  - `enabled`, `dir`, `loaded`, `skills[]`
  - `skills[]` item: `name`, `description`, `keywords[]`
- `skills info <name> --json`:
  - `name`, `description`, `dir`, `keywords[]`, `source`, `preview`
  - optional: `handlerError`
- `skills check --json`:
  - `enabled`, `dir`, `skillFolders`, `loaded`, `missingSkillMD[]`, `result`
  - optional: `note`

## Channel Setup

### Telegram

See [docs/telegram-setup.md](docs/telegram-setup.md) for detailed setup guide.

Quick steps:
1. Create a bot via [@BotFather](https://t.me/BotFather) on Telegram
2. Set `token` in config or `MAVEN_TELEGRAM_TOKEN` env var
3. For progressive replies in **private** chats, set `"streaming": true` under `channels.telegram` (see **Telegram** under Configuration above); your model endpoint must stream tokens
4. Run `make gateway`

### Feishu (Lark)

See [docs/feishu-setup.md](docs/feishu-setup.md) for detailed setup guide.

Quick steps:
1. Create an app at [Feishu Open Platform](https://open.feishu.cn/app)
2. Enable **Bot** capability
3. Add permissions: `im:message`, `im:message:send_as_bot`
4. Configure Event Subscription URL: `https://your-domain/feishu/webhook`
5. Subscribe to event: `im.message.receive_v1`
6. Set `appId`, `appSecret`, `verificationToken` in config
7. Run `make gateway` and `make tunnel` (for public webhook URL)

### WeCom

See [docs/wecom-setup.md](docs/wecom-setup.md) for detailed setup guide.

Quick steps:
1. Create a WeCom intelligent bot in API mode and get `token`, `encodingAESKey`
2. Configure callback URL: `https://your-domain/wecom/bot`
3. Set `token` and `encodingAESKey` in both WeCom console and maven config
4. Optionally set `receiveId` if you need strict decrypt receive-id validation
5. Optional: set `allowFrom` to your user ID(s) as whitelist (if unset/empty, inbound from all users is allowed)
6. Run `make gateway`

WeCom notes:
- Outbound is **reactive only** (passive reply URLs from inbound). **Cron jobs with `deliver: true` skip WeCom** and log a skip message; use another channel for proactive delivery.
- Outbound uses `response_url` and sends `markdown` payloads
- `response_url` is short-lived (often single-use); delayed or repeated replies may fail
- Outbound markdown content over 20480 bytes is truncated

### WhatsApp

Quick steps:
1. Set `"whatsapp": {"enabled": true}` in config
2. Run `make gateway`
3. Scan the QR code displayed in terminal with your WhatsApp
4. Session is stored locally in SQLite (auto-reconnects on restart)

### Web UI

Quick steps:
1. Set `"web": {"enabled": true}` in config
2. Run `make gateway`
3. Open `http://localhost:18790` in your browser (PC or mobile)

Features:
- Responsive design (PC + mobile)
- Dark mode (follows system preference)
- WebSocket real-time communication
- Markdown rendering (code blocks, bold, italic, links)
- Auto-reconnect on connection loss

## Docker Deployment

### Build and Run

```bash
docker build -t maven .

docker run -d \
  -e MAVEN_API_KEY=your-api-key \
  -e MAVEN_TELEGRAM_TOKEN=your-token \
  -p 18790:18790 \
  -p 9876:9876 \
  -p 9886:9886 \
  -v maven-data:/root/.maven \
  maven
```

### Docker Compose

```bash
# Create .env from example
cp .env.example .env
# Edit .env with your credentials

# Start gateway
docker compose up -d

# Start with cloudflared tunnel (for Feishu webhook)
docker compose --profile tunnel up -d

# View logs
docker compose logs -f maven
```

### Cloudflared Tunnel

For Feishu webhooks, you need a public URL:

```bash
# Temporary tunnel (dev)
make tunnel

# Or via docker compose
docker compose --profile tunnel up -d
docker compose logs tunnel | grep trycloudflare
```

Set the output URL + `/feishu/webhook` as your Feishu event subscription URL.

## Security

- `~/.maven/config.json` is set to `chmod 600` (owner read/write only)
- `.gitignore` excludes `config.json`
- Use environment variables for sensitive values in CI/CD and production
- Never commit real API keys or tokens to version control

## Testing

```bash
make test            # Run all tests
make test-race       # Run with race detection
make test-cover      # Run with coverage report
make lint            # Run golangci-lint
```

## Documentation

Published docs: [https://ageneralai.github.io/maven/](https://ageneralai.github.io/maven/)

Source lives in `docs/` (MkDocs + Material). To preview or publish:

```bash
pip install -r requirements-docs.txt
mkdocs serve                    # http://127.0.0.1:8000/
mkdocs gh-deploy --force        # push to gh-pages (maintainers)
```

See [docs/contributing.md](docs/contributing.md) for the full contributor workflow (nav in `mkdocs.yml`, what to commit, deploy permissions).

## License

MIT
