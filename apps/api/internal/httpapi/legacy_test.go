package httpapi

import (
	"log/slog"
	"net/http"

	"keepup/apps/api/internal/config"
)

// legacyTestHandler retains protocol regression coverage for shared helpers.
// The application constructor only mounts account-authorized routes.
func legacyTestHandler(logger *slog.Logger, cfg config.AppConfig, db HealthChecker, service RouteService) http.Handler {
	server := newServer(logger, cfg, db)
	server.routes = service
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleRoot)
	mux.HandleFunc("/livez", server.handleLiveness)
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("POST /routes", server.handleCreateRoute)
	mux.HandleFunc("GET /routes/{code}/access", server.handleRouteAccess)
	mux.HandleFunc("POST /routes/{code}/members", server.handleCreateRouteMember)
	mux.HandleFunc("GET /routes/{code}", server.handleRoute)
	mux.HandleFunc("PATCH /routes/{code}", server.handleUpdateRoute)
	mux.HandleFunc("DELETE /routes/{code}", server.handleDeleteRoute)
	mux.HandleFunc("DELETE /routes/{code}/members/me", server.handleLeaveRoute)
	mux.HandleFunc("GET /ws", server.handleWebSocket)
	return server.withCORS(server.withLogging(mux))
}
