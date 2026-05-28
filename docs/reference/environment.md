# Environment variables

Maven loads config from `~/.maven/config.json` (or `MAVEN_CONFIG`). Environment variables overlay matching config fields at load time — if an env var is set, it overrides the corresponding config value.

## Config path

| Variable | Purpose |
|----------|---------|
| `MAVEN_CONFIG` | Path to config file (default: `~/.maven/config.json`) |

## Provider credentials

| Variable | Purpose |
|----------|---------|
| `ANTHROPIC_API_KEY` | LLM provider API key when `provider.type` is `anthropic` (default) |
| `OPENAI_API_KEY` | LLM provider API key when `provider.type` is `openai`; also used for OpenAI TTS |
| `DEEPGRAM_API_KEY` | Deepgram STT/TTS API key |
| `ELEVENLABS_API_KEY` | ElevenLabs TTS API key |
| `ELEVENLABS_VOICE_ID` | ElevenLabs voice ID |
| `CARTESIA_API_KEY` | Cartesia TTS API key |
| `CARTESIA_VOICE_ID` | Cartesia voice ID |

## Networking

| Variable | Purpose | Notes |
|----------|---------|-------|
| `HTTPS_PROXY` | Proxy URL for HTTPS traffic. | `http://`, `https://`, `socks5://`. |
| `HTTP_PROXY` | Proxy URL for HTTP. | Fallback when `HTTPS_PROXY` unset. |
| `NO_PROXY` | Comma-separated hosts to bypass. | Standard Go semantics. |
| `SSL_CERT_FILE` | CA bundle path for TLS trust. | Required for MITM proxies like [OneCLI](../deployment/onecli.md). |

These are read by Go's `http.DefaultTransport.Proxy` and `crypto/x509`. Maven has no proxy fields in config.

## Build-time variables (Makefile)

Used by `make build-release` / `make package*`:

| Variable | Default | Description |
|----------|---------|-------------|
| `VERSION` | `git describe --tags --match 'v*' --always --dirty` or `dev` | Stamped into the binary. |
| `COMMIT` | `git rev-parse --short HEAD` or `none` | Stamped into the binary. |
| `DATE` | `date -u +%Y-%m-%dT%H:%M:%SZ` | Stamped into the binary. |
| `PLATFORMS` | `darwin/arm64 linux/amd64 linux/arm64` | `make package-all` targets. |
| `FEISHU_PORT` | `9876` | Port `make tunnel` exposes. |

## What's *not* an env var

- Channel tokens (Telegram bot token, Matrix access token, WeCom encoding key, etc.) — set in `channels.<name>` config.
- Workspace path — set in `agent.workspace` config.

## `HOME`

`config.ConfigDir()` derives `~/.maven` from `os.UserHomeDir()`, which honors `HOME` on Unix. Run as a different user or set `HOME` to relocate everything (config, sessions, workspace defaults).
