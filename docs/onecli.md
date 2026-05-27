# OneCLI Agent Vault

[OneCLI](https://github.com/onecli/onecli) is an open-source agent vault that sits between AI agents and external APIs. It stores credentials in an encrypted vault and injects them through an HTTPS proxy so Maven never holds raw API keys in config or environment.

## Architecture

```text
Maven process
  └── http.DefaultClient (HTTPS_PROXY set)
        └── OneCLI proxy (:10254)
              ├── injects API key from vault
              └── forwards to api.anthropic.com / api.openai.com / ...
```

Maven has no OneCLI-specific code. It works because Maven uses `http.DefaultClient`, which respects `HTTPS_PROXY` and `SSL_CERT_FILE` from the environment.

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
curl -fsSL onecli.sh/install | sh
onecli start
```

Verify:

```bash
curl -sf http://127.0.0.1:10255/health
```

## Step 2: Add credentials to the vault

```bash
onecli secrets add anthropic \
  --host-pattern api.anthropic.com \
  --header-name Authorization \
  --format "Bearer {secret}"
```

Paste your API key when prompted. Repeat for other providers (OpenAI, etc.) as needed.

List secrets:

```bash
onecli secrets list
```

## Step 3: Trust OneCLI's CA certificate

OneCLI terminates TLS to inject credentials. Maven must trust OneCLI's CA:

```bash
# Export OneCLI's CA bundle (path varies by install)
export SSL_CERT_FILE=/path/to/onecli-combined-ca.pem
```

Or install the CA into your OS trust store and skip this step.

## Step 4: Configure Maven to use the proxy

Remove API keys from `~/.maven/config.json` — OneCLI injects them:

```json
{
  "provider": {
    "type": "anthropic",
    "apiKey": "",
    "baseUrl": "https://api.anthropic.com"
  }
}
```

Start Maven with the proxy env:

```bash
export HTTPS_PROXY=http://127.0.0.1:10254
export SSL_CERT_FILE=/path/to/onecli-combined-ca.pem
./maven gateway
```

## Step 5: Verify

Send a message through any enabled channel, or run the CLI agent:

```bash
export HTTPS_PROXY=http://127.0.0.1:10254
export SSL_CERT_FILE=/path/to/onecli-combined-ca.pem
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
Environment=HTTPS_PROXY=http://127.0.0.1:10254
Environment=SSL_CERT_FILE=/etc/onecli/ca.pem
```

Ensure `provider.apiKey` is empty in config when using the vault.

## Troubleshooting

**`x509: certificate signed by unknown authority`**
- Set `SSL_CERT_FILE` to OneCLI's CA bundle

**401 from upstream API**
- Secret not configured in vault, or wrong host pattern
- Run `onecli secrets list` and confirm a secret matches the upstream hostname

**Connection refused on :10254**
- OneCLI is not running: `docker ps | grep onecli` or `onecli status`

**Maven still sends its own API key**
- Clear `provider.apiKey` in config and remove `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` from the environment

## See also

- [Proxy](proxy.md) — general proxy configuration for Maven
- [OneCLI docs](https://docs.onecli.sh)
