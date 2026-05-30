# Voice

Maven speaks and listens through a single, transport-agnostic conversation core (`internal/kernel/converse`). The core deals only in **text turns**; microphones, speakers, terminals, and browsers are pluggable modalities behind two one-method ports:

- `Source` — produces user text (keyboard, or mic → STT).
- `Sink` — renders an assistant reply stream (screen, or TTS → speaker).

Speech-to-text and text-to-speech are just codecs at the edge; the core knows nothing about audio. Two transports use it today: the **Web UI** (browser mic/speaker) and the **CLI REPL** (local or Android mic/speaker).

## CLI REPL (and Android)

`maven agent --voice` runs a multimodal REPL: you can type **and** speak at the same time, and replies are simultaneously printed to the terminal and spoken aloud. Typed and spoken turns share one `cli` session, so the conversation is continuous regardless of modality. Speaking or typing again mid-reply **preempts** the in-flight turn (barge-in) — always on, no toggle.

**PulseAudio is the default backend** for CLI voice. With `speech.echoCancel: "pulse"` (default) Maven loads `module-echo-cancel` on startup — trying `aec_method=webrtc` (the same WebRTC algorithm browsers use), then `speex` — and routes capture/playback through dedicated echo-cancelled devices (`maven_echocancel_source` / `maven_echocancel_sink`), so the mic no longer picks up the agent's TTS and barge-in works without feedback loops. Maven distinguishes two failures: PulseAudio unreachable (`pulseaudio unavailable`) vs the module failing to initialize (`echo-cancel module unavailable`). There is no silent fallback — it exits with the exact PulseAudio diagnostic.

Install PulseAudio on your platform.

### Wake phrase (conversation window)

By default `--voice` listens continuously — every transcript becomes a turn. Set a wake phrase to gate turns Siri-style: the phrase opens a conversation window, turns flow until the window idles out (default 8s), then the gate re-arms.

```bash
maven agent --voice --wake-phrase "hey maven"
```

Or in config (the `--wake-phrase` flag overrides it):

```json
{ "speech": { "wake": { "phrase": "hey maven", "timeoutMs": 8000 } } }
```

The phrase is matched on normalized leading words. Say it alone ("hey maven") and the wake utterance is sent to the agent as a greeting so it responds; say it with a command in one breath ("hey maven what's the weather") and the wake words are stripped, sending just "what's the weather". Either way the conversation window opens and stays open until it idles out. The idle timer runs only **between** turns — it pauses while Maven is generating or playing a reply, so a long story cannot expire the window mid-speech. After Maven finishes, you get the full timeout to respond; barge-in while Maven is speaking always reaches the agent. While dormant, speech is ignored; the keyboard always works regardless of wake state. An empty phrase is stock always-on.

### Android

Install the **`android/arm64`** release binary (not `linux/arm64`) with the same script as other platforms:

```bash
curl -fsSL https://ageneral.ai/maven/install.sh | bash
```

Voice also needs a working LLM key in `~/.maven/config.json` (`provider.apiKey`) from `maven onboard` — same as desktop.

#### 1. Packages and permissions

Install PulseAudio via your package manager and grant **Microphone** permission in Android Settings for your terminal app.

#### 2. PulseAudio

On first `--voice` run, Maven reconciles PulseAudio: one daemon, mic source (`module-sles-source`) when missing, then echo-cancel setup. No manual `pulseaudio --start` or `pactl load-module` required.

Verify after startup:

```bash
pactl list sources short   # expect OpenSL_ES_source
pactl list sinks short     # expect AAudio_sink or similar
```

Quick mic check (~1 s of PCM should be non-zero size):

```bash
timeout 1 parec --format=s16le --rate=16000 --channels=1 | wc -c
```

#### 3. Config

Leave `speech.echoCancel` at the default **`"pulse"`**. On Android `module-echo-cancel`'s `webrtc` backend fails to initialize, but the `speex` fallback loads and runs, so Maven gets working OS echo cancellation. Use `"off"` only to disable echo cancellation entirely (raw passthrough, e.g. with headphones).

```json
{
  "speech": {
    "sttProvider": "deepgram",
    "ttsProvider": "cartesia",
    "cartesia": {
      "voiceId": "your-voice-id"
    }
  }
}
```

Default `parec` / `pacat` commands work once PulseAudio has real source and sink devices; no custom `speech.capture` / `speech.playback` needed unless you prefer otherwise.

#### 4. Voice credentials

