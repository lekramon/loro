# Refresh pins together; see docs/deployment.md.
FROM scratch AS context-audit
COPY . /context

FROM node:24.22.1-trixie-slim@sha256:89c5244bc22a9bc3287d89ebc7e49b9cb5bb84cd40e6175ac3421d5a9f68b95a AS frontend
WORKDIR /src
RUN npm install --global pnpm@11.25.0
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY tsconfig.json ./
COPY web ./web
RUN pnpm run build

FROM golang:1.27.3-trixie@sha256:d9fa6e7ba43678ee1e5d1617007db9a134612032a74e2d86b44a9f582f1564cc AS backend
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOCACHE=/tmp/go-build-cache HOME=/tmp
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=frontend /src/internal/webui/dist ./internal/webui/dist
RUN mkdir -p /out /tmp/go-build-cache && chown 10001:10001 /out /tmp/go-build-cache
USER 10001:10001
RUN go test ./... && go vet ./... && go build -trimpath -ldflags="-s -w" -o /out/piratarr ./cmd/piratarr

FROM debian:trixie-slim@sha256:f6b08b09a05725f9431502d3b5714f5c4576e82c1a84fa1debe0c24a7e0c3ec4 AS runtime
ARG DEBIAN_SNAPSHOT=20261104T000000Z
ARG FFMPEG_VERSION=7:7.1.5-0+deb13u1
ARG CA_CERTIFICATES_VERSION=20250419+deb13u1
RUN rm -f /etc/apt/sources.list.d/debian.sources \
 && printf 'deb [check-valid-until=no] http://snapshot.debian.org/archive/debian/%s trixie main\ndeb [check-valid-until=no] http://snapshot.debian.org/archive/debian-security/%s trixie-security main\n' "$DEBIAN_SNAPSHOT" "$DEBIAN_SNAPSHOT" > /etc/apt/sources.list \
 && apt-get -o Acquire::Retries=3 update \
 && apt-get install --yes --no-install-recommends "ca-certificates=$CA_CERTIFICATES_VERSION" "ffmpeg=$FFMPEG_VERSION" \
 && rm -rf /var/lib/apt/lists/* \
 && groupadd --gid 10001 piratarr \
 && useradd --uid 10001 --gid 10001 --create-home --shell /usr/sbin/nologin piratarr \
 && install -d -o 10001 -g 10001 -m 0700 /config \
 && install -d -o 10001 -g 10001 -m 0755 /data /data/downloads /data/media /data/media/movies
COPY --from=backend /out/piratarr /usr/local/bin/piratarr
ENV PIRATARR_CONTAINER=true PIRATARR_LISTEN=0.0.0.0:8080 PIRATARR_DATA_DIR=/config HOME=/home/piratarr
WORKDIR /app
USER 10001:10001
EXPOSE 8080
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=15s --timeout=5s --start-period=15s --retries=3 CMD ["/usr/local/bin/piratarr", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/piratarr"]
