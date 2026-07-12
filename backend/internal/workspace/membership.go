package workspace

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/auth"
	"github.com/go-chi/chi/v5"
)

type WorkspaceInvitation struct {
	ID           string    `json:"id"`
	ScopeType    string    `json:"scope_type"`
	ScopeID      string    `json:"scope_id"`
	InvitedEmail string    `json:"invited_email"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type WorkspaceInvitationResponse struct {
	WorkspaceInvitation
	Token string `json:"token"`
}

type WorkspaceInvitationRequest struct {
	Email string `json:"email"`
}

type AcceptInvitationRequest struct {
	Token string `json:"token"`
}

var (
	ErrWorkspaceInvitationExists  = errors.New("workspace invitation already exists")
	ErrWorkspaceInvitationMissing = errors.New("workspace invitation not found")
	ErrWorkspaceInvitationInvalid = errors.New("invalid workspace invitation")
	ErrWorkspaceInvitationDenied  = errors.New("workspace invitation denied")
	ErrWorkspaceMemberExists      = errors.New("workspace member already exists")
	ErrWorkspaceMemberMissing     = errors.New("workspace member not found")
	ErrWorkspaceMemberDenied      = errors.New("workspace owner cannot be removed")
)

const workspaceInvitationTTL = 7 * 24 * time.Hour

func (s *Service) ListInvitations(ctx context.Context, userID, workspaceID string) ([]WorkspaceInvitation, error) {
	isOwner, err := s.isOwner(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, ErrWorkspaceDenied
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, scope_type, scope_id, invited_email, role, status, expires_at, created_at
		FROM invitations
		WHERE scope_type = 'workspace' AND scope_id = $1
		ORDER BY created_at DESC
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invitations := make([]WorkspaceInvitation, 0)
	for rows.Next() {
		var invitation WorkspaceInvitation
		if err := rows.Scan(&invitation.ID, &invitation.ScopeType, &invitation.ScopeID, &invitation.InvitedEmail, &invitation.Role, &invitation.Status, &invitation.ExpiresAt, &invitation.CreatedAt); err != nil {
			return nil, err
		}
		invitations = append(invitations, invitation)
	}

	return invitations, rows.Err()
}

func (s *Service) CreateInvitation(ctx context.Context, userID, workspaceID string, req WorkspaceInvitationRequest) (*WorkspaceInvitationResponse, error) {
	isOwner, err := s.isOwner(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, ErrWorkspaceDenied
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	var existingMember bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM workspace_members wm
			JOIN users u ON u.id = wm.user_id
			WHERE wm.workspace_id = $1 AND lower(u.email) = $2
		)
	`, workspaceID, email).Scan(&existingMember); err != nil {
		return nil, err
	}
	if existingMember {
		return nil, ErrWorkspaceMemberExists
	}

	pendingExists, err := s.hasPendingWorkspaceInvitation(ctx, workspaceID, email)
	if err != nil {
		return nil, err
	}
	if pendingExists {
		return nil, ErrWorkspaceInvitationExists
	}

	for attempt := 0; attempt < 3; attempt++ {
		token, tokenHash, err := generateInvitationToken()
		if err != nil {
			return nil, err
		}

		invitation, err := s.insertWorkspaceInvitation(ctx, userID, workspaceID, email, token, tokenHash)
		if err == nil {
			return invitation, nil
		}
		if isUniqueViolation(err) {
			continue
		}
		return nil, err
	}

	return nil, fmt.Errorf("failed to create workspace invitation")
}

