# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

KeepUp is a real-time location tracking and trip sharing application with a Next.js frontend and Go backend. The app allows users to create trips, invite friends, and track locations in real-time on maps.

## Development Commands

### Starting Development Environment
```bash
./scripts/dev.sh start
```
This starts all services including the frontend (Next.js), backend (Go/Gin), PostgreSQL database with PostGIS extension, and pgAdmin.

### Stopping Development Environment
```bash
./scripts/dev.sh stop
```

### Frontend Development (within container)
```bash
cd frontend
npm run dev          # Start development server with Turbopack
npm run build        # Build for production
npm run start        # Start production server
npm run lint         # Run ESLint
```

### Backend Development (within container)
```bash
cd backend
go run main.go       # Run development server
go build            # Build binary
go mod tidy          # Clean up dependencies
```

### Accessing Services
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080
- pgAdmin (PostgreSQL admin): http://localhost:5431

## Architecture

### Containerized Development
- **Docker Compose**: All services run in containers for consistent development environment
- **Frontend**: Next.js 15.3.4 app in Node.js 24 Alpine container with hot reload
- **Backend**: Go 1.24.5 with Gin framework in multi-stage Docker build
- **Database**: PostgreSQL 17 with PostGIS 3.5 extension for geospatial data
- **Database Admin**: pgAdmin 4 for database management

### Frontend Structure
- **Framework**: Next.js 15.3.4 with TypeScript 5.8 and App Router
- **Styling**: Tailwind CSS v4 with shadcn/ui components
- **UI Components**: shadcn/ui with Radix UI primitives and Lucide React icons
- **Fonts**: Geist Sans and Geist Mono from Google Fonts

### Backend Structure
- **Framework**: Go 1.24.5 with Gin web framework
- **Database**: PostgreSQL with lib/pq driver
- **API**: RESTful API with `/api/v1` prefix
- **CORS**: Enabled for all origins in development

### Code Organization
```
frontend/src/
├── app/                 # Next.js App Router pages
│   ├── layout.tsx      # Root layout with fonts and metadata
│   ├── page.tsx        # Landing page
│   └── globals.css     # Global styles
├── components/         # React components
│   └── ui/            # shadcn/ui components (Button, Card, etc.)
└── lib/               # Utilities and helpers
    └── utils.ts       # Tailwind CSS utility functions

backend/
├── main.go            # Main server file with all routes (to be refactored)
├── go.mod             # Go module dependencies
└── Dockerfile         # Multi-stage Docker build
```

### Database
- **PostgreSQL 17** with PostGIS extension for geospatial operations
- Persistent data stored in `./data/postgres/`
- Development credentials: `keepup/keepup/keepup` (user/password/database)

## Code Style and Configuration

### TypeScript
- Strict mode enabled
- Path aliases: `@/*` maps to `./src/*`
- Target: ES2017 with Next.js plugin

### ESLint
- Extends Next.js core web vitals and TypeScript recommended configs
- Prettier integration for code formatting

### Prettier
- 4-space indentation, single quotes, 80 character line width
- Trailing commas (ES5), arrow function parentheses always

## Key Features (Based on Landing Page)
- Real-time location tracking with interactive maps
- Group trip sharing with password-based invitations
- Trip replay functionality with playback controls
- Authentication system (routes to `/auth/signin`)

## API Endpoints

The backend provides these basic endpoints:
- `GET /health` - Health check with database status
- `GET /api/v1/ping` - Simple ping endpoint
- `GET /api/v1/trips` - List trips (placeholder)
- `GET /api/v1/users/me` - Get current user (placeholder)

## Development Notes
- Frontend uses Turbopack for faster development builds
- Component library follows shadcn/ui conventions with "new-york" style
- Icons sourced from Lucide React
- Database includes geospatial capabilities via PostGIS for location features
- Backend currently has all code in main.go (ready for refactoring into separate files)

## Recent Changes
- Frontend now has its own Dockerfile.dev
- Air was added to the backend container setup for HMR