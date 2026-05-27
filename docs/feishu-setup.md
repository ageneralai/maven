# Feishu (Lark) bot setup

## Prerequisites

- A Feishu account (must belong to a team; free teams work)
- `maven` built (`make build`)
- A publicly reachable URL for the webhook (e.g. a `cloudflared` tunnel)

## Step 1: Create a Feishu app

1. Open [Feishu Open Platform](https://open.feishu.cn/)
2. Click **Create app** → **Custom app**
3. Enter app name (e.g. `maven`) and description
4. In the app, open **Credentials & basic info** and record:
   - **App ID** (e.g. `cli_a5xxxxx`)
   - **App Secret**

## Step 2: Add bot capability

1. Open **App capabilities** → **Add capability**
2. Choose **Bot**

## Step 3: Configure scopes

Open **Permissions** and enable:

| Scope | Purpose |
|------|---------|
| `im:message` | Read and send messages |
| `im:message:send_as_bot` | Send as the app bot |

## Step 4: Event subscription

1. Open **Events & callbacks** → **Event configuration**
2. Set **Request URL** to your public URL:
   ```
   https://your-domain.com/feishu/webhook
   ```
3. Feishu sends a challenge; maven answers automatically
4. Under **Encryption strategy**, note **Verification Token**
5. Add event: `im.message.receive_v1` (receive message v2.0)

## Step 5: Publish the app

1. Open **Version management & release**
2. Create a version, set version number and release notes
3. Submit review (custom apps can often be approved by your own admin)

> After permission changes, publish a new version for them to take effect.

## Step 6: Configure maven

### Option A: Config file

Edit `~/.maven/config.json`:

```json
{
  "channels": {
    "feishu": {
      "enabled": true,
      "appId": "cli_a5xxxxx",
      "appSecret": "your-app-secret",
      "verificationToken": "your-verification-token",
      "port": 9876,
      "allowFrom": []
    }
  }
}
```

Or run `make setup` for interactive config.

## Configuration reference

| Field | Type | Description |
|------|------|-------------|
| `enabled` | bool | Enable the Feishu channel |
| `appId` | string | Feishu app App ID |
| `appSecret` | string | Feishu app App Secret |
| `verificationToken` | string | Event subscription verification token (empty = skip verification) |
| `encryptKey` | string | Event encryption key (optional) |
| `port` | int | Webhook HTTP port (default 9876) |
| `allowFrom` | []string | Allowed `open_id` values (`[]` = everyone) |

## Step 7: Expose localhost (tunnel)

Feishu needs a public URL. For dev/test, `cloudflared` is convenient.

### Ephemeral tunnel

```bash
# Install cloudflared
brew install cloudflared

# Start tunnel
make tunnel
# or
cloudflared tunnel --url http://localhost:9876
```

Output includes a temporary URL:

```
https://xxx-xxx-xxx.trycloudflare.com
```

Use that base URL + `/feishu/webhook` as the Feishu event URL.

> The ephemeral URL changes on restart; update Feishu when it does.

### Named tunnel (production)

```bash
cloudflared tunnel login
cloudflared tunnel create maven
cloudflared tunnel route dns maven feishu-bot.yourdomain.com
cloudflared tunnel run maven
```

Set the Feishu URL to `https://feishu-bot.yourdomain.com/feishu/webhook`.

### Docker Compose tunnel

```bash
docker compose --profile tunnel up -d
docker compose logs tunnel | grep trycloudflare
```

## Step 8: Run and test

```bash
make gateway
```

Success logs:

```
[feishu] webhook server listening on :9876
[gateway] channels started: [feishu]
```

In Feishu, open a chat with your bot and send a message.

## Troubleshooting

**Q: Feishu messages get no reply?**
- Confirm the event URL is set and challenge verification passed
- Confirm `im.message.receive_v1` is subscribed
- Confirm the app is published and approved
- Confirm the tunnel is up

**Q: “Access denied”?**
- Confirm `im:message` and/or `im:message:send_as_bot` are granted
- Republish after permission changes

**Q: Webhook 401?**
- Match `verificationToken` to the open platform value
- Empty string skips verification (dev/test only)

**Q: How do I get a user `open_id`?**
- Check maven logs for `inbound from feishu/ou_xxx`
- `ou_xxx` is the `open_id`; add it to `allowFrom`

**Q: Restrict to myself?**
- Send a message, read `open_id` from logs
- Add to `allowFrom`:

  ```json
  "allowFrom": ["ou_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"]
  ```
