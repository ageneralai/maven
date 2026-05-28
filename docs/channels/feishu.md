# Feishu (Lark)

Receive and send messages via a Feishu bot. Maven implements the Open Platform v3 webhook flow with image download support.

## 1. Create a Feishu app

1. Open the [Feishu Open Platform](https://open.feishu.cn/) and create a **custom app**.
2. Under **Credentials & basic info**, copy:
    - **App ID** (`cli_…`)
    - **App Secret**

## 2. Enable the bot

In the app console: **App capabilities → Add capability → Bot**.

## 3. Grant scopes

| Scope | Why |
|-------|-----|
| `im:message` | Receive messages. |
| `im:message:send_as_bot` | Send messages as the bot. |

Publish a new version after permission changes so they take effect.

## 4. Configure the event webhook

In **Events & callbacks → Event configuration**:

- **Request URL:** `https://your-domain.com/feishu/webhook`
- Save — Feishu sends a `challenge`; Maven answers it automatically.
- **Verification token:** copy it (you'll put it in Maven config).
- **Encryption strategy:** optional; if you set an `encryptKey` here, set it in Maven config too.
- **Events:** subscribe to `im.message.receive_v1`.

## 5. Configure Maven

```json
{
  "channels": {
    "feishu": {
      "enabled": true,
      "appId": "cli_a5xxxxx",
      "appSecret": "your-app-secret",
      "verificationToken": "your-verification-token",
      "encryptKey": "",
      "port": 9876,
      "allowFrom": [],
      "proxy": ""
    }
  }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `enabled` | yes | Master toggle. |
| `appId` | yes | App ID from credentials. |
| `appSecret` | yes | App secret from credentials. |
| `verificationToken` | recommended | Empty disables verification (dev only). |
| `encryptKey` | when enabled in console | Event encryption key. |
| `port` | no | Webhook HTTP port. Default `9876`. |
| `allowFrom` | no | Allowed `open_id` values. Empty means all. |
| `proxy` | no | Per-channel proxy URL. |

## 6. Publish

Open **Version management & release**, create a version, submit for review (your own admin can approve custom apps).

## 7. Tunnel for development

Feishu needs a public webhook URL. For local dev use a cloudflared tunnel:

```bash
make tunnel
# or
cloudflared tunnel --url http://localhost:9876
```

Pipe the printed `https://*.trycloudflare.com/feishu/webhook` URL into Feishu's event subscription. The ephemeral URL changes on restart; production should use a named tunnel.

## 8. Run

```bash
make gateway
```

Look for:

```text
INFO feishu webhook server listening port=9876
INFO gateway channels started channels=[feishu]
```

In Feishu, message the bot directly to test.

## Inbound message types

| Type | Behavior |
|------|----------|
| `text` | Trimmed `text.content` becomes the prompt. |
| `image` | Downloads via `/open-apis/im/v1/images/{key}?image_type=message` and attaches as a multimodal image block. Falls back to URL-only block if download fails. Capped at 10 MB. |
| Other (`post`, `share_chat`, etc.) | Currently unsupported — silently skipped. |

## Outbound

`Send` calls `/open-apis/im/v1/messages?receive_id_type=chat_id` with a text payload built by `feishuTextMessagePayload`. Markdown rendering is not converted; the model's output goes as plain text.

Tenant access tokens are cached for `expire - 60s`.

## Allowlist by `open_id`

Find a user's `open_id` from inbound logs:

```text
INFO feishu inbound from feishu/ou_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Add it to `allowFrom`:

```json
"allowFrom": ["ou_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"]
```

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Challenge fails | Webhook URL reachable? Tunnel up? Token verification mismatch? |
| 401 from Feishu | `verificationToken` matches console. |
| "Access denied" from Feishu | `im:message` and `im:message:send_as_bot` granted, **app version published** after permission changes. |
| Image download fails | Tenant token can read messages? Image not larger than 10 MB? Logs show the URL it tried. |
| No reply | App receiving events? Logs say `inbound from feishu/ou_…`? `allowFrom` not too narrow? |
