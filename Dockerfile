FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN set -eux; \
  go mod download

COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN set -eux; \
  CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
      -ldflags="-s -w" \
      -o /ailurophile \
      . && \
  mkdir -p /rootfs/run

FROM scratch

LABEL org.opencontainers.image.title="ailurophile"
LABEL org.opencontainers.image.description="Puma metrics sidecar for CloudWatch and OTLP"

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /rootfs/ /
COPY --from=builder /ailurophile /ailurophile

ENTRYPOINT ["/ailurophile"]
