# Docker

Maven ships with a `Dockerfile` and `docker-compose.yml`. The container exposes the gateway port (`18790`) and per-channel webhook ports (`9876` Feishu, `9886` WeCom). All persistent state lives under `/root/.maven` — mount a volume there.

## Build

```bash
docker build -t maven .
```

The build is a two-stage `golang:1.25.5` → `debian:bookworm-slim`. CGO is disabled; the binary stamps version via Go ldflags from `VERSION`, `COMMIT`, and `DATE` build args.

```bash
docker build \
  --build-arg VERSION=$(git describe --tags --always --dirty) \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -t maven:dev .
```

## Run

```bash
docker run -d \
  --name maven \
  -p 18790:18790 \
  -p 9876:9876 \
  -p 9886:9886 \
  -v maven-data:/root/.maven \
  maven
```

The first run starts the gateway but exits if there is no `/root/.maven/config.json`. Either:

- Mount your config in: `-v $HOME/.maven/config.json:/root/.maven/config.json:ro`
- Or `docker exec` into the container and run `maven onboard` (less common in containers).

The recommended pattern: keep `~/.maven/` on the host as the volume source.

```bash
docker run -d \
  --name maven \
  -p 18790:18790 \
  -v $HOME/.maven:/root/.maven \
  maven
```

## docker-compose

The repository's `docker-compose.yml`:

```yaml
services:
  maven:
    build: .
    restart: unless-stopped
    ports:
      - "18790:18790"
      - "9876:9876"
      - "9886:9886"
    volumes:
      - maven-data:/root/.maven

  tunnel:
    image: cloudflare/cloudflared:latest
    restart: unless-stopped
    profiles: ["tunnel"]
    command: tunnel --url http://maven:9876
    depends_on:
      - maven

volumes:
  maven-data:
```

```bash
docker compose up -d --build         # gateway only
docker compose --profile tunnel up -d # gateway + cloudflared tunnel
docker compose logs -f maven
docker compose down
```

The `tunnel` profile is opt-in and points cloudflared at the Feishu webhook port. Use the tunnel URL as your Feishu event subscription endpoint.

## Environment variables

Pass any of the following via `-e` or `environment:`:

| Variable | Purpose |
|----------|---------|
| `HTTPS_PROXY` | Outbound HTTPS proxy (LLM, channels). |
| `HTTP_PROXY` | Fallback when `HTTPS_PROXY` unset. |
| `NO_PROXY` | Bypass hosts. |
| `SSL_CERT_FILE` | CA bundle path; required for MITM proxies (e.g. OneCLI). |
| `DEEPGRAM_API_KEY`, `OPENAI_API_KEY`, `ELEVENLABS_API_KEY`, `CARTESIA_API_KEY` | Voice provider credentials. |
| `ELEVENLABS_VOICE_ID`, `CARTESIA_VOICE_ID` | Required for those providers. |

`MAVEN_*_API_KEY` overrides exist for each provider; see [Reference: Environment](../reference/environment.md).

## Hot reload in containers

`gateway.hotReload` uses `fsnotify` against the parent directory of the config file. Docker Desktop's bind mounts can swallow events; prefer:

- A volume mount of the **directory** (`-v $HOME/.maven:/root/.maven`), not a single-file bind.
- Or rebuild/restart on config changes if reload doesn't fire.

## Production checklist

- [ ] Mount `/root/.maven` as a persistent volume.
- [ ] Set `HTTPS_PROXY` and `SSL_CERT_FILE` if egress is gated by a vault (see [OneCLI](onecli.md)).
- [ ] Use a reverse proxy (Caddy, nginx, Cloudflare Tunnel) for TLS termination if exposing channels.
- [ ] Restrict `channels.*.allowFrom` to expected senders.
- [ ] Set `gateway.host = "127.0.0.1"` and let the reverse proxy front it if you only want local binding.
- [ ] Run the container as a non-root user in production (currently the base image is rootful; bake a non-root layer if needed).

## Healthcheck

There is no HTTP healthcheck endpoint today. Use process liveness (`docker inspect ... | jq '.[0].State.Status'`) or rely on the `health.HealthReporter` if you wire your own observer.
