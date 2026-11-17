# syntax=docker/dockerfile:1.7

FROM golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS build
WORKDIR /src
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath \
      -ldflags "-s -w -X github.com/kristianvld/dtask/internal/version.Version=${VERSION} -X github.com/kristianvld/dtask/internal/version.Commit=${COMMIT} -X github.com/kristianvld/dtask/internal/version.Date=${BUILD_DATE}" \
      -o /out/dtask ./cmd/dtask

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
COPY requirements.txt /tmp/requirements.txt
RUN apk add --no-cache \
    bash \
    busybox \
    ca-certificates \
    coreutils \
    python3 \
    tzdata \
  && apk add --no-cache --virtual .pip-build py3-pip \
  && pip3 install --require-hashes --no-cache-dir --break-system-packages -r /tmp/requirements.txt \
  && apk del --no-cache .pip-build \
  && adduser -D -h /home/dtask dtask \
  && mkdir -p /tmp/dtask/logs \
  && rm -rf /root/.cache /tmp/*

COPY --from=build /out/dtask /usr/local/bin/dtask

# root is required for host/compose chroot execution modes
USER root
ENTRYPOINT ["/usr/local/bin/dtask"]
