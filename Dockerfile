FROM golang:1.25.5 AS builder
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w \
  -X github.com/ageneralai/maven/internal/version.Version=${VERSION} \
  -X github.com/ageneralai/maven/internal/version.Commit=${COMMIT} \
  -X github.com/ageneralai/maven/internal/version.Date=${DATE}" \
  -o /maven ./cmd/maven

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
COPY --from=builder /maven /usr/local/bin/maven
RUN mkdir -p /root/.maven/workspace
VOLUME ["/root/.maven"]
EXPOSE 18790 9876 9886
ENTRYPOINT ["maven"]
CMD ["gateway"]
