# Web UI voice (STT / TTS)

Optional browser voice for the Web UI: microphone capture → STT → agent → TTS → WebSocket binary playback. Code: `internal/channel/web` (wire), `internal/voice` (session, factory, plugin list), `pkg/voice` (interfaces, `MergeKeys`, normalization), `pkg/{deepgram,cartesia,elevenlabs,openai}` (provider implementations + `plugin.Plugin`).

## Audio contract (TTS → browser)

`pkg/voice.TTS` streams **raw PCM**: **signed 16-bit little-endian, mono, 24 kHz**. Chunks are **contiguous samples with no container header** (no WAV, no MP3). The static client builds `AudioBuffer` from each chunk directly; it does not strip RIFF headers.

If a new TTS provider returns **WAV** (or any headered format) without changing the client, the first ~20 ms will be wrong. Configure providers for **raw linear PCM** (see implementations under `pkg/deepgram`, `pkg/openai`, `pkg/elevenlabs`, `pkg/cartesia`).

## Configuration

In `~/.maven/config.json` under `channels.web.voice`:

| Field | Purpose |
|-------|---------|
| `enabled` | Turn voice UI on (mic button, `/ws/voice`). |
| `sttProvider` | `deepgram` (only STT option today). |
| `ttsProvider` | `deepgram` · `openai` · `elevenlabs` · `cartesia` (default in examples: `openai`). |

Credentials resolve via env; see `pkg/voice.MergeKeys` and `internal/voice/factory.go`.

**Common env vars**

- **Deepgram**: `DEEPGRAM_API_KEY` or `MAVEN_DEEPGRAM_API_KEY`
- **OpenAI** (TTS/STT when applicable): `OPENAI_API_KEY` / `MAVEN_OPENAI_API_KEY`, or the gateway provider key from config for OpenAI-type providers
- **ElevenLabs**: `ELEVENLABS_API_KEY` / `MAVEN_ELEVENLABS_API_KEY`, **`ELEVENLABS_VOICE_ID`** (required for ElevenLabs TTS)
- **Cartesia**: `CARTESIA_API_KEY` / `MAVEN_CARTESIA_API_KEY`, **`CARTESIA_VOICE_ID`** (required); optional `CARTESIA_MODEL_ID`, `CARTESIA_API_VERSION`

See `config.example.json` → `channels.web.voice` and `.env.example` for placeholders.

## Related files

- `pkg/voice/voice.go` — `TTS` / `STT` interface doc (normative wire contract)
- `pkg/voice/keys.go` — `MergeKeys`
- `internal/voice/plugins.go` — default speech plugins for the gateway registry
- `internal/channel/web/static/index.html` — PCM enqueue / playback queue