func (s *Service) AcceptInvitation(ctx context.Context, userID, workspaceID string, req AcceptInvitationRequest) (*WorkspaceInvitation, error) {
	tokenHash := hashInvitationToken(req.Token)
	if tokenHash == "" {
		return nil, ErrWorkspaceInvitationInvalid
	}

	var invitation WorkspaceInvitation
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, scope_type, scope_id, invited_email, role, status, expires_at, created_at
		FROM invitations
		WHERE token_hash = $1 AND scope_type = 'workspace' AND scope_id = $2
	`, tokenHash, workspaceID).Scan(&invitation.ID, &invitation.ScopeType, &invitation.ScopeID, &invitation.InvitedEmail, &invitation.Role, &invitation.Status, &invitation.ExpiresAt, &invitation.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkspaceInvitationMissing
		}
		return nil, err
	}

	if invitation.Status != "pending" || time.Now().After(invitation.ExpiresAt) {
		return nil, ErrWorkspaceInvitationInvalid
	}

	var userEmail string
	if err := s.db.QueryRowContext(ctx, `SELECT lower(email) FROM users WHERE id = $1`, userID).Scan(&userEmail); err != nil {
		return nil, err
	}
	if userEmail != strings.ToLower(strings.TrimSpace(invitation.InvitedEmail)) {
		return nil, ErrWorkspaceInvitationDenied
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var memberExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2
		)
	`, workspaceID, userID).Scan(&memberExists); err != nil {
		return nil, err
	}
	if memberExists {
		return nil, ErrWorkspaceMemberExists
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, 'member')
	`, workspaceID, userID); err != nil {
		return nil, err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE invitations
		SET status = 'accepted', accepted_by_user_id = $1
		WHERE id = $2 AND status = 'pending'
	`, userID, invitation.ID)
	if err != nil {
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return nil, ErrWorkspaceInvitationInvalid
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	invitation.Status = "accepted"
	return &invitation, nil
}

func (s *Service) RemoveMember(ctx context.Context, userID, workspaceID, memberUserID string) error {
	isOwner, err := s.isOwner(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !isOwner {
		return ErrWorkspaceDenied
	}

	var ownerID string
	if err := s.db.QueryRowContext(ctx, `SELECT owner_user_id FROM workspaces WHERE id = $1`, workspaceID).Scan(&ownerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrWorkspaceMissing
		}
		return err
	}
	if ownerID == memberUserID {
		return ErrWorkspaceMemberDenied
	}

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM workspace_members
		WHERE workspace_id = $1 AND user_id = $2
	`, workspaceID, memberUserID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrWorkspaceMemberMissing
	}

	return nil
}

func (s *Service) hasPendingWorkspaceInvitation(ctx context.Context, workspaceID, email string) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM invitations
			WHERE scope_type = 'workspace'
			  AND scope_id = $1
			  AND lower(invited_email) = $2
			  AND status = 'pending'
			  AND expires_at > now()
		)
	`, workspaceID, email).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Service) insertWorkspaceInvitation(ctx context.Context, invitedByUserID, workspaceID, email, token, tokenHash string) (*WorkspaceInvitationResponse, error) {
	expiresAt := time.Now().Add(workspaceInvitationTTL)
	var invitation WorkspaceInvitation
	if err := s.db.QueryRowContext(ctx, `
		INSERT INTO invitations (
			scope_type,
			scope_id,
			invited_email,
			role,
			token_hash,
			expires_at,
			invited_by_user_id
		)
		VALUES ('workspace', $1, $2, 'member', $3, $4, $5)
		RETURNING id, scope_type, scope_id, invited_email, role, status, expires_at, created_at
	`, workspaceID, email, tokenHash, expiresAt, invitedByUserID).Scan(&invitation.ID, &invitation.ScopeType, &invitation.ScopeID, &invitation.InvitedEmail, &invitation.Role, &invitation.Status, &invitation.ExpiresAt, &invitation.CreatedAt); err != nil {
		return nil, err
	}

	return &WorkspaceInvitationResponse{WorkspaceInvitation: invitation, Token: token}, nil
}

func generateInvitationToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(bytes)
	return token, hashInvitationToken(token), nil
}

func hashInvitationToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Service) handleListInvitations(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	invitations, err := s.ListInvitations(r.Context(), userID, workspaceID)
	if err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, invitations)
}

func (s *Service) handleCreateInvitation(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req WorkspaceInvitationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	invitation, err := s.CreateInvitation(r.Context(), userID, workspaceID, req)
	if err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, invitation)
}

func (s *Service) handleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req AcceptInvitationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	invitation, err := s.AcceptInvitation(r.Context(), userID, workspaceID, req)
	if err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, invitation)
}

func (s *Service) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	memberUserID := chi.URLParam(r, "memberUserID")
	if err := s.RemoveMember(r.Context(), userID, workspaceID, memberUserID); err != nil {
		handleWorkspaceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