STT and TTS keys are separate from your LLM provider key. Export before running:

```bash
export DEEPGRAM_API_KEY=...
export CARTESIA_API_KEY=...
export CARTESIA_VOICE_ID=...   # optional if already in config
```

See [Credentials](#credentials) for all providers and env fallbacks.

#### 5. Run

```bash
maven agent --voice
```

Use **headphones** for speaker-only use. The `speex` fallback on Android suppresses only ~11 dB of echo — enough to calm the VAD against false barge-in, but not enough for clean speaker-only STT. (Linux's `webrtc` backend is much stronger and is genuinely clean speaker-only.)

#### Android checklist

| Step | Required for `--voice` |
|------|------------------------|
| `install.sh` or `maven-android-arm64` binary | Yes |
| Terminal app mic permission (Android Settings) | Yes |
| PulseAudio installed | Yes |
| `speech.echoCancel: "pulse"` (default; speex fallback) | Yes |
| `DEEPGRAM_API_KEY` + TTS provider key | Yes |
| LLM `provider.apiKey` | Yes |
| Headphones (clean speaker-only STT) | Strongly recommended |

#### Echo cancellation on Android

Default `pulse` loads `module-echo-cancel`: `webrtc` fails to initialize on Android, so Maven falls back to `speex`, which loads and runs (~11 dB suppression). Maven first reconciles PulseAudio (single daemon + mic source) so the module has a master source. `off` disables echo cancellation and PulseAudio module management entirely; Maven still runs `parec` / `pacat` for capture and playback — see the CLI diagram and PCM table below.

```mermaid
flowchart LR
    KB[Keyboard] --> CORE
    MIC[Echo-cancel mic<br/>parec → AEC source] -->|PCM s16le 16kHz| STT[STT provider] --> CORE[converse core]
    CORE -->|RunStream deltas| SCR[Screen maven ▸]
    CORE -->|sentences| TTS[TTS provider] -->|PCM s16le 24kHz| SPK[Echo-cancel sink<br/>pacat → AEC sink]
```

The CLI REPL uses one transcript shape (keyboard-only or `--voice`): after each `maven ▸` reply, the next line is `you ▸` (type on it or speak to populate via STT). Empty Enter does not add another prompt.

Audio device I/O is delegated to external processes that stream **raw PCM** over stdout/stdin — no CGO, no ALSA/PulseAudio linkage in the binary. The same binary therefore runs unchanged on a Linux desktop and on Android; only the configured commands differ.

| Direction | Format | Default command |
|-----------|--------|-----------------|
| Mic → STT | PCM s16le, 16 kHz, mono | `parec --format=s16le --rate=16000 --channels=1 --latency-msec=50 --device=maven_echocancel_source` |
| TTS → Speaker | PCM s16le, 24 kHz, mono | `pacat --format=s16le --rate=24000 --channels=1 --latency-msec=100 --device=maven_echocancel_sink` |

The low `--latency-msec` values are deliberate: small mic fragments let the VAD detect speech onset within ~50 ms, and a bounded playback buffer means killing `pacat` on barge-in silences the speaker near-instantly (matching the browser's queue flush). Without them, PulseAudio's default buffering delays both onset detection and the barge-in cut.

Echo cancellation is selected by `speech.echoCancel`. Default `pulse` loads `module-echo-cancel` (webrtc, then speex) under internal device names, routes capture/playback through it so the agent never hears itself, and unloads it on exit — recommended on all platforms, including Android where it falls back to speex. `off` skips PulseAudio module management and runs `speech.capture` / `speech.playback` (command + args) verbatim with no forced device — the explicit no-AEC mode, e.g. with headphones. PulseAudio provides `parec`, `pacat`, and `pactl`.

```json
{
  "speech": {
    "sttProvider": "deepgram",
    "ttsProvider": "openai",
    "capture":  { "command": "parec", "args": ["--format=s16le", "--rate=16000", "--channels=1", "--latency-msec=50"] },
    "playback": { "command": "pacat", "args": ["--format=s16le", "--rate=24000", "--channels=1", "--latency-msec=100"] }
  }
}
```

STT/TTS providers and credentials are identical to the Web UI (see below). The CLI builds its own minimal voice provider registry; no gateway or channels are required.

## Web UI

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

A simple RMS threshold on the inbound PCM triggers barge-in: in-flight TTS aborts, the browser drops its playback queue, and the new transcript starts a fresh agent turn.

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
