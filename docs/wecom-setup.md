# WeCom (WeChat Work) intelligent bot integration

## Prerequisites

- A WeCom (enterprise WeChat) admin account
- `maven` built (`make build`)
- A publicly reachable callback URL (HTTPS recommended in production)

> Maven implements the channel protocol and business logic only. Public ingress, TLS certificates, DNS, and reverse proxies are your deployment responsibility.

## Protocol (single supported mode)

The `wecom` channel implements **WeCom intelligent bot API mode** only (not custom-app callback mode).

Callback behavior:

- URL verification: `GET` with `msg_signature`, `timestamp`, `nonce`, `echostr`
- Message push: `POST` with JSON body as encrypted payload (`{"encrypt":"..."}`)
- After decryption, inbound payload is JSON (includes `msgid`, `from.userid`, `response_url`, `msgtype`, etc.)
- Outbound replies use `response_url` with `markdown` messages

## Supported scope

Supported today:

- Inbound: `text`, `voice`, `mixed` (only `text` parts are extracted)
- Outbound: `markdown` via `response_url`
- `allowFrom` allowlist (if unset or empty, all users are allowed by default)
- `msgid` deduplication
- Callback signature verification and encryption/decryption

Not supported:

- Template cards
- Streaming replies
- Complex event handling

## Step 1: Create a WeCom intelligent bot

1. Sign in to the WeCom admin console
2. Go to **Security & management** → **Admin tools** → **Create bot**
3. Choose **Create in API mode**

   ![74444fff04262489ee33c735877cc976.png](https://i.mji.rip/2026/02/09/74444fff04262489ee33c735877cc976.png)

4. Record:
   - `Token`
   - `EncodingAESKey`

> Optional: `ReceiveID`—if you know the receiver ID used for encrypt/decrypt checks, you can set it in maven; if omitted, strict ReceiveID validation is not enforced.

## Step 2: Configure the callback URL

In the WeCom bot settings:

- URL: `https://your-domain.com/wecom/bot`
- Token: must match your config file
- EncodingAESKey: must match your config file

Saving triggers URL verification; maven handles it automatically.

## Step 3: Configure maven

Edit `~/.maven/config.json`:

```json
{
  "channels": {
    "wecom": {
      "enabled": true,
      "token": "your-token",
      "encodingAESKey": "your-43-char-encoding-aes-key",
      "receiveId": "",
      "port": 9886,
      "allowFrom": ["zhangsan"]
    }
  }
}
```

### Fields

| Field | Type | Description |
|------|------|-------------|
| `enabled` | bool | Enable the WeCom channel |
| `token` | string | Callback signing token |
| `encodingAESKey` | string | 43-character encrypt/decrypt key |
| `receiveId` | string | Optional; enables strict receiver-ID checks |
| `port` | int | Callback HTTP port (default 9886) |
| `allowFrom` | []string | Optional allowlist; if unset or empty, all users are accepted |

### Environment variables (optional overrides)

```bash
export MAVEN_WECOM_TOKEN="your-token"
export MAVEN_WECOM_ENCODING_AES_KEY="your-43-char-encoding-aes-key"
export MAVEN_WECOM_RECEIVE_ID="optional-receive-id"
```

## Step 4: Run and verify

```bash
make gateway
```

Healthy startup looks like:

```text
[wecom] callback server listening on :9886
[gateway] channels started: [wecom]
```

Send a text message to the bot in WeCom and confirm the gateway replies.

## Limits and risks

- `allowFrom` defaults to open:
  - Unset or `[]`: accept all inbound users
  - Non-empty list: only listed users
  - Misconfiguring `allowFrom: [""]` enables an allowlist that can reject everyone
- Outbound depends on a temporary `response_url`:
  - Maven can only reply if a recent inbound message cached `response_url`
  - `response_url` is short-lived; avoid delayed or multi-shot sends
  - After expiry, sends fail with an error
- Outbound `markdown.content` is capped at 20480 bytes; excess is truncated (no auto chunking)
- Never commit `token` / `encodingAESKey` to source control
