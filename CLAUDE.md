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
- **Authentication**: Client-side auth with localStorage token storage
- **Fonts**: Geist Sans and Geist Mono from Google Fonts

### Backend Structure
- **Framework**: Go 1.24.5 with Gin web framework
- **Database**: PostgreSQL with lib/pq driver and golang-migrate for migrations
- **Authentication**: JWT tokens with bcrypt password hashing
- **API**: RESTful API with `/api/v1` prefix
- **CORS**: Enabled for all origins in development
- **Hot Reload**: Air for automatic rebuilds during development

### Code Organization
```
frontend/src/
├── app/                 # Next.js App Router pages
│   ├── auth/           # Authentication pages
│   │   ├── login/      # Login page
│   │   └── signup/     # Signup page
│   ├── dashboard/      # Protected dashboard page
│   ├── layout.tsx      # Root layout with fonts and metadata
│   ├── page.tsx        # Landing page
│   └── globals.css     # Global styles
├── components/         # React components
│   └── ui/            # shadcn/ui components (Button, Card, Input, Label)
└── lib/               # Utilities and helpers
    └── utils.ts       # Tailwind CSS utility functions

backend/
├── main.go            # Main server file with authentication and user management
├── migrations/        # Database migration files
│   ├── 000001_create_users_table.up.sql
│   ├── 000001_create_users_table.down.sql
│   ├── 000002_add_email_to_users.up.sql
│   └── 000002_add_email_to_users.down.sql
├── go.mod             # Go module dependencies
├── .air.toml          # Air configuration for hot reload
└── Dockerfile.dev     # Development Docker build
```

### Database
- **PostgreSQL 17** with PostGIS extension for geospatial operations
- **Migrations**: golang-migrate/migrate for schema versioning
- **Schema Tracking**: Automatic `schema_migrations` table for version control
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

## Key Features
- **Authentication System**: Complete user registration and login with email/password
- **User Dashboard**: Protected dashboard displaying user information
- Real-time location tracking with interactive maps (planned)
- Group trip sharing with password-based invitations (planned)
- Trip replay functionality with playback controls (planned)

## API Endpoints

### Authentication
- `POST /api/v1/auth/signup` - User registration with email/password (username defaults to email)
- `POST /api/v1/auth/login` - User login with email/password, returns JWT tokens

### Protected Routes (Require Bearer Token)
- `GET /api/v1/users/me` - Get current authenticated user

### General
- `GET /health` - Health check with database status
- `GET /api/v1/ping` - Simple ping endpoint
- `GET /api/v1/trips` - List trips (placeholder)

## Security & Authentication
- **Password Hashing**: bcrypt with automatic salt generation
- **JWT Tokens**: Short-lived access tokens (15 min) + long-lived refresh tokens (7 days)
- **Input Validation**: Email format validation and password strength requirements (8+ chars)
- **SQL Injection Protection**: Parameterized queries throughout
- **Middleware Protection**: JWT middleware for protected routes
- **Client-side Auth**: Token storage in localStorage with automatic redirects

## Development Notes
- Frontend uses Turbopack for faster development builds
- Component library follows shadcn/ui conventions with "new-york" style
- Icons sourced from Lucide React
- Database includes geospatial capabilities via PostGIS for location features
- Backend uses Air for hot reload during development
- Migrations run automatically on backend startup
- Database schema versioned with golang-migrate
- Authentication pages built with form validation and error handling
- Protected routes implement client-side authentication checks

## Environment Variables
- `JWT_SECRET` - Secret key for JWT token signing (use strong secret in production)
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` - Database connection settings

## Authentication Flow
1. **Signup**: User registers with email/password → username defaults to email → redirected to dashboard
2. **Login**: User authenticates with email/password → JWT tokens stored in localStorage → redirected to dashboard
3. **Protected Routes**: Dashboard checks for access token → displays "Hello `<username>`!" message
4. **Logout**: Clears tokens from localStorage → redirects to login page

## Available Routes
- `/` - Landing page with links to auth pages
- `/auth/signup` - User registration form
- `/auth/login` - User login form  
- `/dashboard` - Protected dashboard displaying user info