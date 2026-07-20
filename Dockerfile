# syntax=docker/dockerfile:1.7

FROM golang:1.23-alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X pocket-mvp-backend/internal/buildinfo.Version=${VERSION} -X pocket-mvp-backend/internal/buildinfo.Commit=${COMMIT} -X pocket-mvp-backend/internal/buildinfo.BuildDate=${BUILD_DATE}" \
    -o /out/pocket-api ./cmd/api

FROM alpine:3.21 AS runner
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 pocket \
    && adduser -S -D -H -u 10001 -G pocket pocket

COPY --from=builder /out/pocket-api /app/pocket-api

USER pocket
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:8080/readyz || exit 1

ENTRYPOINT ["/app/pocket-api"]
