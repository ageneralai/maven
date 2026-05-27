# OneCLI Agent Vault

[OneCLI](https://github.com/onecli/onecli) is an open-source agent vault that sits between AI agents and external APIs. It stores credentials in an encrypted vault and injects them through an HTTPS proxy so Maven never holds raw API keys in config or environment.

## Architecture

```text
Maven process
  └── http.DefaultClient (HTTPS_PROXY set with agent token)
        └── OneCLI gateway (:10255)
              ├── injects API key from vault (replaces x-api-key / Authorization)
              └── forwards to api.anthropic.com / api.openai.com / ...
```

Maven has no OneCLI-specific code. It works because Maven uses `http.DefaultClient`, which respects `HTTPS_PROXY` and `SSL_CERT_FILE` from the environment.

Port layout: **10254** = dashboard (web UI), **10255** = gateway (HTTPS proxy).

## Prerequisites

- Maven built and configured (see [Getting Started](getting-started.md))
- Docker (or native OneCLI install)

## Step 1: Start OneCLI

```bash
docker run -d \
  --name onecli \
  -p 10254:10254 \
  -p 10255:10255 \
  -v onecli-data:/app/data \
  ghcr.io/onecli/onecli
```

Or install natively:

```bash
curl -fsSL https://onecli.sh/install | sh
```

Verify both services are up:

```bash
curl -sf http://127.0.0.1:10254/v1/health   # dashboard
curl -sf http://127.0.0.1:10255/healthz      # gateway
```

## Step 2: Add credentials and get your agent token

Open the dashboard at **http://127.0.0.1:10254**, then:

1. Go to **Secrets** and add your Anthropic (or OpenAI) API key.
2. Go to **Agents**, open the default agent, and copy its **access token** (`aoc_...`).

The gateway authenticates each proxied request by this token and injects the matching credential.

## Step 3: Trust OneCLI's CA certificate

OneCLI terminates TLS to inject credentials. Go's `x509` package reads `SSL_CERT_FILE` natively on Linux, so no custom code is needed — just point the env var at OneCLI's generated CA:

```bash
# Docker install
export SSL_CERT_FILE=/path/to/onecli-data/gateway/ca.pem

# Native install
export SSL_CERT_FILE=~/.onecli/gateway/ca.pem
```

Alternatively, install the CA into your OS trust store and omit `SSL_CERT_FILE`.

## Step 4: Configure Maven to use the proxy

Set `provider.apiKey` to a non-empty placeholder in `~/.maven/config.json` — OneCLI replaces it at the gateway before the request reaches Anthropic:

```json
{
  "provider": {
    "type": "anthropic",
    "apiKey": "placeholder",
    "baseUrl": "https://api.anthropic.com"
  }
}
```

Start Maven with the proxy env, embedding your agent token in the proxy URL:

```bash
export HTTPS_PROXY=http://x:aoc_YOUR_TOKEN@127.0.0.1:10255
export SSL_CERT_FILE=~/.onecli/gateway/ca.pem
./maven gateway
```

The `x:TOKEN` format is standard HTTP Basic auth — `x` is a dummy username, the token is the password.

## Step 5: Verify

Send a message through any enabled channel, or run the CLI agent:

```bash
export HTTPS_PROXY=http://x:aoc_YOUR_TOKEN@127.0.0.1:10255
export SSL_CERT_FILE=~/.onecli/gateway/ca.pem
./maven agent "hello"
```

Check OneCLI audit logs:

```bash
docker logs onecli 2>&1 | tail -20
```

You should see requests to `api.anthropic.com` with `injections_applied=1`.

## systemd example

```ini
[Service]
Environment=HTTPS_PROXY=http://x:aoc_YOUR_TOKEN@127.0.0.1:10255
Environment=SSL_CERT_FILE=/home/user/.onecli/gateway/ca.pem
```

`provider.apiKey` in config must be a non-empty string (e.g. `"placeholder"`); the gateway replaces it.

## Troubleshooting

**`x509: certificate signed by unknown authority`**
- Set `SSL_CERT_FILE` to OneCLI's CA: `~/.onecli/gateway/ca.pem` (native) or the Docker volume path

**401 from upstream API**
- Secret not configured in vault, or agent token missing/wrong in `HTTPS_PROXY`
- Open the dashboard at `:10254`, confirm the secret exists and the token matches the agent

**Connection refused on :10255**
- OneCLI gateway is not running: `docker ps | grep onecli`

**Maven fails to start with "provider.apiKey is required"**
- Set `apiKey` to any non-empty string (e.g. `"placeholder"`) — the gateway overwrites it

**Maven still sends its own API key / 401 despite gateway**
- Confirm the agent token is correct in `HTTPS_PROXY` (`http://x:aoc_TOKEN@...`)
- Remove `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` from the environment so Maven doesn't read them as the real key

## See also

- [Proxy](proxy.md) — general proxy configuration for Maven
- [OneCLI docs](https://onecli.sh/docs)
