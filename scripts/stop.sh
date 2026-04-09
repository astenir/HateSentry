#!/bin/bash

# HateSentry Stop Script

set -e

echo "🛑 Stopping HateSentry..."

docker-compose down

echo "✅ HateSentry stopped!"
