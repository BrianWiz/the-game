# API image. Build context: api/. Static binary (pure-Go SQLite, no CGO) on distroless.
# Designed for a read-only root filesystem: state lives in the /data volume, secrets under
# /run/secrets. Runs as UID 10020 so it can't read other services' secrets.
FROM golang:1.27.1-trixie AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOWORK=off
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian13
COPY --from=build /out/api /usr/local/bin/api
USER 10020:10020
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/api", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/api"]
