FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum
COPY ../../go.mod ../../go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . ./examples/logging/

# Build the application
RUN go build -o /logging-service ./examples/logging/main.go

# Use a smaller image for the final stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /logging-service .

# Run the application
CMD ["./logging-service"] 