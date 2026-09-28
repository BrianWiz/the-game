# Bot image. Build context: bot/. Static binary (pure-Go SQLite, no CGO) on distroless.
# Designed for a read-only root filesystem: state lives in the /data volume, secrets under
# /run/secrets. Runs as UID 10040 so it can't read other services' secrets.
FROM golang:1.27.1-trixie AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOWORK=off
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot

FROM gcr.io/distroless/static-debian13
COPY --from=build /out/bot /usr/local/bin/bot
USER 10040:10040
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD ["/usr/local/bin/bot", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/bot"]
