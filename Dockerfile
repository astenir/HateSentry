# Build the review console assets.
FROM node:22-alpine AS web-builder

ARG NPM_REGISTRY=https://registry.npmmirror.com

WORKDIR /app/web

COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts --registry=${NPM_REGISTRY} --replace-registry-host=always
RUN node node_modules/esbuild/install.js

COPY web/ ./
RUN npm run build

# Build the API binary.
FROM golang:1.24-alpine AS builder

ARG GOPROXY=https://goproxy.cn
ENV GOPROXY=${GOPROXY}

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o hatesentry .

# Final stage
FROM scratch

WORKDIR /app

# Copy CA certificates for HTTPS provider and webhook calls.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary from builder
COPY --from=builder --chown=1000:1000 /app/hatesentry .

# Copy config
COPY --chown=1000:1000 config/config.yaml ./config/

# Copy the same-origin review console.
COPY --from=web-builder --chown=1000:1000 /app/web/dist ./web/dist

USER 1000:1000

EXPOSE 8080

CMD ["./hatesentry"]
