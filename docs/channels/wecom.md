# WeCom

Receive and reply via a WeCom (企业微信) **intelligent bot in API mode**. This is *not* the custom-app callback mode — Maven implements only the AI bot v1 contract: encrypted webhook in, `response_url` reply out.

!!! warning "Reactive-only"
    WeCom is a **reactive-only** channel. Outbound delivery uses a short-lived `response_url` from the most recent inbound. Cron jobs with `deliver: true` skip WeCom and log a warning. Use another channel for proactive notifications.

## Supported scope

| Capability | Status |
|------------|--------|
| Inbound `text` | Yes |
| Inbound `voice` | Yes (`voice.content` field) |
| Inbound `image` | Yes (URL download as multimodal block, ≤10 MB) |
| Inbound `mixed` | Yes (extracts only `text` items) |
| Outbound `markdown` via `response_url` | Yes (capped 20480 bytes) |
| Streaming | No |
| Template cards | No |
| `media_id` image download | No (would require access token) |

## 1. Create the bot

1. WeCom admin → **Security & management → Admin tools → Create bot**.
2. Choose **API mode**.
3. Save the **Token** and **EncodingAESKey** (43 chars).
4. Optional: note the **ReceiveID** for strict decrypt validation.

## 2. Configure the callback

In the bot's WeCom settings, set:

- **URL:** `https://your-domain.com/wecom/bot`
- **Token** and **EncodingAESKey** matching Maven config.

WeCom calls the URL with verification; Maven handles it automatically.

## 3. Configure Maven

```json
{
  "channels": {
    "wecom": {
      "enabled": true,
      "token": "your-token",
      "encodingAESKey": "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
      "receiveId": "",
      "port": 9886,
      "allowFrom": ["zhangsan"],
      "proxy": ""
    }
  }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `enabled` | yes | Master toggle. |
| `token` | yes | Callback signing token. |
| `encodingAESKey` | yes | Encrypt/decrypt key. Must be **exactly 43 characters**. |
| `receiveId` | no | Optional; enables strict receiver-ID checks on decrypt. |
| `port` | no | Callback HTTP port. Default `9886`. |
| `allowFrom` | no | Allowed sender user IDs. Empty means all. |
| `proxy` | no | Per-channel proxy URL. |

## 4. Run

```bash
make gateway
```

```text
INFO wecom callback server listening port=9886
INFO gateway channels started channels=[wecom]
```

## Inbound message flow

1. WeCom POSTs `{ "encrypt": "…base64…" }` to `/wecom/bot` with `msg_signature`, `timestamp`, `nonce` query params.
2. Maven verifies the signature, decrypts to JSON, checks `msgid` for dedup, applies `allowFrom`, parses content, and publishes to the bus.
3. Maven returns a HTTP 200 with an encrypted `{"encrypt": "…", "msgsignature": "…", …}` envelope wrapping `"success"`.

## Outbound flow

When the agent reply is published:

1. Maven looks up the cached `response_url` keyed by chat (1 hour TTL).
2. POSTs `{"msgtype":"markdown","markdown":{"content":"…"}}` to that URL.
3. Retries up to 3 times on transient errors (`errcode -1`, `6000`, HTTP 5xx). Payload errors (`44004` etc.) do **not** retry.
4. Content exceeding 20480 bytes is truncated at a UTF-8 boundary.

## `response_url` caveats

- **Short-lived.** Often single-use. Delayed or repeated sends may fail.
- **Per-chat.** Bound to the latest inbound from that user/group.
- **No `media_id` upload.** Maven only supports markdown payloads via this endpoint.

## Image downloads

When inbound carries `image.url`, Maven fetches it (≤10 MB), detects the MIME type (or uses the `Content-Type`), and attaches a base64 multimodal block. If only `media_id` is present, Maven logs a warning — fetching by `media_id` requires the corp access token flow not implemented in API-mode bots.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Callback verification fails | `token` matches console. `encodingAESKey` is exactly 43 chars. Signature query params present. |
| 401 on POST | `msg_signature` mismatch — usually a `token` typo. |
| Duplicate messages | `msgid` dedup is in-memory; if your deployment is multi-replica behind a load balancer, prefer a single replica or implement shared dedup. |
| Reply not delivered | `response_url` cache may have expired (1h TTL) or been consumed. The user must send something new to refresh it. |
| Cron delivery skipped | Expected. WeCom is reactive-only; pick another channel for proactive delivery. |
| 44004 from WeCom | Content too large. The truncation is 20480 bytes; verify your model isn't returning more in one chunk. |
