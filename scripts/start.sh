#!/bin/bash

# HateSentry Startup Script

set -e

echo "🚀 Starting HateSentry..."

# Check if .env file exists
if [ ! -f .env ]; then
    echo "⚠️  .env file not found. Creating from .env.example..."
    cp .env.example .env
    echo "✅ Created .env file. Please edit it with your configuration before running again."
    exit 1
fi

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker and try again."
    exit 1
fi

# Check if required ports are available
PORTS=(8080 3306 6379 5672 15672 11434)
for port in "${PORTS[@]}"; do
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1 ; then
        echo "⚠️  Port $port is already in use. This might cause conflicts."
    fi
done

# Build and start containers
echo "📦 Building Docker images..."
docker-compose build

echo "🐳 Starting containers..."
docker-compose up -d

# Wait for services to be healthy
echo "⏳ Waiting for services to be ready..."
sleep 10

# Check service health
MAX_RETRIES=30
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if docker-compose ps | grep -q "healthy"; then
        echo "✅ Services are healthy!"
        break
    fi
    echo "⏳ Waiting for services... ($((RETRY_COUNT + 1))/$MAX_RETRIES)"
    sleep 2
    RETRY_COUNT=$((RETRY_COUNT + 1))
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo "⚠️  Services did not become healthy within expected time."
    echo "Run 'docker-compose logs' to check for errors."
fi

# Print service URLs
echo ""
echo "🎉 HateSentry is running!"
echo ""
echo "Service URLs:"
echo "  - API:         http://localhost:8080"
echo "  - Health:      http://localhost:8080/api/v1/health"
echo "  - RabbitMQ UI: http://localhost:15672 (guest/guest)"
echo ""
echo "To view logs:"
echo "  docker-compose logs -f"
echo ""
echo "To stop services:"
echo "  docker-compose down"
echo ""
