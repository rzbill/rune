FROM golang:1.21 AS builder

WORKDIR /app

# Copy the source code
COPY ./examples/rune-hello-world/main.go .

# Build the application
RUN go build -o /rune-hello-world main.go

# Use Ubuntu for the final stage (has bash by default)
FROM ubuntu:22.04

WORKDIR /app

# Install basic utilities for debugging
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    vim \
    nano \
    procps \
    htop \
    && rm -rf /var/lib/apt/lists/*

# Copy the binary from the builder stage
COPY --from=builder /rune-hello-world .

# Create a simple script for interactive testing
RUN echo '#!/bin/bash\necho "Interactive shell for Rune Hello World"\necho "Available commands:"\necho "  - curl localhost:7863/health"\necho "  - curl localhost:7863/"\necho "  - curl localhost:7863/debug"\necho "  - ps aux"\necho "  - env"\necho "  - ls -la"\necho ""\necho "Starting bash..."\nexec bash' > /app/debug.sh && chmod +x /app/debug.sh

# Expose the port
EXPOSE 8080

# Run the application
CMD ["./rune-hello-world"] 