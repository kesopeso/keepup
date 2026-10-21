package httpapi

import (
	"context"
	"net/http"
	"time"

	"keepup/apps/api/internal/auth"
)

// GoogleAccountService extends account authority without requiring Google in transport fakes.
type GoogleAccountService interface {
	StartGoogle(context.Context, string, string) (auth.GoogleStart, error)
	ProveGoogle(context.Context, string, string, string, string) (auth.GoogleResult, error)
	CompleteGoogle(context.Context, string, string, string, string, string) (auth.GoogleResult, error)
	GoogleContext(context.Context, string, string) (auth.GoogleContext, error)
	ExchangeGoogle(context.Context, string, string, string) (auth.GoogleResult, error)
	ConfirmGoogleEmail(context.Context, string, string) error
	ResendGoogleEmail(context.Context, string, string, string) error
}

func (h *accountHandler) googleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/google/start", h.googleStart)
	mux.HandleFunc("POST /auth/google/proof", h.googleProof)
	mux.HandleFunc("POST /auth/google/complete", h.googleComplete)
	mux.HandleFunc("GET /auth/google/context", h.googleContext)
	mux.HandleFunc("POST /auth/google/exchange", h.googleExchange)
	mux.HandleFunc("POST /auth/google/email/confirm", h.googleEmailConfirm)
	mux.HandleFunc("POST /auth/google/email/resend", h.googleEmailResend)
}

func (h *accountHandler) googleService(w http.ResponseWriter) (GoogleAccountService, bool) {
	service, ok := h.service.(GoogleAccountService)
	if !ok {
		h.writeError(w, auth.ErrGoogleDisabled)
	}
	return service, ok
}

func (h *accountHandler) flowCookieName() string {
	if h.secure {
		return "__Host-keepup_google"
	}
	return "keepup_google"
}

func (h *accountHandler) flowTransport(r *http.Request) string {
	if h.native(r) {
		return "native"
	}
	return "web"
}

func (h *accountHandler) flowToken(r *http.Request) string {
	if h.native(r) {
		return bearerToken(r.Header.Get("Authorization"))
	}
	cookie, err := r.Cookie(h.flowCookieName())
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *accountHandler) googleStart(w http.ResponseWriter, r *http.Request) {
	if err := accountDecode(w, r, &struct{}{}); err != nil {
		h.writeError(w, err)
		return
	}
	service, ok := h.googleService(w)
	if !ok {
		return
	}
	result, err := service.StartGoogle(r.Context(), h.flowTransport(r), h.sourceAddress(r))
	if err != nil {
		h.writeError(w, err)
		return
	}
	response := struct {
		Nonce     string    `json:"nonce"`
		ExpiresAt time.Time `json:"expiresAt"`
		FlowToken string    `json:"flowToken,omitempty"`
	}{Nonce: result.Nonce, ExpiresAt: result.ExpiresAt}
	if h.native(r) {
		response.FlowToken = result.Token
	} else {
		http.SetCookie(w, &http.Cookie{Name: h.flowCookieName(), Value: result.Token, Path: "/", Expires: result.ExpiresAt, HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode})
	}
	h.json(w, http.StatusOK, response)
}

func (h *accountHandler) googleProof(w http.ResponseWriter, r *http.Request) {
	var input struct {
		IDToken string `json:"idToken"`
	}
	if err := accountDecodeLimit(w, r, &input, 16384); err != nil {
		h.writeError(w, err)
		return
	}
	service, ok := h.googleService(w)
	if !ok {
		return
	}
	result, err := service.ProveGoogle(r.Context(), h.flowToken(r), h.flowTransport(r), input.IDToken, h.sourceAddress(r))
	h.googleResult(w, r, result, err)
}

func (h *accountHandler) googleComplete(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := accountDecode(w, r, &input); err != nil {
		h.writeError(w, err)
		return
	}
	service, ok := h.googleService(w)
	if !ok {
		return
	}
	result, err := service.CompleteGoogle(r.Context(), h.flowToken(r), h.flowTransport(r), input.Username, input.Password, h.sourceAddress(r))
	h.googleResult(w, r, result, err)
}

func (h *accountHandler) googleExchange(w http.ResponseWriter, r *http.Request) {
	if err := accountDecode(w, r, &struct{}{}); err != nil {
		h.writeError(w, err)
		return
	}
	service, ok := h.googleService(w)
	if !ok {
		return
	}
	result, err := service.ExchangeGoogle(r.Context(), h.flowToken(r), h.flowTransport(r), h.sourceAddress(r))
	h.googleResult(w, r, result, err)
}

func (h *accountHandler) googleResult(w http.ResponseWriter, r *http.Request, result auth.GoogleResult, err error) {
	if err != nil {
		h.writeError(w, err)
		return
	}
	if result.Account == nil {
		h.json(w, http.StatusOK, result.Context)
		return
	}
	account := result.Account
	response := struct {
		Status       string    `json:"status"`
		User         auth.User `json:"user"`
		ExpiresAt    time.Time `json:"expiresAt"`
		SessionToken string    `json:"sessionToken,omitempty"`
	}{Status: "authenticated", User: account.Session.User, ExpiresAt: account.Session.ExpiresAt}
	if h.native(r) {
		response.SessionToken = account.Token
	} else {
		h.setCookie(w, account.Token, account.Session.ExpiresAt)
		http.SetCookie(w, &http.Cookie{Name: h.flowCookieName(), Path: "/", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	}
	h.json(w, http.StatusOK, response)
}

func (h *accountHandler) googleContext(w http.ResponseWriter, r *http.Request) {
	service, ok := h.googleService(w)
	if !ok {
		return
	}
	result, err := service.GoogleContext(r.Context(), h.flowToken(r), h.flowTransport(r))
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.json(w, http.StatusOK, result)
}

func (h *accountHandler) googleEmailConfirm(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
	if err := accountDecode(w, r, &input); err != nil {
		h.writeError(w, err)
		return
	}
	service, ok := h.googleService(w)
	if !ok {
		return
	}
	if err := service.ConfirmGoogleEmail(r.Context(), input.Token, h.sourceAddress(r)); err != nil {
		h.writeError(w, err)
		return
	}
	h.json(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (h *accountHandler) googleEmailResend(w http.ResponseWriter, r *http.Request) {
	if err := accountDecode(w, r, &struct{}{}); err != nil {
		h.writeError(w, err)
		return
	}
	service, ok := h.googleService(w)
	if !ok {
		return
	}
	if err := service.ResendGoogleEmail(r.Context(), h.flowToken(r), h.flowTransport(r), h.sourceAddress(r)); err != nil {
		h.writeError(w, err)
		return
	}
	h.accepted(w)
}
