package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"keepup/apps/api/internal/auth"
	"keepup/apps/api/internal/live"
	"keepup/apps/api/internal/routes"
)

type routeSessionKey struct{}

func routeSession(r *http.Request) auth.Session {
	return r.Context().Value(routeSessionKey{}).(auth.Session)
}

func (s *Server) accountHTTPHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /routes", s.accountCreate)
	mux.HandleFunc("GET /routes/{code}/access", s.accountAccess)
	mux.HandleFunc("POST /routes/{code}/members", s.accountJoin)
	mux.HandleFunc("GET /routes/{code}", s.accountSnapshot)
	mux.HandleFunc("PATCH /routes/{code}", s.accountUpdate)
	mux.HandleFunc("DELETE /routes/{code}", s.accountDelete)
	mux.HandleFunc("DELETE /routes/{code}/members/me", s.accountLeave)
	mux.HandleFunc("POST /routes/{code}/socket-ticket", s.accountTicket)
	mux.HandleFunc("GET /me/routes", s.accountList)
	protected := s.accountTransport.protect(s.requireRouteAccount(mux))
	root := http.NewServeMux()
	root.Handle("/routes", protected)
	root.Handle("/routes/", protected)
	root.Handle("/me/", protected)
	root.HandleFunc("GET /ws", s.accountSocket)
	root.HandleFunc("/livez", s.handleLiveness)
	root.HandleFunc("/healthz", s.handleHealth)
	root.HandleFunc("/", s.handleRoot)
	return s.withLogging(root)
}

func (s *Server) requireRouteAccount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := s.accountTransport
		token := h.token(r)
		session, err := h.service.Authenticate(r.Context(), token)
		if err == nil {
			err = auth.RequireActive(session)
		}
		if err != nil {
			h.writeError(w, err)
			return
		}
		if !h.native(r) {
			h.setCookie(w, token, session.ExpiresAt)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeSessionKey{}, session)))
	})
}

func (s *Server) accountRouteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrSession), errors.Is(err, auth.ErrPending), errors.Is(err, auth.ErrRateLimited):
		s.accountTransport.writeError(w, err)
	case errors.Is(err, routes.ErrUnauthorized):
		s.accountTransport.fail(w, 403, "unauthorized")
	default:
		s.writeRouteError(w, err)
	}
}

