# syntax=docker/dockerfile:1

FROM node:24-alpine AS web-build
WORKDIR /src

COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci

COPY web ./web
RUN mkdir -p internal/ui/static && cd web && npm run build

FROM golang:1.25.5-alpine AS api-build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web-build /src/internal/ui/static ./internal/ui/static

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/open-spanner ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S open-spanner \
    && adduser -S -G open-spanner open-spanner \
    && mkdir -p /data \
    && chown open-spanner:open-spanner /data

COPY --from=api-build /out/open-spanner /usr/local/bin/open-spanner

ENV OPEN_SPANNER_HTTP_ADDR=:18081
ENV OPEN_SPANNER_DB_DRIVER=sqlite
ENV OPEN_SPANNER_SQLITE_PATH=/data/open-spanner.db

USER open-spanner
VOLUME ["/data"]
EXPOSE 18081

ENTRYPOINT ["/usr/local/bin/open-spanner"]
