# Dedicated server image. Expects `game/scripts/export.sh server` to have produced build/server/.
FROM debian:trixie-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates libfontconfig1 \
  && rm -rf /var/lib/apt/lists/* \
  && useradd --system --uid 10010 --home /srv/game game
WORKDIR /srv/game
COPY build/server/ ./
USER game
EXPOSE 7777
ENTRYPOINT ["./the-game-server.x86_64", "--headless", "--"]
CMD ["--server", "--port=7777"]
