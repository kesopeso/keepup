#!/usr/bin/env bash

echo "🚀 Starting KeepUp development environment..."

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Execute from the project root folder
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

# Build and start services
docker compose up -d

echo "✅ Development environment started!"
echo "Frontend: http://localhost:3000"
