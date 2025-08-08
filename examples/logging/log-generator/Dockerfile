FROM golang:1.21 AS builder

WORKDIR /app

# Copy the source code
COPY ./examples/logging/log-generator/main.go .

# Build the application
RUN go build -o /log-generator main.go

# Use Ubuntu for the final stage (has bash by default)
FROM ubuntu:22.04

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /log-generator .

# Run the application
CMD ["./log-generator"] 