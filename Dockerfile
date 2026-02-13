# Build stage
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o herald ./cmd/herald

# Runtime stage
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/herald /herald

EXPOSE 8420

VOLUME ["/data", "/config"]

ENTRYPOINT ["/herald"]
CMD ["serve", "--config", "/config/herald.yaml"]
