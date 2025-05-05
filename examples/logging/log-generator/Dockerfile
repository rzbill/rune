FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy the source code
COPY ./examples/logging/log-generator/main.go .

# Build the application
RUN go build -o /log-generator main.go

# Use a smaller image for the final stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /log-generator .

# Run the application
CMD ["./log-generator"] 