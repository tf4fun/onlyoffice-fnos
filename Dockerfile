FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o onlyoffice-connector ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/onlyoffice-connector .
COPY --from=builder /app/web ./web

# Create data directory
RUN mkdir -p /data

EXPOSE 10099

# Environment variables for configuration
ENV DOCUMENT_SERVER_URL=""
ENV DOCUMENT_SERVER_SECRET=""
ENV BASE_URL=""
ENV PORT="10099"
ENV DATA_DIR="/data"

CMD ["./onlyoffice-connector", "-data", "/data", "-port", "10099"]
