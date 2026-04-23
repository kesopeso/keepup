package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
)

type app struct {
	logger *slog.Logger
	port   string
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	application := &app{
		logger: logger,
		port:   port,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", application.handleRoot)
	mux.HandleFunc("/healthz", application.handleHealth)

	server := &http.Server{
		Addr:    ":" + application.port,
		Handler: application.withCORS(application.withLogging(mux)),
	}

	application.logger.Info("starting api server", "port", application.port)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		application.logger.Error("api server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func (a *app) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	a.writeJSON(w, http.StatusOK, map[string]any{
		"name":    "KeepUp API",
		"status":  "ok",
		"version": "dev",
	})
}

func (a *app) handleHealth(w http.ResponseWriter, r *http.Request) {
	a.writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func (a *app) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		a.logger.Error("failed to write json response", "error", err)
	}
}

func (a *app) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		next.ServeHTTP(w, r)
	})
}

func (a *app) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
