# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev sudo

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o reconya-api ./cmd/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata sqlite nmap sudo

# Set working directory
WORKDIR /app

# Copy binary from build stage
COPY --from=builder /app/reconya-api /app/

# Environment files will be passed through environment variables in docker-compose

# Create directories
RUN mkdir -p /app/data

# Expose port
EXPOSE 3008

# Command to run the application
CMD ["/app/reconya-api"]