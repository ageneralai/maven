# Web UI voice (STT / TTS)

Optional browser voice for the Web UI: microphone capture → STT → agent → TTS → WebSocket binary playback. Code: `internal/plugins/channel/web/voice/transport.go` (wire), `internal/kernel/voice` (session coordinator, factory, provider interfaces).

## Audio contract (TTS → browser)

`internal/kernel/voice.TTS` streams **raw PCM**: **signed 16-bit little-endian, mono, 24 kHz**. Chunks are **contiguous samples with no container header** (no WAV, no MP3). The static client builds `AudioBuffer` from each chunk directly; it does not strip RIFF headers.

If a new TTS provider returns **WAV** (or any headered format) without changing the client, the first ~20 ms will be wrong. Configure providers for **raw linear PCM** (see implementations under `internal/plugins/voice/`).

## Configuration

**Platform speech providers** (`~/.maven/config.json` → `speech`):

| Field | Purpose |
|-------|---------|
| `sttProvider` | `deepgram` (default). Shared by any channel that needs STT. |
| `ttsProvider` | `openai` (default) · `deepgram` · `elevenlabs` · `cartesia`. |

**Web UI voice transport** (`channels.web.voice`):

| Field | Purpose |
|-------|---------|
| `enabled` | Turn browser voice UI on (mic button, `/ws/voice`). Uses `speech.*` providers. |

Credentials resolve via environment variables; see `internal/kernel/voice/keys.go` (`MergeKeys`).

**Common env vars**

- **Deepgram**: `DEEPGRAM_API_KEY` or `MAVEN_DEEPGRAM_API_KEY`
- **OpenAI** (TTS): `OPENAI_API_KEY` / `MAVEN_OPENAI_API_KEY`, or the gateway provider key from config for OpenAI-type providers
- **ElevenLabs**: `ELEVENLABS_API_KEY` / `MAVEN_ELEVENLABS_API_KEY`, **`ELEVENLABS_VOICE_ID`** (required)
- **Cartesia**: `CARTESIA_API_KEY` / `MAVEN_CARTESIA_API_KEY`, **`CARTESIA_VOICE_ID`** (required); optional `CARTESIA_MODEL_ID`, `CARTESIA_API_VERSION`

See `config.example.json` → `speech` and `.env.example` for placeholders.

## Related files

- `internal/kernel/voice/voice.go` — `TTS` / `STT` interface definitions
- `internal/kernel/voice/keys.go` — `MergeKeys` (credential resolution)
- `internal/kernel/voice/providers.go` — provider name resolution from `speech` config
- `internal/kernel/voice/factory.go` — `NewSTT` / `NewTTS` factory
- `internal/kernel/voice/session.go` — `Session` coordinator (mic STT + agent TTS without transport)
- `internal/plugins/channel/web/voice/transport.go` — WebSocket voice transport
- `internal/plugins/channel/web/static/index.html` — PCM enqueue / playback queue
- `internal/plugins/voice/deepgram/` — Deepgram STT + TTS implementation
- `internal/plugins/voice/openai/` — OpenAI TTS implementation
- `internal/plugins/voice/elevenlabs/` — ElevenLabs TTS implementation
- `internal/plugins/voice/cartesia/` — Cartesia TTS implementation
