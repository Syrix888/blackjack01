
# Start with a minimal Go builder image
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy Go source code
COPY container_src/go.mod ./
RUN go mod download

COPY container_src/*.go ./

# Build the Go binary (static build, no cgo)
RUN CGO_ENABLED=0 GOOS=linux go build -o blackjack-server main.go

# ---

# Use a minimal final image
FROM alpine:3.19

WORKDIR /app

# Copy built Go binary from builder
COPY --from=builder /app/blackjack-server .

# Expose port 8080 for Cloudflare container runtime
EXPOSE 8080

# Run the server
CMD ["./blackjack-server"]
