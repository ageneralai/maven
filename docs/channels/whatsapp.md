# WhatsApp

Receive and send WhatsApp messages by logging in as a personal account. Maven uses [whatsmeow](https://github.com/tulir/whatsmeow); session state persists in a SQLite store.

!!! warning "Personal account ToS"
    WhatsApp's Terms of Service prohibit unauthorized automation on the consumer apps. Use a dedicated number you control; assume Meta may ban automation at any time. For business deployments, prefer the official WhatsApp Business API.

## 1. Configure

```json
{
  "channels": {
    "whatsapp": {
      "enabled": true,
      "jid": "",
      "storePath": "",
      "allowFrom": []
    }
  }
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Master toggle. |
| `jid` | `""` | Default outbound JID when `OutboundMessage.ChatID` is empty. Rarely used. |
| `storePath` | `~/.maven/whatsapp-store.db` | SQLite path for the session. |
| `allowFrom` | `[]` (all) | Allowed sender JIDs. Match is tried against both the raw JID and the non-AD (device-stripped) variant. |

## 2. Run and scan

```bash
make gateway
```

On first start the channel prints a QR code:

```text
INFO whatsapp scan QR code to login
[ASCII QR …]
```

In WhatsApp on your phone: **Settings → Linked Devices → Link a Device** → scan.

After a successful link, the next start reuses the SQLite session and skips the QR.

## Inbound

| Event | Behavior |
|-------|----------|
| Text (`Conversation`) | Trimmed text becomes the prompt. |
| Text (`ExtendedTextMessage`) | Used when the simple `Conversation` field is empty. |
| Image (`ImageMessage`) | Downloads with `client.Download(ctx, image)` (20s timeout). Caption (if any) becomes the prompt; the image becomes a multimodal block. MIME type from message or sniffed; `application/octet-stream` is rewritten to `image/jpeg`. |
| Other | Currently ignored. |

`evt.Info.IsFromMe` events are dropped (don't loop on the bot's own sends).

## Outbound

`Send` parses the chat JID and calls `client.SendMessage` with a 30s timeout. JID parsing accepts:

| Input | Resolves to |
|-------|-------------|
| `8613800138000` | `8613800138000@s.whatsapp.net` |
| `+8613800138000` | `8613800138000@s.whatsapp.net` (after stripping `+`) |
| `8613800138000@s.whatsapp.net` | parsed as-is |
| `8613800138000:2@s.whatsapp.net` | parsed as-is (device JID) |

## Allowlist semantics

```go
if !allow.Allow(sender) && !allow.Allow(rawSender) { reject }
```

`sender` is the non-AD form (`evt.Info.Sender.ToNonAD().String()`), `rawSender` is the device-suffixed form. Listing the user JID without device suffix matches every device they link.

A note: `+`-prefixed entries in `allowFrom` are **not** normalized; list the JID form (`84…@s.whatsapp.net`).

## State path

`storePath` is created if missing; permissions follow `os.MkdirAll(dir, 0755)`. The file contains keys and session data — protect it.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| QR code never appears | Make sure `whatsapp.enabled=true`. Container running interactively? QR is printed to stdout. |
| "Sorry, I encountered an error" | LLM provider auth, usually. |
| `event AddressBook` log spam | Normal — whatsmeow syncs contact history on first login. |
| Re-login required | Linked devices were unlinked on the phone. Delete the SQLite store and re-scan. |
| Bot ignores messages from a number | `allowFrom` set? Try the non-AD JID (no `:N` suffix). |
