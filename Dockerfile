FROM golang:1.25.5 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /maven ./cmd/maven

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
COPY --from=builder /maven /usr/local/bin/maven
RUN mkdir -p /root/.maven/workspace
VOLUME ["/root/.maven"]
EXPOSE 18790 9876 9886
ENTRYPOINT ["maven"]
CMD ["gateway"]
