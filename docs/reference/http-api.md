# HTTP API

The gateway listens on `gateway.host:gateway.port` (default `0.0.0.0:18790`) and serves a small set of HTTP endpoints. Most channels expose their own webhook ports separately (Feishu `9876`, WeCom `9886`).

## Endpoint summary

| Path | Method | Channel/feature | Description |
|------|--------|-----------------|-------------|
| `/` | GET | Web UI | Embedded SPA. |
| `/web/config` | GET | Web UI | JSON `{ voiceEnabled }`. |
| `/ws` | WebSocket | Web UI | Text chat. |
| `/ws/voice` | WebSocket | Web UI voice | Realtime PCM voice. |
| `/v1/responses` | POST | Web UI | OpenAI Responses-compatible SSE. |
| `/feishu/webhook` | POST | Feishu | Encrypted event subscription. |
| `/wecom/bot` | GET, POST | WeCom | URL verification + encrypted message. |

The Web UI endpoints only register when `channels.web.enabled = true`. Channel webhooks bind their own listeners on their own ports.

## `GET /`

Returns the embedded SPA. Cache headers come from `http.FileServer`. No auth — fronts behind a reverse proxy in production.

## `GET /web/config`

```json
{ "voiceEnabled": true }
```

Used by the SPA to decide whether to show the microphone button.

## `WebSocket /ws`

Subprotocol: none. Origin checked permissively (`InsecureSkipVerify: true`) — gate at the reverse proxy.

**Client → server frame:**

```json
{ "type": "message", "content": "hello" }
```

**Server → client frame:**

```json
{ "type": "stream", "delta": "Hel" }
{ "type": "stream", "delta": "lo." }
{ "type": "stream_done" }
```

For non-streaming replies:

```json
{ "type": "message", "content": "Hello." }
```

Each connection is assigned `clientID = web-<n>`. Inbound messages are published to the bus with `Channel = "web"`, `ChatID = clientID`, `SenderID = clientID`. The allowlist (`channels.web.allowFrom`) is applied here.

## `WebSocket /ws/voice`

Subprotocol: none. Query parameters:

| Param | Required | Description |
|-------|----------|-------------|
| `session` | yes (or `Maven-Session-Id` header) | UUID; resolves to a Maven session ID. |

**Client → server (binary frames):**

Raw PCM, signed 16-bit little-endian, **16 kHz mono**. The browser AudioWorklet downsamples microphone audio to this rate.

**Server → client (binary frames):**

Raw PCM, signed 16-bit little-endian, **24 kHz mono**, no container header. The client builds `AudioBuffer` from each chunk.

**Server → client (single-byte `0x00`):**

Sentinel that flushes the browser's audio queue. Sent when the user starts speaking (the client interrupts in-flight TTS).

**Server → client (text JSON):**

```json
{ "type": "message", "content": "…" }
{ "type": "stream_done" }
```

`message` frames sometimes deliver final non-streamed replies. `stream_done` follows a TTS sequence.

## `POST /v1/responses`

OpenAI Responses-compatible SSE. See [Channels: Web UI](../channels/web.md) for the full event timeline.

**Request:**

```http
POST /v1/responses HTTP/1.1
Content-Type: application/json
Maven-Session-Id: 550e8400-e29b-41d4-a716-446655440000

{
  "input": "Say hi",
  "previous_response_id": ""
}
```

| Field | Type | Description |
|-------|------|-------------|
| `input` | string \| array | A user message or an OpenAI-style message array; the last user message is used as the prompt. |
| `previous_response_id` | string | Optional. Must be a previously issued `resp_…`. Threads the call into the same session as that response. |

| Header | Required | Description |
|--------|----------|-------------|
| `Maven-Session-Id` | yes for new conversations | UUID. Translates to a `web-<uuid>` session. |
| `Content-Type: application/json` | yes | — |

**Response:** `Content-Type: text/event-stream`, framed as:

```text
event: response.created
data: {"type":"response.created","response":{"id":"resp_…","status":"in_progress"}}

event: response.output_item.added
data: {…}

event: response.content_part.added
data: {…}

event: response.output_text.delta
data: {"type":"response.output_text.delta","item_index":0,"content_index":0,"delta":"…"}

…

event: response.output_text.done
data: {…}

event: response.content_part.done
data: {…}

event: response.output_item.done
data: {…}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_…","status":"completed"}}

data: [DONE]
```

**Error shape:**

```json
{
  "error": {
    "message": "unknown previous_response_id",
    "type": "invalid_request_error"
  }
}
```

| Status | Type | Cause |
|--------|------|-------|
| 400 | `invalid_request_error` | Empty input, invalid JSON, unknown / malformed `previous_response_id`, session mismatch. |
| 405 | — | Wrong method. |
| 500 | `server_error` | Runtime stream failure. |
| 500 (text) | — | Connection does not support flushing (no `http.Flusher`). |

`previous_response_id` parsing requires the prefix `resp_` and a non-empty suffix; platform-style IDs (e.g. OpenAI's own `tSmd…`) are rejected.

## `POST /feishu/webhook`

Bound on `channels.feishu.port` (default 9876).

Request body is JSON; the channel performs URL verification (`challenge`), HMAC token check, optional decryption, then publishes inbound. See [Channels: Feishu](../channels/feishu.md).

## `GET /wecom/bot` and `POST /wecom/bot`

Bound on `channels.wecom.port` (default 9886).

- `GET` carries `msg_signature`, `timestamp`, `nonce`, `echostr` — Maven verifies and returns the decrypted echo string.
- `POST` carries `{ "encrypt": "…" }` plus query signature; Maven verifies, decrypts, publishes, and replies with an encrypted `"success"` envelope.

See [Channels: WeCom](../channels/wecom.md) for the wire details.

## CORS

No CORS headers are added by the gateway. Front it with a reverse proxy that adds the headers your clients need.

## Auth

There is no built-in HTTP auth. Treat the gateway as a trusted backend. Use a reverse proxy for TLS termination, authentication, and rate limiting.
