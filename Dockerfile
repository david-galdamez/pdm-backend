# --- build stage ---------------------------------------------------------
FROM golang:1.25.0-alpine AS builder

WORKDIR /app

# Layer is cached as long as go.mod/go.sum don't change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0: the postgres driver (pgx) is pure Go, so a static binary
# runs on the minimal alpine base below with no libc surprises.
ENV CGO_ENABLED=0 GOOS=linux

RUN go build -ldflags="-s -w" -o /out/server . && \
    go build -ldflags="-s -w" -o /out/migrate ./cmd/migrations && \
    go build -ldflags="-s -w" -o /out/resetdb ./cmd/resetdb

# --- final stage ----------------------------------------------------------
FROM alpine:3.20

# ca-certificates: TLS to the database (e.g. Neon) and any outbound HTTPS.
# wget: used by the HEALTHCHECK below, no extra image needed for it.
RUN apk add --no-cache ca-certificates wget && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/resetdb /app/resetdb

USER app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/health || exit 1

CMD ["./server"]
