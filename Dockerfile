# syntax=docker/dockerfile:1

FROM golang:1.25.5-alpine AS api-build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/open-spanner ./cmd/api
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/open-spanner-export-worker ./cmd/export-worker
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/open-spanner-alert-worker ./cmd/alert-worker
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/open-spanner-entitlement-worker ./cmd/entitlement-worker

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S open-spanner \
    && adduser -S -G open-spanner open-spanner \
    && mkdir -p /data/exports \
    && chown -R open-spanner:open-spanner /data

COPY --from=api-build /out/open-spanner /usr/local/bin/open-spanner
COPY --from=api-build /out/open-spanner-export-worker /usr/local/bin/open-spanner-export-worker
COPY --from=api-build /out/open-spanner-alert-worker /usr/local/bin/open-spanner-alert-worker
COPY --from=api-build /out/open-spanner-entitlement-worker /usr/local/bin/open-spanner-entitlement-worker

ENV OPEN_SPANNER_HTTP_ADDR=:18081
ENV OPEN_SPANNER_GRPC_ADDR=:18090
ENV OPEN_SPANNER_DB_DRIVER=sqlite
ENV OPEN_SPANNER_SQLITE_PATH=/data/open-spanner.db
ENV OPEN_SPANNER_EXPORT_STORAGE_PATH=/data/exports
ENV OPEN_SPANNER_EMBEDDED_UI_ENABLED=false

USER open-spanner
VOLUME ["/data"]
EXPOSE 18081 18082 18083 18084 18090

ENTRYPOINT ["/usr/local/bin/open-spanner"]
