# Telegram

Receive and send messages via a Telegram bot. Supports text, photos, documents, audio/voice, video, status-card streaming, custom slash commands, and message reactions.

## 1. Create a bot

1. Open [@BotFather](https://t.me/BotFather) in Telegram and send `/newbot`.
2. Choose a display name and a username ending in `bot`.
3. Save the **bot token** BotFather returns (e.g. `1234567890:ABCdefGHIjklMNOpqrsTUVwxyz`).

## 2. Configure

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "1234567890:ABCdefGHIjklMNOpqrsTUVwxyz",
      "allowFrom": ["123456789"],
      "streaming": true,
      "feedback": "normal",
      "rootDir": "",
      "proxy": ""
    }
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Master toggle. |
| `token` | string | — | Bot token from BotFather. Must match `^\d+:[A-Za-z0-9_-]+$`. |
| `allowFrom` | []string | `[]` (all) | Numeric user IDs allowed to message the bot. |
| `streaming` | bool | `false` | Stream model output progressively. Private DMs use Bot API `sendMessageDraft`; groups/supergroups use `editMessageText`. |
| `feedback` | string | `"normal"` | Inbound feedback: `debug` (reaction + typing + verbose card), `normal` (reaction + typing), `minimal` (typing only), `silent`. |
| `rootDir` | string | `""` | Telegram assets directory. Empty means `<workspace>/.telegram`. |
| `proxy` | string | `""` | Per-channel proxy (`http://`, `https://`, `socks5://`). Empty uses `HTTPS_PROXY` env. |

## 3. Find your user ID

Message [@userinfobot](https://t.me/userinfobot) on Telegram — it replies with your numeric ID. Add it to `allowFrom` so only you can talk to the bot.

## 4. Run

```bash
make gateway
```

You should see:

```text
INFO telegram authorized username=maven_bot
INFO telegram polling started
INFO telegram bot commands registered count=4
```

Open a chat with the bot in Telegram and send a message.

## Streaming

When `streaming: true` and your LLM provider returns SSE chunks:

- **Private chats** use Bot API `sendMessageDraft` (slot id 1) for the content message and `editMessageText` for the status card. Drafts allow text up to ~4096 runes.
- **Groups / supergroups** fall back to placeholder + `editMessageText` for both because `sendMessageDraft` is private-only.

The status card shows iteration counts, each tool call with a brief input summary, and streamed subprocess output for `DelegateTask`. On completion, the final reply is sent as a normal message and the intermediate placeholders are deleted.

If the model does **not** stream (provider returns one chunk at the end), you still get the streaming pipeline — just with a single delta at the end.

## Slash commands

The Telegram channel supports three sources:

1. **`/new`** — built-in routing hint that rotates the session.
2. **Workspace slashes** under `<workspace>/.telegram/slashes/*.md` — markdown with YAML frontmatter (see [Guides: Slash commands](../guides/slash-commands.md)).
3. **Plugin slashes** like `/compact`, `/status`, `/memory`, `/jobs` — registered automatically via `SetMyCommands` on gateway start (merged with workspace defs; workspace overrides description and handling when names collide). Commands with hyphens (e.g. `/cron-add`) still work when typed but are omitted from the Telegram menu because Bot API names must match `[a-z0-9_]{1,32}`.

`SetMyCommands` registers the merged list with Telegram so users see autocomplete.

## Files and media

Inbound media is handled differently per type:

| Type | Behavior |
|------|----------|
| Photo | Downloaded; largest size attached as multimodal image block. |
| Document with image MIME | Downloaded; attached as image block. |
| Document (other) | Saved to `<workspace>/uploads/<ms>_<sanitized-name>`; the prompt gets `[File saved to: …]`. |
| Voice / Audio / Video | Saved to `<workspace>/uploads/<ms>_<name>`; prompt gets a `[…saved to:…]` line. |
| Media group (album) | Buffered 500 ms then flushed as one inbound with combined caption text + image blocks. |
| Reply context | Extracted into a `[Replying to <user>]` block. |
| Forwarded | `[Forwarded from <origin>]` line; "Summarize or process" hint if there's no caption. |
| External reply / quoted | Extracted with optional photo download. |

## Reactions and typing

`PreProcessFeedback` reacts to the user's message with 👀 or 👍 depending on the `feedback` setting and sends a chat action so users see the bot is "typing". Reactions are best-effort; failures are logged at warn.

## Region restrictions / proxy

Set `proxy` per channel or `HTTPS_PROXY` for the whole process. Both work. See [Deployment: Proxy](../deployment/proxy.md).

## Troubleshooting

| Symptom | Check |
|---------|-------|
| No reply | `[telegram] authorized as @…` in logs? LLM key valid? Sender in `allowFrom`? |
| "Sorry, I encountered an error" | Logs at `error`; usually LLM provider auth or model id. |
| Streaming flickers in groups | Telegram's `editMessageText` rate limits; the channel respects `retry_after` automatically. |
| 429 from Bot API | The channel parses retry-after and waits; persistent rate-limits suggest too-aggressive streaming or invalid HTML in content (the channel retries the final send as plaintext). |
| File download fails | Check workspace write permission; size limits enforced by Telegram (~50 MB). |
