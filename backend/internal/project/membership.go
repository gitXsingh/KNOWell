package project

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

type ProjectMember struct {
	UserID   string    `json:"user_id"`
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type ProjectInvitation struct {
	ID           string    `json:"id"`
	ScopeType    string    `json:"scope_type"`
	ScopeID      string    `json:"scope_id"`
	InvitedEmail string    `json:"invited_email"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type ProjectInvitationResponse struct {
	ProjectInvitation
	Token string `json:"token"`
}

type ProjectMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type ProjectMemberRoleRequest struct {
	Role string `json:"role"`
}

type AcceptProjectInvitationRequest struct {
	Token string `json:"token"`
}

var (
	ErrProjectInvitationExists  = errors.New("project invitation already exists")
	ErrProjectInvitationMissing = errors.New("project invitation not found")
	ErrProjectInvitationInvalid = errors.New("invalid project invitation")
	ErrProjectInvitationDenied  = errors.New("project invitation denied")
	ErrProjectMemberExists      = errors.New("project member already exists")
	ErrProjectMemberMissing     = errors.New("project member not found")
	ErrProjectRoleInvalid       = errors.New("invalid project member role")
	ErrProjectMemberDenied      = errors.New("project member management denied")
)

const projectInvitationTTL = 7 * 24 * time.Hour

var allowedProjectMemberRoles = map[string]struct{}{
	"owner":     {},
	"tech_lead": {},
	"developer": {},
	"viewer":    {},
}

func (s *Service) ListMembers(ctx context.Context, userID, workspaceID, projectID string) ([]ProjectMember, error) {
	if _, err := s.Get(ctx, userID, projectID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT pm.user_id, u.email, u.full_name, pm.role, pm.joined_at
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = $1
		ORDER BY pm.joined_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]ProjectMember, 0)
	for rows.Next() {
		var member ProjectMember
		if err := rows.Scan(&member.UserID, &member.Email, &member.FullName, &member.Role, &member.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	return members, rows.Err()
}

func (s *Service) CreateMember(ctx context.Context, userID, workspaceID, projectID string, req ProjectMemberRequest) (*ProjectMember, error) {
	if !s.canEditProject(ctx, userID, projectID) {
		return nil, ErrProjectDenied
	}

	role, err := normalizeProjectMemberRole(req.Role)
	if err != nil {
		return nil, err
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	var targetUser ProjectMember
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, email, full_name
		FROM users
		WHERE lower(email) = $1
	`, email).Scan(&targetUser.UserID, &targetUser.Email, &targetUser.FullName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectMemberMissing
		}
		return nil, err
	}

	if exists, err := s.projectMemberExists(ctx, projectID, targetUser.UserID); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrProjectMemberExists
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := ensureWorkspaceMemberTx(ctx, tx, workspaceID, targetUser.UserID); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_members (project_id, user_id, role)
		VALUES ($1, $2, $3)
	`, projectID, targetUser.UserID, role); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	targetUser.Role = role
	return &targetUser, nil
}

func (s *Service) ListMemberInvitations(ctx context.Context, userID, workspaceID, projectID string) ([]ProjectInvitation, error) {
	if !s.canEditProject(ctx, userID, projectID) {
		return nil, ErrProjectDenied
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, scope_type, scope_id, invited_email, role, status, expires_at, created_at
		FROM invitations
		WHERE scope_type = 'project' AND scope_id = $1
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invitations := make([]ProjectInvitation, 0)
	for rows.Next() {
		var invitation ProjectInvitation
		if err := rows.Scan(&invitation.ID, &invitation.ScopeType, &invitation.ScopeID, &invitation.InvitedEmail, &invitation.Role, &invitation.Status, &invitation.ExpiresAt, &invitation.CreatedAt); err != nil {
			return nil, err
		}
		invitations = append(invitations, invitation)
	}

	return invitations, rows.Err()
}

func (s *Service) CreateMemberInvitation(ctx context.Context, userID, workspaceID, projectID string, req ProjectMemberRequest) (*ProjectInvitationResponse, error) {
	if !s.canEditProject(ctx, userID, projectID) {
		return nil, ErrProjectDenied
	}

	role, err := normalizeProjectMemberRole(req.Role)
	if err != nil {
		return nil, err
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	if exists, err := s.projectMemberExistsByEmail(ctx, projectID, email); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrProjectMemberExists
	}

	pendingExists, err := s.hasPendingProjectInvitation(ctx, projectID, email)
	if err != nil {
		return nil, err
	}
	if pendingExists {
		return nil, ErrProjectInvitationExists
	}

	for attempt := 0; attempt < 3; attempt++ {
		token, tokenHash, err := generateProjectInvitationToken()
		if err != nil {
			return nil, err
		}

		invitation, err := s.insertProjectInvitation(ctx, userID, projectID, email, role, token, tokenHash)
		if err == nil {
			return invitation, nil
		}
		if isUniqueViolation(err) {
			continue
		}
		return nil, err
	}

	return nil, fmt.Errorf("failed to create project invitation")
}

func (s *Service) AcceptMemberInvitation(ctx context.Context, userID, workspaceID, projectID string, req AcceptProjectInvitationRequest) (*ProjectInvitation, error) {
	projectWorkspaceID, err := s.projectWorkspaceID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if projectWorkspaceID != workspaceID {
		return nil, ErrProjectInvitationMissing
	}

	tokenHash := hashProjectInvitationToken(req.Token)
	if tokenHash == "" {
		return nil, ErrProjectInvitationInvalid
	}

	var invitation ProjectInvitation
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, scope_type, scope_id, invited_email, role, status, expires_at, created_at
		FROM invitations
		WHERE token_hash = $1 AND scope_type = 'project' AND scope_id = $2
	`, tokenHash, projectID).Scan(&invitation.ID, &invitation.ScopeType, &invitation.ScopeID, &invitation.InvitedEmail, &invitation.Role, &invitation.Status, &invitation.ExpiresAt, &invitation.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectInvitationMissing
		}
		return nil, err
	}

	if invitation.Status != "pending" || time.Now().After(invitation.ExpiresAt) {
		return nil, ErrProjectInvitationInvalid
	}

	var userEmail string
	if err := s.db.QueryRowContext(ctx, `SELECT lower(email) FROM users WHERE id = $1`, userID).Scan(&userEmail); err != nil {
		return nil, err
	}
	if userEmail != strings.ToLower(strings.TrimSpace(invitation.InvitedEmail)) {
		return nil, ErrProjectInvitationDenied
	}

	role, err := normalizeProjectMemberRole(invitation.Role)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := ensureWorkspaceMemberTx(ctx, tx, workspaceID, userID); err != nil {
		return nil, err
	}

	var memberExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2
		)
	`, projectID, userID).Scan(&memberExists); err != nil {
		return nil, err
	}
	if memberExists {
		return nil, ErrProjectMemberExists
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_members (project_id, user_id, role)
		VALUES ($1, $2, $3)
	`, projectID, userID, role); err != nil {
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
		return nil, ErrProjectInvitationInvalid
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	invitation.Status = "accepted"
	invitation.Role = role
	return &invitation, nil
}

