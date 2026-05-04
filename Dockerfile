FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o terminal-endpoint ./cmd/terminal-endpoint/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates bash

RUN adduser -D -s /bin/bash endpoint

COPY --from=builder /build/terminal-endpoint /usr/local/bin/terminal-endpoint

USER endpoint

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["terminal-endpoint"]
