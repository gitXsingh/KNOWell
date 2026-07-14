package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gitXsingh/knowell/backend/internal/auth"
	gh "github.com/gitXsingh/knowell/backend/internal/github"
	wh "github.com/gitXsingh/knowell/backend/internal/webhook"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	projects, err := s.List(r.Context(), userID, workspaceID)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, projects)
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req ProjectRequest
	if err := decodeProjectJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	project, err := s.Create(r.Context(), userID, workspaceID, req)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	project, err := s.Get(r.Context(), userID, projectID)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (s *Service) handleUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req ProjectRequest
	if err := decodeProjectJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	project, err := s.Update(r.Context(), userID, projectID, req)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	if err := s.Delete(r.Context(), userID, projectID); err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Service) handleSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	project, err := s.Settings(r.Context(), userID, projectID)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (s *Service) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req SourceSettingsRequest
	if err := decodeProjectJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	project, err := s.UpdateSettings(r.Context(), userID, projectID, req)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (s *Service) handleListRepositoryOptions(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	options, err := s.ListRepositoryOptions(r.Context(), userID, projectID, r.URL.Query().Get("query"))
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, options)
}

func (s *Service) handleGetRepository(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	repository, err := s.GetRepository(r.Context(), userID, projectID)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, repository)
}

func (s *Service) handleConnectRepository(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req RepositoryConnectionRequest
	if err := decodeProjectJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	repository, err := s.ConnectRepository(r.Context(), userID, projectID, req)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, repository)
}

func (s *Service) handleDeleteRepository(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	if err := s.DisconnectRepository(r.Context(), userID, projectID); err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Service) handleSyncRepositoryWebhook(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	repository, err := s.SyncRepositoryWebhook(r.Context(), userID, projectID)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, repository)
}

func decodeProjectJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func handleProjectError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrProjectExists):
		writeError(w, http.StatusConflict, "project_exists", "A project with that name or slug already exists")
	case errors.Is(err, ErrProjectDenied):
		writeError(w, http.StatusForbidden, "forbidden", "You do not have access to this project")
	case errors.Is(err, ErrProjectMissing):
		writeError(w, http.StatusNotFound, "project_not_found", "Project not found")
	case errors.Is(err, ErrProjectState) || errors.Is(err, ErrProjectSource):
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "Invalid project configuration")
	case errors.Is(err, ErrRepositoryDisabled):
		writeError(w, http.StatusUnprocessableEntity, "repository_disabled", "The github_repository source is not enabled for this project")
	case errors.Is(err, ErrRepositoryMissing):
		writeError(w, http.StatusNotFound, "repository_not_found", "No repository is connected to this project")
	case errors.Is(err, ErrProjectInvitationExists), errors.Is(err, ErrProjectMemberExists):
		writeError(w, http.StatusConflict, "conflict", "This invitation or membership already exists")
	case errors.Is(err, ErrProjectInvitationMissing), errors.Is(err, ErrProjectMemberMissing):
		writeError(w, http.StatusNotFound, "not_found", "Invitation or membership not found")
	case errors.Is(err, ErrProjectInvitationInvalid):
		writeError(w, http.StatusUnprocessableEntity, "invalid_invitation", "This invitation is invalid or expired")
	case errors.Is(err, ErrProjectInvitationDenied), errors.Is(err, ErrProjectMemberDenied):
		writeError(w, http.StatusForbidden, "forbidden", "You do not have permission to manage invitations or members")
	case errors.Is(err, ErrProjectRoleInvalid):
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "The selected role is invalid")
	case errors.Is(err, gh.ErrGitHubAccountMissing):
		writeError(w, http.StatusNotFound, "github_account_missing", "Connect a GitHub account before linking a repository")
	case errors.Is(err, gh.ErrGitHubRepositoryMissing):
		writeError(w, http.StatusNotFound, "github_repository_missing", "The GitHub repository was not found or is not accessible")
	case errors.Is(err, gh.ErrGitHubWebhookConfigMissing), errors.Is(err, gh.ErrGitHubNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "github_not_configured", "GitHub integration is not configured")
	case errors.Is(err, wh.ErrEventDenied):
		writeError(w, http.StatusForbidden, "forbidden", "You do not have access to these webhook events")
	case errors.Is(err, wh.ErrEventMissing):
		writeError(w, http.StatusNotFound, "webhook_event_not_found", "Webhook event not found")
	case errors.Is(err, ErrProjectInvalid):
		writeError(w, http.StatusBadRequest, "invalid_request", "Owner and repository name are required")
	case isPGUniqueViolation(err):
		writeError(w, http.StatusConflict, "repository_already_linked", "This repository is already linked to another project")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

func isPGUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Service) handleListWebhookEvents(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	if s.webhook == nil {
		writeError(w, http.StatusServiceUnavailable, "webhook_unavailable", "Webhook service is not available")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	events, err := s.webhook.ListProjectEvents(r.Context(), userID, projectID)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, events)
}

func (s *Service) handleProcessWebhookEvents(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	if s.webhook == nil {
		writeError(w, http.StatusServiceUnavailable, "webhook_unavailable", "Webhook service is not available")
		return
	}
	if !s.canEditProject(r.Context(), userID, chi.URLParam(r, "projectID")) {
		writeError(w, http.StatusForbidden, "forbidden", "You do not have access to this project")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	if err := s.webhook.ProcessPendingProjectEvents(r.Context(), projectID); err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":{"code":%q,"message":%q}}`, code, message)
}
