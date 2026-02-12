FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /herald ./cmd/herald

FROM scratch
COPY --from=builder /herald /herald
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 65534:65534
EXPOSE 8420
HEALTHCHECK --interval=30s --timeout=3s CMD ["/herald", "check"]
ENTRYPOINT ["/herald", "serve"]
