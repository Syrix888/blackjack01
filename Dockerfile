FROM golang:1.22-alpine AS builder

WORKDIR /app

# If go.mod exists, copy it (optional, since no external modules)
# COPY container_src/go.mod ./
# RUN go mod download

COPY container_src/*.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o blackjack-server main.go

FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/blackjack-server .

EXPOSE 8080

CMD ["./blackjack-server"]
