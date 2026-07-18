package handlers

import (
	"encoding/json"
	"net/http"

	"codeberg.org/Sylos/Spectra/sdk"
)

// AuthHandler serves token issue/refresh endpoints.
type AuthHandler struct {
	BaseHandler
	fs *sdk.SpectraFS
}

// NewAuthHandler creates an auth handler.
func NewAuthHandler(fs *sdk.SpectraFS) *AuthHandler {
	return &AuthHandler{fs: fs}
}

type issueTokenRequest struct {
	World string `json:"world"`
}

type refreshTokenRequest struct {
	World        string `json:"world"`
	RefreshToken string `json:"refresh_token"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	World        string `json:"world"`
}

// IssueToken handles POST /api/v1/auth/token
func (h *AuthHandler) IssueToken(w http.ResponseWriter, req *http.Request) {
	if !h.fs.AuthEnabled() {
		h.sendError(w, http.StatusBadRequest, "auth is not enabled")
		return
	}
	var body issueTokenRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.World == "" {
		body.World = "primary"
	}
	pair, err := h.fs.IssueTokens(body.World)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.sendJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    sdk.FormatExpiresAt(pair.ExpiresAt),
		World:        pair.World,
	})
}

// RefreshToken handles POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, req *http.Request) {
	if !h.fs.AuthEnabled() {
		h.sendError(w, http.StatusBadRequest, "auth is not enabled")
		return
	}
	var body refreshTokenRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.World == "" {
		body.World = "primary"
	}
	if body.RefreshToken == "" {
		h.sendError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}
	pair, err := h.fs.RefreshAccessToken(body.World, body.RefreshToken)
	if err != nil {
		if _, ok := sdk.IsUnauthorized(err); ok {
			h.sendError(w, http.StatusUnauthorized, err.Error())
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.sendJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    sdk.FormatExpiresAt(pair.ExpiresAt),
		World:        pair.World,
	})
}
