#!/usr/bin/env bash

# Check if argument is provided
if [ $# -eq 0 ]; then
    echo "❌ Error: No argument provided"
    echo ""
    echo "Usage: $0 [start|stop]"
    echo "  start: Start development environment"
    echo "  stop: Stop development environment"
    exit 1
fi

ACTION="$1"

# Setup pgadmin folder with proper permissions
setup_pgadmin_folder() {
    if [ ! -d "./data/pgadmin" ]; then
        echo "📁 Creating data/pgadmin folder with proper permissions..."
        mkdir -p ./data/pgadmin
        sudo chown 5050:0 ./data/pgadmin
        echo "✅ data/pgadmin folder created successfully!"
    fi
}

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Execute from the project root folder
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

case "$ACTION" in
    start)
        echo "🚀 Starting KeepUp development environment..."
        
        # Check if services are not running and start them
        if ! docker compose ps --services --filter "status=running" | grep -q .; then
            setup_pgadmin_folder
            docker compose up -d
        fi
        
        echo "✅ Development environment started!"
        echo "Frontend: http://localhost:3000"
        echo "Postgres DB admin: http://localhost:5431"
        ;;
    stop)
        echo "🛑 Stopping KeepUp development environment..."
        
        # Check if services are running and stop them
        if docker compose ps --services --filter "status=running" | grep -q .; then
            docker compose down
        fi
        
        echo "✅ Development environment stopped!"
        ;;
    *)
        echo "❌ Error: Invalid argument '$ACTION'"
        echo ""
        echo "Usage: $0 [start|stop]"
        echo "  start: Start development environment"
        echo "  stop: Stop development environment"
        exit 1
        ;;
esac
