# --- Stage 1: Build the Go binary ---
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy dependency files first to leverage Docker caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build a fully statically-linked binary. 
# CGO_ENABLED=0 removes any need for local C compilers (gcc/musl-dev).
RUN CGO_ENABLED=0 go build -o bin/start-menu-service ./cmd/main.go

# --- Stage 2: Final ultra-lightweight runtime ---
FROM alpine:3.19

RUN apk add --no-cache ca-certificates curl

RUN mkdir -p /etc/ssl/certs/custom

RUN curl -sL https://letsencrypt.org/certs/isrgrootx1.pem -o /etc/ssl/certs/custom/isrgrootx1.pem

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/bin/start-menu-service ./bin/start-menu-service

# Expose the port Gin is listening on
EXPOSE 8082

# Set Gin to release mode by default
ENV GIN_MODE=release

# Command to run the application
CMD ["./bin/start-menu-service"]
