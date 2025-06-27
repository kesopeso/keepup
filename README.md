# KeepUp

Real-time location tracking and trip sharing application.

## Tech Stack

- **Frontend**: Next.js 15.3.4, TypeScript 5.8, Tailwind CSS, shadcn/ui
- **Backend**: Go 1.24.4, Gin, PostgreSQL with PostGIS, Redis
- **Infrastructure**: Docker Compose for local development

## Quick Start

1. **Start development environment:**
   ```bash
   ./scripts/dev.sh
   ```

2. **Run database migrations:**
   ```bash
   ./scripts/migrate.sh
   ```

3. **Access the application:**
   - Frontend: http://localhost:3000
   - Backend API: http://localhost:8080
   - Database: localhost:5432

## Architecture

- **Real-time tracking**: WebSockets with Redis pub/sub
- **Geospatial data**: PostgreSQL with PostGIS extension  
- **Migrations**: golang-migrate for database schema management
- **Hot reload**: Air for Go backend, Next.js dev server for frontend

## Development Commands

```bash
# Start all services
docker-compose up

# Run migrations
./scripts/migrate.sh up

# Reset database
./scripts/migrate.sh down
./scripts/migrate.sh up

# View logs
docker-compose logs -f [service-name]
```