func (s *Service) UpdateMember(ctx context.Context, userID, workspaceID, projectID, memberUserID string, req ProjectMemberRoleRequest) (*ProjectMember, error) {
	if !s.canEditProject(ctx, userID, projectID) {
		return nil, ErrProjectDenied
	}

	role, err := normalizeProjectMemberRole(req.Role)
	if err != nil {
		return nil, err
	}

	var member ProjectMember
	if err := s.db.QueryRowContext(ctx, `
		SELECT pm.user_id, u.email, u.full_name, pm.role, pm.joined_at
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = $1 AND pm.user_id = $2
	`, projectID, memberUserID).Scan(&member.UserID, &member.Email, &member.FullName, &member.Role, &member.JoinedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectMemberMissing
		}
		return nil, err
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE project_members
		SET role = $1
		WHERE project_id = $2 AND user_id = $3
	`, role, projectID, memberUserID); err != nil {
		return nil, err
	}

	member.Role = role
	return &member, nil
}

func (s *Service) RemoveMember(ctx context.Context, userID, workspaceID, projectID, memberUserID string) error {
	if !s.canEditProject(ctx, userID, projectID) {
		return ErrProjectDenied
	}

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM project_members
		WHERE project_id = $1 AND user_id = $2
	`, projectID, memberUserID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrProjectMemberMissing
	}

	return nil
}

