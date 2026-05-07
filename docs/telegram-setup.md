# Telegram Bot setup

## Prerequisites

- A Telegram account
- `maven` built (`make build`)

## Step 1: Create a Telegram bot

1. In Telegram, search for **@BotFather** and send `/newbot`
2. When prompted, enter the bot display name (e.g. `My Claw Assistant`)
3. Enter the bot username (must end with `bot`, e.g. `maven_bot`)
4. BotFather returns a **bot token**, for example:
   ```
   1234567890:ABCdefGHIjklMNOpqrsTUVwxyz
   ```
5. Save this token

## Step 2: Configure maven

### Option A: Config file

Edit `~/.maven/config.json`:

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "1234567890:ABCdefGHIjklMNOpqrsTUVwxyz",
      "allowFrom": [],
      "proxy": "",
      "rootDir": ""
    }
  }
}
```

### Option B: Environment variable

```bash
export MAVEN_TELEGRAM_TOKEN="1234567890:ABCdefGHIjklMNOpqrsTUVwxyz"
```

## Step 3: Configuration reference

| Field | Type | Description |
|------|------|-------------|
| `enabled` | bool | Whether the Telegram channel is enabled |
| `token` | string | Bot token from BotFather |
| `allowFrom` | []string | Allowed user IDs (`[]` = everyone) |
| `proxy` | string | Proxy URL (e.g. `socks5://127.0.0.1:1080`); required in some regions |
| `rootDir` | string | Telegram assets directory; default `<agent.workspace>/.telegram` with `slashes/` and `handlers/` |

### Find your user ID

1. In Telegram, search for **@userinfobot** and send any message
2. It replies with your numeric user ID
3. Add the ID to `allowFrom` so only you can use the bot:
   ```json
   "allowFrom": ["123456789"]
   ```

## Step 4: Run and test

```bash
# Start gateway
make gateway

# Or run directly
./maven gateway
```

Success is indicated by log lines like:

```
[telegram] authorized as @maven_bot
[telegram] polling started
```

In Telegram, find your bot by username and send a message to test.

## Proxy (regions without direct Telegram API access)

If you cannot reach the Telegram API directly, configure a proxy:

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "your-token",
      "proxy": "socks5://127.0.0.1:1080"
    }
  }
}
```

Supported schemes: `socks5://`, `http://`, `https://`

## Troubleshooting

**Q: The bot does not respond?**
- Check logs for `[telegram] authorized as @xxx`
- Confirm the API key is configured (`maven status`)
- If you are behind a restrictive network, confirm the proxy settings

**Q: You get “Sorry, I encountered an error”?**
- Check logs for `[gateway] agent error`
- Confirm the API proxy/key is valid

**Q: How do I restrict usage to myself?**
- Get your user ID and add it to `allowFrom`
