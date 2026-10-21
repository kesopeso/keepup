package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"keepup/apps/api/internal/auth"
	"keepup/apps/api/internal/config"
)

// AccountService is the account API's authority boundary.
type AccountService interface {
	Signup(context.Context, auth.SignupInput, string) (auth.Result, error)
	Login(context.Context, string, string, string) (auth.Result, error)
	Authenticate(context.Context, string) (auth.Session, error)
	Logout(context.Context, string, bool) error
	Rotate(context.Context, string) (auth.Result, error)
	ResendVerification(context.Context, string, string) error
	ConfirmVerification(context.Context, string, string) error
	ForgotPassword(context.Context, string, string) error
	ResetPassword(context.Context, string, string, string) error
	ChangePassword(context.Context, string, string, string, string) (auth.Result, error)
}

type accountHandler struct {
	service AccountService
	logger  *slog.Logger
	cfg     config.AuthConfig
	secure  bool
}

// NewAccountHandler serves the staged /auth API, without legacy wildcard CORS.
func NewAccountHandler(logger *slog.Logger, cfg config.AuthConfig, env string, service AccountService) http.Handler {
	h := &accountHandler{service: service, logger: logger, cfg: cfg, secure: env != "development"}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/signup", h.signup)
	mux.HandleFunc("POST /auth/login", h.login)
	mux.HandleFunc("GET /auth/me", h.me)
	mux.HandleFunc("POST /auth/logout", h.logout)
	mux.HandleFunc("POST /auth/logout-all", h.logoutAll)
	mux.HandleFunc("POST /auth/session/rotate", h.rotate)
	mux.HandleFunc("POST /auth/verification/resend", h.resendVerification)
	mux.HandleFunc("POST /auth/verification/confirm", h.confirmVerification)
	mux.HandleFunc("POST /auth/password/forgot", h.forgotPassword)
	mux.HandleFunc("POST /auth/password/reset", h.resetPassword)
	mux.HandleFunc("POST /auth/password/change", h.changePassword)
	return h.protect(mux)
}

func (h *accountHandler) cookieName() string {
	if h.secure {
		return "__Host-keepup_session"
	}
	return "keepup_session"
}

func (h *accountHandler) native(r *http.Request) bool {
	return r.Header.Get("X-KeepUp-Client") == "native" && r.Header.Get("Origin") == "" && r.Header.Get("Sec-Fetch-Site") == "" && r.Header.Get("Cookie") == ""
}

func (h *accountHandler) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		origin := r.Header.Get("Origin")
		allowed := false
		for _, value := range h.cfg.Origins {
			allowed = allowed || value == origin
		}
		// Same-origin browser API only. No CORS/preflight grant for credential exchange.
		if origin != "" && !allowed {
			h.fail(w, http.StatusForbidden, "origin_rejected")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if !h.native(r) && (!allowed || r.Header.Get("X-KeepUp-Request") != "1") {
				h.fail(w, http.StatusForbidden, "csrf_rejected")
				return
			}
			contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || contentType != "application/json" {
				h.fail(w, http.StatusUnsupportedMediaType, "json_required")
				return
			}
		}
		// Cookie and bearer authorities must never be mixed or selected by precedence.
		if r.Header.Get("Authorization") != "" && !h.native(r) {
			h.fail(w, http.StatusForbidden, "invalid_transport")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func accountDecode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return auth.ErrInvalidInput
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return auth.ErrInvalidInput
	}
	return nil
}

func (h *accountHandler) sourceAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	peer = peer.Unmap()
	current := peer
	trusted := func(address netip.Addr) bool {
		for _, prefix := range h.cfg.TrustedProxies {
			if prefix.Contains(address) {
				return true
			}
		}
		return false
	}
	forwarded := r.Header.Get("X-Forwarded-For")
	if len(forwarded) > 4096 {
		return peer.String()
	}
	chain := strings.Split(forwarded, ",")
	for i := len(chain) - 1; i >= 0 && trusted(current); i-- {
		previous, parseErr := netip.ParseAddr(strings.TrimSpace(chain[i]))
		if parseErr != nil {
			return peer.String()
		}
		current = previous.Unmap()
	}
	return current.String()
}