func (s *Service) projectWorkspaceID(ctx context.Context, projectID string) (string, error) {
	var workspaceID string
	if err := s.db.QueryRowContext(ctx, `SELECT workspace_id FROM projects WHERE id = $1`, projectID).Scan(&workspaceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrProjectMissing
		}
		return "", err
	}
	return workspaceID, nil
}

func (s *Service) projectMemberExists(ctx context.Context, projectID, userID string) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2
		)
	`, projectID, userID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Service) projectMemberExistsByEmail(ctx context.Context, projectID, email string) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM project_members pm
			JOIN users u ON u.id = pm.user_id
			WHERE pm.project_id = $1 AND lower(u.email) = $2
		)
	`, projectID, email).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Service) hasPendingProjectInvitation(ctx context.Context, projectID, email string) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM invitations
			WHERE scope_type = 'project'
			  AND scope_id = $1
			  AND lower(invited_email) = $2
			  AND status = 'pending'
			  AND expires_at > now()
		)
	`, projectID, email).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Service) insertProjectInvitation(ctx context.Context, invitedByUserID, projectID, email, role, token, tokenHash string) (*ProjectInvitationResponse, error) {
	expiresAt := time.Now().Add(projectInvitationTTL)
	var invitation ProjectInvitation
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
		VALUES ('project', $1, $2, $3, $4, $5, $6)
		RETURNING id, scope_type, scope_id, invited_email, role, status, expires_at, created_at
	`, projectID, email, role, tokenHash, expiresAt, invitedByUserID).Scan(&invitation.ID, &invitation.ScopeType, &invitation.ScopeID, &invitation.InvitedEmail, &invitation.Role, &invitation.Status, &invitation.ExpiresAt, &invitation.CreatedAt); err != nil {
		return nil, err
	}

	return &ProjectInvitationResponse{ProjectInvitation: invitation, Token: token}, nil
}

func generateProjectInvitationToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(bytes)
	return token, hashProjectInvitationToken(token), nil
}

func hashProjectInvitationToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeProjectMemberRole(role string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(role))
	if normalized == "" {
		normalized = "developer"
	}
	if _, ok := allowedProjectMemberRoles[normalized]; !ok {
		return "", ErrProjectRoleInvalid
	}
	return normalized, nil
}

func ensureWorkspaceMemberTx(ctx context.Context, tx *sql.Tx, workspaceID, userID string) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2
		)
	`, workspaceID, userID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, 'member')
	`, workspaceID, userID); err != nil {
		return err
	}

	return nil
}

func (s *Service) handleListMembers(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	projectID := chi.URLParam(r, "projectID")
	members, err := s.ListMembers(r.Context(), userID, workspaceID, projectID)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, members)
}

func (s *Service) handleCreateMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req ProjectMemberRequest
	if err := decodeProjectJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	projectID := chi.URLParam(r, "projectID")
	member, err := s.CreateMember(r.Context(), userID, workspaceID, projectID, req)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, member)
}

func (s *Service) handleListMemberInvitations(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	projectID := chi.URLParam(r, "projectID")
	invitations, err := s.ListMemberInvitations(r.Context(), userID, workspaceID, projectID)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, invitations)
}

func (s *Service) handleCreateMemberInvitation(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req ProjectMemberRequest
	if err := decodeProjectJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	projectID := chi.URLParam(r, "projectID")
	invitation, err := s.CreateMemberInvitation(r.Context(), userID, workspaceID, projectID, req)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, invitation)
}

func (s *Service) handleAcceptMemberInvitation(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req AcceptProjectInvitationRequest
	if err := decodeProjectJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	projectID := chi.URLParam(r, "projectID")
	invitation, err := s.AcceptMemberInvitation(r.Context(), userID, workspaceID, projectID, req)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, invitation)
}

func (s *Service) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req ProjectMemberRoleRequest
	if err := decodeProjectJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	projectID := chi.URLParam(r, "projectID")
	memberUserID := chi.URLParam(r, "memberUserID")
	member, err := s.UpdateMember(r.Context(), userID, workspaceID, projectID, memberUserID, req)
	if err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, member)
}

func (s *Service) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	projectID := chi.URLParam(r, "projectID")
	memberUserID := chi.URLParam(r, "memberUserID")
	if err := s.RemoveMember(r.Context(), userID, workspaceID, projectID, memberUserID); err != nil {
		handleProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