func (s *Server) accountCreate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TransportMode string `json:"transportMode"`
		Name          string `json:"name"`
		Description   string `json:"description"`
		Password      string `json:"password"`
		SharingPolicy string `json:"sharingPolicy"`
	}
	if err := accountDecode(w, r, &input); err != nil {
		s.accountTransport.writeError(w, err)
		return
	}
	result, err := s.accountRoutes.CreateRoute(r.Context(), routeSession(r), routes.CreateRouteInput{TransportMode: input.TransportMode, Name: input.Name, Description: input.Description, Password: input.Password, SharingPolicy: input.SharingPolicy})
	if err != nil {
		s.accountRouteError(w, err)
		return
	}
	s.writeJSON(w, 201, result)
}
func (s *Server) accountAccess(w http.ResponseWriter, r *http.Request) {
	result, err := s.accountRoutes.AccessRoute(r.Context(), routeSession(r), r.PathValue("code"))
	if err != nil {
		s.accountRouteError(w, err)
		return
	}
	s.writeJSON(w, 200, result)
}
func (s *Server) accountJoin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TransportMode string `json:"transportMode"`
		Password      string `json:"password"`
	}
	if err := accountDecode(w, r, &input); err != nil {
		s.accountTransport.writeError(w, err)
		return
	}
	result, err := s.accountRoutes.JoinRoute(r.Context(), routeSession(r), r.PathValue("code"), routes.JoinRouteInput{TransportMode: input.TransportMode, Password: input.Password})
	if err != nil {
		s.accountRouteError(w, err)
		return
	}
	if !result.Restored {
		s.broadcastLiveEvent(result.Route.ID, live.Event{"type": "member_joined", "member": result.Member})
	}
	s.writeJSON(w, 201, result)
}
func (s *Server) accountSnapshot(w http.ResponseWriter, r *http.Request) {
	result, err := s.accountRoutes.Snapshot(r.Context(), routeSession(r), r.PathValue("code"))
	if err != nil {
		s.accountRouteError(w, err)
		return
	}
	s.writeJSON(w, 200, result)
}
func (s *Server) accountUpdate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	if err := accountDecode(w, r, &input); err != nil {
		s.accountTransport.writeError(w, err)
		return
	}
	result, err := s.accountRoutes.UpdateRoute(r.Context(), routeSession(r), r.PathValue("code"), routes.UpdateRouteInput{Name: input.Name, Description: input.Description, Status: input.Status})
	if err != nil {
		s.accountRouteError(w, err)
		return
	}
	event := "route_updated"
	if result.Status == routes.RouteStatusClosed {
		event = "route_closed"
	}
	s.broadcastLiveEvent(result.ID, live.Event{"type": event, "route": result})
	s.writeJSON(w, 200, result)
}
func (s *Server) accountLeave(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength != 0 {
		if err := accountDecode(w, r, &struct{}{}); err != nil {
			s.accountTransport.writeError(w, err)
			return
		}
	}
	result, err := s.accountRoutes.LeaveRoute(r.Context(), routeSession(r), r.PathValue("code"))
	if err != nil {
		s.accountRouteError(w, err)
		return
	}
	s.broadcastLiveEvent(result.Member.RouteID, live.Event{"type": "member_left", "member": result.Member})
	s.writeJSON(w, 200, result)
}
func (s *Server) accountDelete(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength != 0 {
		if err := accountDecode(w, r, &struct{}{}); err != nil {
			s.accountTransport.writeError(w, err)
			return
		}
	}
	if err := s.accountRoutes.DeleteRoute(r.Context(), routeSession(r), r.PathValue("code")); err != nil {
		s.accountRouteError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) accountTicket(w http.ResponseWriter, r *http.Request) {
	if err := accountDecode(w, r, &struct{}{}); err != nil {
		s.accountTransport.writeError(w, err)
		return
	}
	result, err := s.accountRoutes.IssueSocketTicket(r.Context(), routeSession(r), r.PathValue("code"))
	if err != nil {
		s.accountRouteError(w, err)
		return
	}
	s.writeJSON(w, 200, result)
}

func listRouteInput(values url.Values) (routes.ListRoutesInput, error) {
	input := routes.ListRoutesInput{View: "active", Owned: true, Joined: true, Limit: 20}
	for name, entries := range values {
		if len(entries) != 1 {
			return input, routes.ErrInvalidInput
		}
		value := entries[0]
		switch name {
		case "view":
			if value != "active" && value != "archived" {
				return input, routes.ErrInvalidInput
			}
			input.View = value
		case "owned", "joined", "left":
			if value != "true" && value != "false" {
				return input, routes.ErrInvalidInput
			}
			switch name {
			case "owned":
				input.Owned = value == "true"
			case "joined":
				input.Joined = value == "true"
			case "left":
				input.Left = value == "true"
			}
		case "limit":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 100 {
				return input, routes.ErrInvalidInput
			}
			input.Limit = n
		case "cursor":
			if value == "" {
				return input, routes.ErrInvalidInput
			}
			input.Cursor = value
		default:
			return input, routes.ErrInvalidInput
		}
	}
	return input, nil
}
func (s *Server) accountList(w http.ResponseWriter, r *http.Request) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		s.accountRouteError(w, routes.ErrInvalidInput)
		return
	}
	input, err := listRouteInput(values)
	if err != nil {
		s.accountRouteError(w, err)
		return
	}
	result, err := s.accountRoutes.ListRoutes(r.Context(), routeSession(r), input)
	if err != nil {
		s.accountRouteError(w, err)
		return
	}
	s.writeJSON(w, 200, result)
}

func (s *Server) accountSocket(w http.ResponseWriter, r *http.Request) {
	h := s.accountTransport
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	allowed := h.native(r)
	if r.Header.Get("X-KeepUp-Client") == "" && r.Header.Get("Authorization") == "" {
		for _, origin := range h.cfg.Origins {
			if r.Header.Get("Origin") == origin {
				allowed = true
			}
		}
	}
	if !allowed {
		h.fail(w, 403, "invalid_transport")
		return
	}
	if r.URL.RawQuery != "" || r.Header.Get("Authorization") != "" {
		h.fail(w, 400, "invalid_input")
		return
	}
	s.handleAccountWebSocket(w, r)
}
