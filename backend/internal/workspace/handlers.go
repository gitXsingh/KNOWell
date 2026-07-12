package workspace

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gitXsingh/knowell/backend/internal/auth"
	"github.com/go-chi/chi/v5"
)

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaces, err := s.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong")
		return
	}

	writeJSON(w, http.StatusOK, workspaces)
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req WorkspaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	workspace, err := s.Create(r.Context(), userID, req)
	if err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, workspace)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	workspace, err := s.Get(r.Context(), userID, workspaceID)
	if err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, workspace)
}

func (s *Service) handleUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req WorkspaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	workspace, err := s.Update(r.Context(), userID, workspaceID, req)
	if err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, workspace)
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	if err := s.Delete(r.Context(), userID, workspaceID); err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Service) handleJoinByKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req JoinByKeyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	workspace, err := s.JoinByKey(r.Context(), userID, req.Key)
	if err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, workspace)
}

func (s *Service) handleGetJoinKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	resp, err := s.GetJoinKey(r.Context(), userID, workspaceID)
	if err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) handleRegenerateJoinKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	resp, err := s.RegenerateJoinKey(r.Context(), userID, workspaceID)
	if err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) handleListJoinRequests(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	requests, err := s.ListJoinRequests(r.Context(), userID, workspaceID)
	if err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, requests)
}

func (s *Service) handleApproveRejectJoinRequest(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req ApproveRejectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")

	var result *JoinRequest
	var err error
	switch req.Action {
	case "approve":
		result, err = s.ApproveJoinRequest(r.Context(), userID, workspaceID, req.UserID)
	case "reject":
		result, err = s.RejectJoinRequest(r.Context(), userID, workspaceID, req.UserID)
	default:
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "action must be 'approve' or 'reject'")
		return
	}

	if err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleMembers(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	members, err := s.Members(r.Context(), userID, workspaceID)
	if err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, members)
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func handleWorkspaceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrWorkspaceExists):
		writeError(w, http.StatusConflict, "workspace_exists", "A workspace with that name or slug already exists")
	case errors.Is(err, ErrWorkspaceDenied):
		writeError(w, http.StatusForbidden, "forbidden", "You do not have access to this workspace")
	case errors.Is(err, ErrWorkspaceMissing):
		writeError(w, http.StatusNotFound, "workspace_not_found", "Workspace not found")
	case errors.Is(err, ErrWorkspaceJoinKeyInvalid):
		writeError(w, http.StatusNotFound, "invalid_join_key", "Invalid workspace join key")
	case errors.Is(err, ErrWorkspaceAlreadyMember):
		writeError(w, http.StatusConflict, "already_member", "You are already a member of this workspace")
	case errors.Is(err, ErrJoinRequestExists):
		writeError(w, http.StatusConflict, "join_request_exists", "A pending join request already exists")
	case errors.Is(err, ErrJoinRequestMissing):
		writeError(w, http.StatusNotFound, "join_request_not_found", "Join request not found")
	case errors.Is(err, ErrJoinRequestAlreadyReviewed):
		writeError(w, http.StatusConflict, "join_request_reviewed", "Join request has already been reviewed")
	case errors.Is(err, ErrWorkspaceInvitationExists), errors.Is(err, ErrWorkspaceMemberExists):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, ErrWorkspaceInvitationMissing), errors.Is(err, ErrWorkspaceMemberMissing):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, ErrWorkspaceInvitationInvalid):
		writeError(w, http.StatusUnprocessableEntity, "invalid_invitation", err.Error())
	case errors.Is(err, ErrWorkspaceInvitationDenied), errors.Is(err, ErrWorkspaceMemberDenied):
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
	case strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid"):
		writeError(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong")
	}
}
