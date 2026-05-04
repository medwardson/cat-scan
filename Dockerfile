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
      .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

LABEL org.opencontainers.image.title="ailurophile"
LABEL org.opencontainers.image.description="Puma metrics sidecar for CloudWatch and OTLP"

COPY --from=builder /ailurophile /ailurophile

EXPOSE 8090

ENTRYPOINT ["/ailurophile"]
