# OneCLI vault

[OneCLI](https://github.com/onecli/onecli) is an open-source agent vault. It sits between AI agents and external APIs: secrets live in an encrypted vault, an HTTPS proxy injects them at request time, and your agent process never holds raw API keys.

Maven has zero OneCLI-specific code. It works because Maven uses `http.DefaultClient`, which respects `HTTPS_PROXY` and `SSL_CERT_FILE`.

## Architecture

```mermaid
flowchart LR
    Maven[Maven process<br/>http.DefaultClient + HTTPS_PROXY] -->|TLS via gateway CA| GW[OneCLI gateway :10255]
    GW -->|inject API key from vault| Upstream[api.anthropic.com / api.openai.com / …]
    Operator[Operator] -->|browser| Dash[Dashboard :10254]
    Dash --> Vault[(Encrypted vault)]
    GW --> Vault
```

Port layout:

- **`:10254`** — dashboard (web UI).
- **`:10255`** — gateway (HTTPS proxy).

## Prerequisites

- Maven built and configured ([Get started](../getting-started.md)).
- Docker, or native OneCLI install.

## 1. Start OneCLI

=== "Docker"

    ```bash
    docker run -d \
      --name onecli \
      -p 10254:10254 \
      -p 10255:10255 \
      -v onecli-data:/app/data \
      ghcr.io/onecli/onecli
    ```

=== "Native"

    ```bash
    curl -fsSL https://onecli.sh/install | sh
    ```

Verify both services:

```bash
curl -sf http://127.0.0.1:10254/v1/health    # dashboard
curl -sf http://127.0.0.1:10255/healthz       # gateway
```

## 2. Add credentials and get an agent token

Open the dashboard at **<http://127.0.0.1:10254>**:

1. **Secrets:** add your Anthropic (or OpenAI) API key.
2. **Agents:** open the default agent and copy its access token (`aoc_…`).

The gateway authenticates each proxied request by this token and injects the matching credential.

## 3. Trust OneCLI's CA

OneCLI terminates TLS to inject credentials. Go reads `SSL_CERT_FILE` natively, so no custom code is needed:

=== "Docker install"

    ```bash
    export SSL_CERT_FILE=/path/to/onecli-data/gateway/ca.pem
    ```

=== "Native install"

    ```bash
    export SSL_CERT_FILE=~/.onecli/gateway/ca.pem
    ```

Alternatively, install the CA into your OS trust store and omit `SSL_CERT_FILE`.

## 4. Configure Maven

Set `provider.apiKey` to a non-empty placeholder; OneCLI replaces it at the gateway:

```json
{
  "provider": {
    "type": "anthropic",
    "apiKey": "placeholder",
    "baseUrl": "https://api.anthropic.com"
  }
}
```

Start Maven with the proxy env, embedding the agent token in the URL:

```bash
export HTTPS_PROXY=http://x:aoc_YOUR_TOKEN@127.0.0.1:10255
export SSL_CERT_FILE=~/.onecli/gateway/ca.pem
./maven gateway
```

The `x:TOKEN` form is HTTP Basic auth — `x` is a dummy username, the token is the password.

## 5. Verify

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

`provider.apiKey` in config must still be a non-empty string (e.g. `"placeholder"`); the gateway replaces it.

## Troubleshooting

| Symptom | Resolution |
|---------|------------|
| `x509: certificate signed by unknown authority` | `SSL_CERT_FILE` not set or wrong path. Use `~/.onecli/gateway/ca.pem` (native) or the Docker volume path. |
| `401` from upstream API | Secret not configured in the vault, or the agent token in `HTTPS_PROXY` is wrong. Check the dashboard. |
| Connection refused on `:10255` | OneCLI gateway not running. `docker ps | grep onecli`. |
| `provider.apiKey is required` | Maven validates it; set to any non-empty string. |
| Maven still uses its own key (401) | Remove `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` from the environment so Maven doesn't read them as the real key. |

## See also

- [Proxy](proxy.md) — general process-level egress.
- [OneCLI docs](https://onecli.sh/docs).
