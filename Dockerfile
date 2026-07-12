# Jabali Sounder — headless server image.
# Single static binary that serves both the API and the SPA on one port.
# SQLite by default (pure-Go driver, no libc), so this runs CGO-free on musl.
#
#   docker build -t jabali-sounder .
#   docker run -p 8484:8484 -v sounder-data:/data \
#     -e JABALI_SOUNDER_ADMIN_PASSWORD=change-me jabali-sounder
#
# syntax=docker/dockerfile:1

# --- Stage 1: build the SPA -------------------------------------------------
FROM node:22-alpine AS ui
WORKDIR /ui
COPY manager-ui/package.json manager-ui/package-lock.json ./
RUN npm ci
COPY manager-ui/ ./
RUN npm run build

# --- Stage 2: build the static server binary --------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Embed the freshly built SPA where the embedui build tag picks it up.
COPY --from=ui /ui/dist ./manager-api/cmd/server/dist
ARG VERSION=docker
ARG COMMIT=none
ARG DATE=unknown
ENV VPKG=git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/version
RUN CGO_ENABLED=0 go build -tags "embedui,nomsgpack" -trimpath \
      -ldflags "-s -w -X ${VPKG}.Version=${VERSION} -X ${VPKG}.Commit=${COMMIT} -X ${VPKG}.Date=${DATE}" \
      -o /out/jabali-sounder-server ./manager-api/cmd/server

# --- Stage 3: minimal runtime ----------------------------------------------
FROM alpine:3.20
# ca-certificates: outbound TLS (update checks, webhooks, managed panels).
# openssl: generate the encryption key + JWT secret on first run.
RUN apk add --no-cache ca-certificates openssl \
 && adduser -D -H -u 10001 sounder \
 && mkdir -p /data && chown sounder:sounder /data
COPY --from=build /out/jabali-sounder-server /usr/local/bin/jabali-sounder-server
COPY docker-entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Container defaults: bind all interfaces, SQLite + secrets under the /data
# volume. Front with a TLS reverse proxy for anything public.
ENV JABALI_SOUNDER_ENV=production \
    JABALI_SOUNDER_ADDR=0.0.0.0:8484 \
    JABALI_SOUNDER_DATABASE_DRIVER=sqlite \
    JABALI_SOUNDER_DATABASE_URL=/data/sounder.db \
    JABALI_SOUNDER_SECRET_KEY_FILE=/data/secrets.key

USER sounder
WORKDIR /data
VOLUME /data
EXPOSE 8484
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD wget -qO- http://127.0.0.1:8484/health >/dev/null 2>&1 || exit 1
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["serve"]
