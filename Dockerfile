# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go-server/go.mod go-server/go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY go-server/ ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o gows main.go hub.go

# Run stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/gows .

# Copy example config (user should mount their own config.yaml or rename this)
COPY go-server/config.example.yaml ./config.yaml

EXPOSE 8080

CMD ["./gows"]
