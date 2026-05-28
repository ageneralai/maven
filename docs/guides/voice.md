# Voice (Web UI)

The Web UI ships an optional voice mode: microphone capture in the browser → STT → agent → TTS → playback. Maven owns the realtime contract; provider plugins handle the actual STT and TTS.

## Wire diagram

```mermaid
flowchart LR
    Mic[Browser mic] -->|PCM s16le 16kHz| WS1[/ws/voice WebSocket/]
    WS1 --> STT[STT provider]
    STT -->|final transcript| Agent[Pipeline → Runtime]
    Agent -->|stream events| Seg[Sentence segmenter]
    Seg -->|sentence| TTS[TTS provider]
    TTS -->|PCM s16le 24kHz| WS2[same WebSocket]
    WS2 --> Speaker[Browser AudioBuffer queue]
```

The same WebSocket carries upstream microphone PCM and downstream synthesized PCM. A single-byte `0x00` sentinel from server → client flushes the audio queue (used when the client starts speaking again).

## Enable

```json
{
  "channels": {
    "web": {
      "enabled": true,
      "voice": {
        "enabled": true
      }
    }
  },
  "speech": {
    "sttProvider": "deepgram",
    "ttsProvider": "openai"
  }
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `channels.web.enabled` | `false` | Master Web UI toggle. |
| `channels.web.voice.enabled` | `false` | Browser voice transport. Requires Web UI enabled. |
| `speech.sttProvider` | `deepgram` | Speech-to-text provider. Only Deepgram is implemented. |
| `speech.ttsProvider` | `openai` | Text-to-speech provider. `openai` / `deepgram` / `elevenlabs` / `cartesia`. |

## Audio contracts

Both directions use **raw PCM, signed 16-bit little-endian**.

| Direction | Sample rate | Channels | Format |
|-----------|-------------|----------|--------|
| Browser → server (mic) | 16 kHz | mono | Linear PCM (downsampled by `audio-worklet-processor.js`). |
| Server → browser (TTS) | 24 kHz | mono | Raw PCM — no WAV/MP3 container. The client builds `AudioBuffer` from each chunk directly. |

Configure providers accordingly:

- Deepgram TTS: `container=none`, `encoding=linear16`, `sample_rate=24000`.
- ElevenLabs: `output_format=pcm_24000`.
- Cartesia: `container=raw`, `encoding=pcm_s16le`, `sample_rate=24000`.
- OpenAI TTS: `response_format=pcm`.

Headered formats will fail silently at the start of each utterance (the client treats the header bytes as samples).

## Credentials

All voice credentials come from environment variables (`kernel/voice.MergeKeys`):

| Provider | Env (or fallback) | Required extra |
|----------|-------------------|----------------|
| Deepgram | `MAVEN_DEEPGRAM_API_KEY`, then `DEEPGRAM_API_KEY` | — |
| OpenAI TTS | `OPENAI_API_KEY`, then `MAVEN_OPENAI_API_KEY`, then `provider.apiKey` (for OpenAI-type providers) | — |
| ElevenLabs | `MAVEN_ELEVENLABS_API_KEY`, then `ELEVENLABS_API_KEY` | `ELEVENLABS_VOICE_ID` |
| Cartesia | `MAVEN_CARTESIA_API_KEY`, then `CARTESIA_API_KEY` | `CARTESIA_VOICE_ID` |

Per-provider HTTPS proxies and Cartesia model/version overrides live under `speech.<provider>` in config; see [Reference: Configuration](../reference/configuration.md).

## Session model

The browser generates a UUID at page load and includes it as `?session=<uuid>` when dialing `/ws/voice`. The voice transport resolves that into a Maven session via `wsession.ResolveMavenSessionID`. Voice turns share session history with the Web UI text chat.

## Voice activity detection

A simple RMS threshold on the inbound PCM gates a `sess.Interrupt()` + queue flush. When the user starts talking, in-flight TTS aborts, the browser drops its playback queue, and the new transcript starts a fresh agent turn.

## Sentence segmentation

Streamed model output is buffered into a `kernel/voice` segmenter that takes complete sentences (ending in `.`, `!`, `?` before whitespace) up to a maximum of 800 runes. Each sentence becomes a TTS request. This trades a small first-utterance latency for fewer TTS round-trips and natural prosody.

## Provider files

| Plugin | Implements |
|--------|------------|
| `internal/plugins/voice/deepgram` | STT (live WebSocket) + TTS (HTTP streaming). |
| `internal/plugins/voice/openai` | TTS only. |
| `internal/plugins/voice/elevenlabs` | TTS only. Requires `ELEVENLABS_VOICE_ID`. |
| `internal/plugins/voice/cartesia` | TTS only. Requires `CARTESIA_VOICE_ID`. |

## Failure modes

- Missing credentials at startup → provider registration returns nil → factory error at first use ("openai api key is empty"). The Web UI voice button still appears but the WebSocket closes immediately.
- Browser without `AudioWorklet` support (very old browsers) → graceful failure, logged in console.
- Network drop during TTS → partial audio plays; the next sentence retries the HTTP call.

## Disable

Set `channels.web.voice.enabled = false`. The mic button stays hidden and `/ws/voice` is unregistered.
