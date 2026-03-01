# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS build
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

FROM alpine:3.21
RUN apk add --no-cache \
    bash \
    busybox \
    ca-certificates \
    coreutils \
    python3 \
    py3-pip \
    tzdata \
  && pip3 install --no-cache-dir --break-system-packages apprise \
  && apk del --no-network \
    py3-pip \
    py3-setuptools \
    py3-packaging \
    py3-parsing \
    py3-pip-pyc \
    py3-setuptools-pyc \
    py3-packaging-pyc \
    py3-parsing-pyc \
  && adduser -D -h /home/dtask dtask \
  && mkdir -p /tmp/dtask/logs \
  && rm -rf /root/.cache /tmp/*

COPY --from=build /out/dtask /usr/local/bin/dtask

# root is required for host/compose chroot execution modes
USER root
ENTRYPOINT ["/usr/local/bin/dtask"]
