package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func (s *Service) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	session, err := s.Register(r.Context(), req)
	if err != nil {
		s.handleError(w, err)
		return
	}

	s.setSessionCookie(w, session.Token)
	writeJSON(w, http.StatusCreated, session)
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	session, err := s.Login(r.Context(), req)
	if err != nil {
		s.handleError(w, err)
		return
	}

	s.setSessionCookie(w, session.Token)
	writeJSON(w, http.StatusOK, session)
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	session, err := s.Me(r.Context(), userID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, session)
}

func (s *Service) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect")
	case errors.Is(err, ErrEmailExists):
		writeError(w, http.StatusConflict, "email_exists", "An account with that email already exists")
	case strings.Contains(err.Error(), "missing required fields"):
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "Please fill in all required fields")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong")
	}
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	if decoder.More() {
		return errors.New("unexpected trailing JSON data")
	}

	return nil
}