func (h *accountHandler) signup(w http.ResponseWriter, r *http.Request) {
	var input auth.SignupInput
	if err := accountDecode(w, r, &input); err != nil {
		h.writeError(w, err)
		return
	}
	result, err := h.service.Signup(r.Context(), input, h.sourceAddress(r))
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.result(w, r, http.StatusCreated, result)
}

func (h *accountHandler) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if err := accountDecode(w, r, &input); err != nil {
		h.writeError(w, err)
		return
	}
	result, err := h.service.Login(r.Context(), input.Identifier, input.Password, h.sourceAddress(r))
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.result(w, r, http.StatusOK, result)
}

func (h *accountHandler) token(r *http.Request) string {
	if h.native(r) {
		return bearerToken(r.Header.Get("Authorization"))
	}
	cookie, err := r.Cookie(h.cookieName())
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *accountHandler) me(w http.ResponseWriter, r *http.Request) {
	token := h.token(r)
	session, err := h.service.Authenticate(r.Context(), token)
	if err != nil {
		h.writeError(w, err)
		return
	}
	if !h.native(r) {
		h.setCookie(w, token, session.ExpiresAt)
	}
	h.json(w, http.StatusOK, session)
}

func (h *accountHandler) logout(w http.ResponseWriter, r *http.Request) { h.revoke(w, r, false) }
func (h *accountHandler) rotate(w http.ResponseWriter, r *http.Request) {
	if err := accountDecode(w, r, &struct{}{}); err != nil {
		h.writeError(w, err)
		return
	}
	result, err := h.service.Rotate(r.Context(), h.token(r))
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.result(w, r, http.StatusOK, result)
}
func (h *accountHandler) logoutAll(w http.ResponseWriter, r *http.Request) { h.revoke(w, r, true) }
func (h *accountHandler) revoke(w http.ResponseWriter, r *http.Request, all bool) {
	if err := accountDecode(w, r, &struct{}{}); err != nil {
		h.writeError(w, err)
		return
	}
	if err := h.service.Logout(r.Context(), h.token(r), all); err != nil {
		h.writeError(w, err)
		return
	}
	if !h.native(r) {
		http.SetCookie(w, &http.Cookie{Name: h.cookieName(), Path: "/", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *accountHandler) result(w http.ResponseWriter, r *http.Request, status int, result auth.Result) {
	response := struct {
		User         auth.User `json:"user"`
		ExpiresAt    time.Time `json:"expiresAt"`
		SessionToken string    `json:"sessionToken,omitempty"`
	}{User: result.Session.User, ExpiresAt: result.Session.ExpiresAt}
	if h.native(r) {
		response.SessionToken = result.Token
	} else {
		h.setCookie(w, result.Token, result.Session.ExpiresAt)
	}
	h.json(w, status, response)
}

func (h *accountHandler) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: h.cookieName(), Value: token, Path: "/", Expires: expires, HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode})
}

func (h *accountHandler) json(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		h.logger.Error("encode account response")
	}
}

func (h *accountHandler) fail(w http.ResponseWriter, status int, reason string) {
	h.json(w, status, map[string]string{"error": reason})
}

func (h *accountHandler) writeError(w http.ResponseWriter, err error) {
	for _, entry := range []struct {
		err    error
		status int
	}{
		{auth.ErrInvalidInput, 400}, {auth.ErrEmailTaken, 409}, {auth.ErrUsernameTaken, 409},
		{auth.ErrCredentials, 401}, {auth.ErrSession, 401}, {auth.ErrPending, 403}, {auth.ErrRateLimited, 429},
		{auth.ErrInvalidToken, 400}, {auth.ErrTokenExpired, 400},
	} {
		if errors.Is(err, entry.err) {
			if entry.status == 429 {
				delay := h.cfg.Policy.AttemptWindow
				var rate *auth.RateLimitError
				if errors.As(err, &rate) {
					delay = rate.RetryAfter
				}
				w.Header().Set("Retry-After", strconv.FormatInt(int64(math.Max(1, math.Ceil(delay.Seconds()))), 10))
			}
			h.fail(w, entry.status, entry.err.Error())
			return
		}
	}
	// Driver errors may contain SQL/identifiers. Never send or log them here.
	h.logger.Error("account operation failed")
	h.fail(w, 500, "internal_error")
}
