# Matrix setup

## Prerequisites

- A Matrix account on any homeserver (self-hosted or public, e.g. matrix.org)
- `maven` built (`make build`)

## Step 1: Get an access token

The bot uses a dedicated Matrix account. Create one if needed, then obtain a long-lived access token.

### Via curl

```bash
curl -XPOST \
  'https://matrix.example.org/_matrix/client/v3/login' \
  -H 'Content-Type: application/json' \
  -d '{
    "type": "m.login.password",
    "user": "agent",
    "password": "your-password"
  }'
```

The response contains `access_token` and `user_id`. Save both.

### Via Element

1. Log in as the bot account in [Element](https://app.element.io)
2. Go to **Settings → Help & About → Advanced → Access Token**
3. Copy the token

## Step 2: Configure maven

Edit `~/.maven/config.json`:

```json
{
  "channels": {
    "matrix": {
      "enabled": true,
      "homeserver": "https://matrix.example.org",
      "accessToken": "syt_...",
      "userId": "@agent:example.org",
      "deviceId": "MAVEN01",
      "allowFrom": [],
      "allowRooms": []
    }
  }
}
```

## Step 3: Configuration reference

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Whether the Matrix channel is enabled |
| `homeserver` | string | Full URL of the homeserver (e.g. `https://matrix.org`) |
| `accessToken` | string | Long-lived access token for the bot account |
| `userId` | string | Full MXID of the bot (e.g. `@agent:example.org`) |
| `deviceId` | string | Device ID stored in state; auto-generated on first start if omitted |
| `allowFrom` | []string | Allowed sender MXIDs (`[]` = everyone) |
| `allowRooms` | []string | Allowed room IDs (`[]` = all rooms) |

### Find your MXID

Your MXID is shown in Element under **Settings → General** (e.g. `@alice:matrix.org`). Add it to `allowFrom` to restrict the bot to yourself:

```json
"allowFrom": ["@alice:matrix.org"]
```

### Restrict to specific rooms

```json
"allowRooms": ["!roomid:matrix.org"]
```

Room IDs (starting with `!`) are shown in Element under **Room Settings → Advanced**.

## Step 4: Run and test

```bash
# Start gateway
make gateway

# Or run directly
./maven gateway
```

Success is indicated by:

```
[matrix] sync started as @agent:example.org
[matrix] joined room !roomid:example.org (invited by @alice:example.org)
```

Invite the bot MXID to a room and send a message to test.

## State

Maven persists sync state at `<agent.workspace>/.matrix/state.json`:

```json
{
  "nextBatch": "s123_...",
  "filterId": "1",
  "deviceId": "MAVEN01"
}
```

Delete this file to force a full re-sync from the beginning of the room history.

## Troubleshooting

**Q: The bot does not respond?**
- Check logs for `[matrix] sync started as @...`
- Run `maven status` and confirm `Matrix: enabled=true`
- Confirm the access token is valid (try the `/whoami` endpoint)

```bash
curl 'https://matrix.example.org/_matrix/client/v3/account/whoami' \
  -H "Authorization: Bearer syt_..."
```

**Q: The bot does not join rooms?**
- Invite the bot MXID explicitly from your Matrix client
- Check logs for `[matrix] join room ... after invite`
- Confirm `allowRooms` is empty or contains the room ID

**Q: How do I rotate the access token?**
- Generate a new token via login
- Update `accessToken` in config and restart maven
- Delete `~/.maven/.matrix/state.json` to force a clean sync with the new session
