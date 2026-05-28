# Environment variables

Maven prefers config over environment for most things, but a handful of values live in env by design — primarily for credentials handed off to a vault and for per-process egress.

## Networking

| Variable | Purpose | Notes |
|----------|---------|-------|
| `HTTPS_PROXY` | Proxy URL for HTTPS traffic. | `http://`, `https://`, `socks5://`. |
| `HTTP_PROXY` | Proxy URL for HTTP. | Fallback when `HTTPS_PROXY` unset. |
| `NO_PROXY` | Comma-separated hosts to bypass. | Standard Go semantics. |
| `SSL_CERT_FILE` | CA bundle path for TLS trust. | Required for MITM proxies like [OneCLI](../deployment/onecli.md). |

These are read by Go's `http.DefaultTransport.Proxy` and `crypto/x509`. Maven has no proxy fields in config.

## Voice provider credentials

`internal/kernel/voice/keys.go` resolves credentials at gateway start:

| Env | Fallback | Used by |
|-----|----------|---------|
| `MAVEN_DEEPGRAM_API_KEY` | `DEEPGRAM_API_KEY` | Deepgram STT + TTS. |
| `MAVEN_ELEVENLABS_API_KEY` | `ELEVENLABS_API_KEY` | ElevenLabs TTS. |
| `MAVEN_CARTESIA_API_KEY` | `CARTESIA_API_KEY` | Cartesia TTS. |
| `OPENAI_API_KEY` | `MAVEN_OPENAI_API_KEY` then `provider.apiKey` (when provider type is OpenAI) | OpenAI TTS. |

Voice-specific required IDs:

| Env | Required when | Purpose |
|-----|---------------|---------|
| `ELEVENLABS_VOICE_ID` | `speech.ttsProvider = "elevenlabs"` | ElevenLabs voice id. |
| `CARTESIA_VOICE_ID` | `speech.ttsProvider = "cartesia"` | Cartesia voice id. |

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

- LLM keys for the model provider — set in `provider.apiKey`. Use [OneCLI](../deployment/onecli.md) if you want them out of config.
- Channel tokens (Telegram bot token, Matrix access token, WeCom encoding key, etc.) — set in `channels.<name>` config.
- Workspace path — set in `agent.workspace` config.

There is no `MAVEN_CONFIG` env override today; the path is fixed at `~/.maven/config.json` via `config.ConfigPath()`. Override by running as a different user (`HOME`).

## `HOME`

`config.ConfigDir()` derives `~/.maven` from `os.UserHomeDir()`, which honors `HOME` on Unix. Run as a different user or set `HOME` to relocate everything (config, sessions, workspace defaults).
