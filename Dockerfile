FROM golang:1.19-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files first for better caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o compiler-server .

# Use a small image for the final container
FROM alpine:3.16

# Install required compilers and tools
RUN apk add --no-cache \
    python3 \
    go \
    gcc \
    g++ \
    musl-dev \
    openjdk11-jdk \
    nodejs \
    npm

# Create a non-root user to run the application
RUN adduser -D -g '' appuser
USER appuser

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/compiler-server .

# Create necessary directories
RUN mkdir -p sandbox

# Expose the port the server listens on
EXPOSE 8001

# Run the application
CMD ["./compiler-server"]

# Keep container running
CMD ["tail", "-f", "/dev/null"]
