package github

import (
	"errors"
	"net/http"
)

func (s *Service) handleConnect(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	connectURL, err := s.ConnectURL(userID, r.URL.Query().Get("redirect_to"))
	if err != nil {
		handleGitHubError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, ConnectResponse{AuthorizationURL: connectURL})
}

func (s *Service) handleCallback(w http.ResponseWriter, r *http.Request) {
	if oauthError := r.URL.Query().Get("error"); oauthError != "" {
		target := s.callbackRedirectURL(s.cfg.GitHubFrontendCallbackURL, "error", "GitHub authorization was denied")
		http.Redirect(w, r, target, http.StatusFound)
		return
	}

	redirectURL, err := s.HandleCallback(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"))
	if err != nil && redirectURL == "" {
		handleGitHubError(w, err)
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (s *Service) handleAccountStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	status, err := s.AccountStatus(r.Context(), userID)
	if err != nil {
		handleGitHubError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func (s *Service) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if err := s.Disconnect(r.Context(), userID); err != nil {
		handleGitHubError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}

func handleGitHubError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrGitHubNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "github_not_configured", "GitHub OAuth is not configured")
	case errors.Is(err, ErrGitHubStateInvalid):
		writeError(w, http.StatusBadRequest, "invalid_state", "GitHub OAuth state is invalid or expired")
	case errors.Is(err, ErrGitHubAccountLinked):
		writeError(w, http.StatusConflict, "github_account_linked", "That GitHub account is already linked")
	case errors.Is(err, ErrGitHubAccountMissing):
		writeError(w, http.StatusNotFound, "github_account_missing", "No GitHub account is connected")
	default:
		writeError(w, http.StatusInternalServerError, "github_error", "GitHub connection failed")
	}
}
