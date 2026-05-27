# Proxy

Maven routes all outbound HTTP through the Go standard library's default transport. There is no proxy field in `config.json` — configure egress at the process level instead.

## How it works

Every outbound call — LLM APIs, channels (Telegram, Feishu, WeCom), voice TTS, tool HTTP — uses `http.DefaultClient`, whose transport reads proxy settings from the environment:

| Variable | Purpose |
|----------|---------|
| `HTTPS_PROXY` | Proxy URL for HTTPS traffic (e.g. `http://127.0.0.1:10254`) |
| `HTTP_PROXY` | Proxy URL for HTTP traffic |
| `NO_PROXY` | Comma-separated hosts to bypass the proxy |
| `SSL_CERT_FILE` | Path to a CA bundle for TLS trust (required for MITM proxies like OneCLI) |

Go's `http.DefaultTransport` calls `http.ProxyFromEnvironment` automatically. No Maven-specific configuration is needed.

## Basic usage

```bash
export HTTPS_PROXY=http://127.0.0.1:7890
./maven gateway
```

Supported proxy schemes: `http://`, `https://`, `socks5://`.

## Regions without direct API access

If Telegram, Anthropic, or other APIs are unreachable from your network, set a proxy before starting Maven:

```bash
export HTTPS_PROXY=socks5://127.0.0.1:1080
./maven gateway
```

This applies to all outbound HTTP — channels, LLM, and tools — through one path.

## TLS and custom CA certificates

Some proxies (including [OneCLI](onecli.md)) terminate TLS and re-encrypt to upstream APIs. The client must trust the proxy's CA certificate:

```bash
export SSL_CERT_FILE=/path/to/proxy-ca.pem
export HTTPS_PROXY=http://127.0.0.1:10254
./maven gateway
```

Alternatively, install the CA into your OS trust store and omit `SSL_CERT_FILE`.

## systemd / Docker

Set environment variables in your unit file or container spec:

```ini
# /etc/systemd/system/maven.service.d/proxy.conf
[Service]
Environment=HTTPS_PROXY=http://127.0.0.1:10254
Environment=SSL_CERT_FILE=/etc/onecli/ca.pem
```

```yaml
# docker-compose.yml
services:
  maven:
    environment:
      HTTPS_PROXY: http://onecli:10254
      SSL_CERT_FILE: /etc/onecli/ca.pem
```

## Troubleshooting

**Connection refused or timeout**
- Confirm the proxy is running: `curl -x $HTTPS_PROXY https://api.anthropic.com`
- Check `NO_PROXY` is not excluding the target host

**TLS certificate errors (`x509: certificate signed by unknown authority`)**
- Set `SSL_CERT_FILE` to the proxy's CA bundle, or install the CA system-wide

**Bot connects but LLM calls fail (or vice versa)**
- Maven uses one transport for all egress — if proxy works for one, it works for all. Check that the proxy allows the specific upstream host.
