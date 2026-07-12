FROM node:24-bookworm-slim AS web-builder
WORKDIR /src/web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.25-bookworm AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=web-builder /src/internal/app/frontend ./internal/app/frontend
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/gsm-panel ./cmd/gsm-panel

FROM debian:bookworm-slim
ARG TARGETARCH
ENV DEBIAN_FRONTEND=noninteractive \
    WEB_BIND=0.0.0.0 \
    WEB_PORT=8080 \
    GSM_DATA_DIR=/data \
    PAL_SERVER_DIR=/home/steam/Steam/steamapps/common/PalServer \
    BACKUP_DIR=/backups \
    TZ=Asia/Shanghai
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl wget tar gzip unzip xz-utils procps net-tools iproute2 bash openjdk-17-jre-headless \
    && if [ "$TARGETARCH" = "amd64" ]; then dpkg --add-architecture i386 && apt-get update && apt-get install -y --no-install-recommends lib32gcc-s1 lib32stdc++6 libc6-i386 libcurl4-gnutls-dev libssl-dev libsdl2-2.0-0 libpulse0 libfontconfig1 libudev1 libvulkan1 libx11-6 libxrandr2 libxi6 libgtk-3-0 libelf1 libatomic1 zlib1g; fi \
    && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /app /data /backups /home/steam/Steam/steamapps/common /usr/local/share/gsm-panel/data
COPY --from=go-builder /out/gsm-panel /usr/local/bin/gsm-panel
COPY data/game_catalog.json /usr/local/share/gsm-panel/data/game_catalog.json
COPY data/plugin_catalog.json /usr/local/share/gsm-panel/data/plugin_catalog.json
COPY data/online_templates.json /usr/local/share/gsm-panel/data/online_templates.json
EXPOSE 8080 8211/udp 8212/tcp 25565/tcp 25575/tcp 2456/udp 2457/udp 7777/tcp 19132/udp
VOLUME ["/data", "/home/steam/Steam/steamapps/common", "/backups"]
WORKDIR /app
ENTRYPOINT ["/usr/local/bin/gsm-panel"]
CMD ["web"]
