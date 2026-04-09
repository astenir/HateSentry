# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o hatesentry .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/hatesentry .

# Copy config
COPY config/config.yaml ./config/

# Create non-root user
RUN addgroup -g 1000 hatesentry && \
    adduser -D -u 1000 -G hatesentry hatesentry && \
    chown -R hatesentry:hatesentry /app

USER hatesentry

EXPOSE 8080

CMD ["./hatesentry"]
