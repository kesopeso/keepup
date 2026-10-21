package httpapi

import "net/http"

func (h *accountHandler) accepted(w http.ResponseWriter) {
	h.json(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (h *accountHandler) resendVerification(w http.ResponseWriter, r *http.Request) {
	if err := accountDecode(w, r, &struct{}{}); err != nil {
		h.writeError(w, err)
		return
	}
	token := h.token(r)
	if err := h.service.ResendVerification(r.Context(), token, h.sourceAddress(r)); err != nil {
		h.writeError(w, err)
		return
	}
	session, err := h.service.Authenticate(r.Context(), token)
	if err != nil {
		h.writeError(w, err)
		return
	}
	if !h.native(r) {
		h.setCookie(w, token, session.ExpiresAt)
	}
	h.accepted(w)
}

func (h *accountHandler) confirmVerification(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
	if err := accountDecode(w, r, &input); err != nil {
		h.writeError(w, err)
		return
	}
	if err := h.service.ConfirmVerification(r.Context(), input.Token, h.sourceAddress(r)); err != nil {
		h.writeError(w, err)
		return
	}
	h.json(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (h *accountHandler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Identifier string `json:"identifier"`
	}
	if err := accountDecode(w, r, &input); err != nil {
		h.writeError(w, err)
		return
	}
	if err := h.service.ForgotPassword(r.Context(), input.Identifier, h.sourceAddress(r)); err != nil {
		h.writeError(w, err)
		return
	}
	h.accepted(w)
}

func (h *accountHandler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := accountDecode(w, r, &input); err != nil {
		h.writeError(w, err)
		return
	}
	if err := h.service.ResetPassword(r.Context(), input.Token, input.Password, h.sourceAddress(r)); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *accountHandler) changePassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CurrentPassword string `json:"currentPassword"`
		Password        string `json:"password"`
	}
	if err := accountDecode(w, r, &input); err != nil {
		h.writeError(w, err)
		return
	}
	result, err := h.service.ChangePassword(r.Context(), h.token(r), input.CurrentPassword, input.Password, h.sourceAddress(r))
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.result(w, r, http.StatusOK, result)
}
