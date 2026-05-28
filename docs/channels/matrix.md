# Matrix

Receive and send plaintext messages via the Matrix protocol on any homeserver (self-hosted, matrix.org, …). Uses [mautrix-go](https://github.com/mautrix/go) for the sync loop.

!!! note "Plaintext only"
    Encrypted (E2EE) rooms are **not** supported in v1. The bot will see encrypted events as opaque and ignore them.

## 1. Provision a bot account

Create a dedicated Matrix account on your homeserver. Don't use your personal account.

## 2. Get an access token

=== "curl"

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

    Response contains `access_token` and `user_id`.

=== "Element"

    1. Log in as the bot in [Element](https://app.element.io).
    2. **Settings → Help & About → Advanced → Access Token**.
    3. Copy the token.

## 3. Configure

```json
{
  "channels": {
    "matrix": {
      "enabled": true,
      "homeserver": "https://matrix.example.org",
      "accessToken": "syt_…",
      "userId": "@agent:example.org",
      "deviceId": "MAVEN01",
      "allowFrom": ["@alice:example.org"],
      "allowRooms": ["!roomid:example.org"]
    }
  }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `enabled` | yes | Master toggle. |
| `homeserver` | yes | Full URL of the homeserver. Validated by `url.Parse`. |
| `accessToken` | yes | Long-lived access token for the bot account. |
| `userId` | yes | Full MXID. Must start with `@`. |
| `deviceId` | no | Persisted in `<workspace>/.matrix/state.json`. Auto-generated as `MAVEN<8-hex>` on first start if omitted. |
| `allowFrom` | no | Allowed sender MXIDs. Empty means all. |
| `allowRooms` | no | Allowed room IDs (starting with `!`). Empty means all. |

## 4. Run

```bash
make gateway
```

```text
INFO matrix sync started user_id=@agent:example.org
INFO matrix joined room room=!abc:example.org invited_by=@alice:example.org
```

Invite the bot MXID to a room and send a message.

## Auto-join

When the bot receives an `m.room.member` event with `membership: invite` and the state key matches its own MXID, it calls `JoinRoomByID`. Failed joins log at error but don't crash the sync.

## Sender allowlist

`allowFrom` is matched **literally** against the inbound `evt.Sender.String()`. The MXID format is `@localpart:server`. If you need to match a user across multiple servers, list each MXID.

## Room allowlist

`allowRooms` filters by `evt.RoomID.String()`. Combine with `allowFrom` to restrict the bot to specific people in specific rooms.

## State persistence

`<workspace>/.matrix/state.json`:

```json
{
  "nextBatch": "s123_…",
  "filterId": "1",
  "deviceId": "MAVEN01"
}
```

| Field | Purpose |
|-------|---------|
| `nextBatch` | Sync resumption token. Lets the bot pick up where it left off after restart. |
| `filterId` | Homeserver-side filter id (server returns one on first `POST /filter`). |
| `deviceId` | Stable device id across restarts. |

Delete this file to force a full re-sync.

## Outbound

`Send` calls `SendText(roomID, text)` and chunks at 32 000 bytes per send (rune-safe). Markdown is not converted to Matrix HTML — Maven sends `m.text`.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| `matrix sync started` never appears | Token invalid? Use `/whoami` to verify (`curl 'https://…/_matrix/client/v3/account/whoami' -H "Authorization: Bearer syt_…"`). |
| Bot won't join rooms | Inviter MXID set correctly? Check `[matrix] join room … err=…`. |
| Encrypted room ignored | Expected. E2EE not in v1. |
| Old messages spammed on restart | Delete `<workspace>/.matrix/state.json` only if you want a *clean* re-sync; otherwise the next-batch token resumes correctly. |
| Rotating the token | Generate new token via login → update config → delete `state.json` → restart. |
