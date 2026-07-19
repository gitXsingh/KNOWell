package workspace

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/common/validate"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const joinKeyPrefix = "TEAM-"

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func isValidUUID(s string) bool {
	return uuidRe.MatchString(strings.ToLower(s))
}

type Service struct {
	db *sql.DB
}

type Workspace struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_user_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Kind      string    `json:"kind"`
	JoinKey   string    `json:"join_key,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Member struct {
	UserID   string    `json:"user_id"`
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type WorkspaceRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	Kind string `json:"kind"`
}

type JoinByKeyRequest struct {
	Key string `json:"key"`
}

type JoinKeyResponse struct {
	Key string `json:"join_key"`
}

type JoinRequest struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	UserID          string     `json:"user_id"`
	UserEmail       string     `json:"user_email"`
	UserFullName    string     `json:"user_full_name"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	ReviewedByID    *string    `json:"reviewed_by_user_id,omitempty"`
}

type ApproveRejectRequest struct {
	UserID string `json:"user_id"`
	Action string `json:"action"`
}

var (
	ErrWorkspaceExists          = errors.New("workspace already exists")
	ErrWorkspaceDenied          = errors.New("workspace access denied")
	ErrWorkspaceMissing         = errors.New("workspace not found")
	ErrWorkspaceJoinKeyInvalid  = errors.New("invalid workspace join key")
	ErrWorkspaceAlreadyMember   = errors.New("already a member of this workspace")
	ErrJoinRequestExists        = errors.New("join request already exists")
	ErrJoinRequestMissing       = errors.New("join request not found")
	ErrJoinRequestAlreadyReviewed = errors.New("join request already reviewed")
)

func NewService(database *sql.DB) *Service {
	return &Service{db: database}
}

func (s *Service) Routes(router chi.Router, authMiddleware func(http.Handler) http.Handler) {
	router.With(authMiddleware).Get("/", s.handleList)
	router.With(authMiddleware).Post("/", s.handleCreate)
	router.With(authMiddleware).Post("/join", s.handleJoinByKey)
	router.With(authMiddleware).Get("/{workspaceID}", s.handleGet)
	router.With(authMiddleware).Patch("/{workspaceID}", s.handleUpdate)
	router.With(authMiddleware).Delete("/{workspaceID}", s.handleDelete)
	router.With(authMiddleware).Get("/{workspaceID}/members", s.handleMembers)
	router.With(authMiddleware).Get("/{workspaceID}/invitations", s.handleListInvitations)
	router.With(authMiddleware).Post("/{workspaceID}/invitations", s.handleCreateInvitation)
	router.With(authMiddleware).Post("/{workspaceID}/invitations/accept", s.handleAcceptInvitation)
	router.With(authMiddleware).Delete("/{workspaceID}/members/{memberUserID}", s.handleRemoveMember)
	router.With(authMiddleware).Get("/{workspaceID}/join-key", s.handleGetJoinKey)
	router.With(authMiddleware).Post("/{workspaceID}/join-key/regenerate", s.handleRegenerateJoinKey)
	router.With(authMiddleware).Get("/{workspaceID}/join-requests", s.handleListJoinRequests)
	router.With(authMiddleware).Post("/{workspaceID}/join-requests/approve", s.handleApproveRejectJoinRequest)
}

func (s *Service) List(ctx context.Context, userID string) ([]Workspace, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT w.id, w.owner_user_id, w.name, w.slug, w.kind, w.created_at, w.updated_at
		FROM workspaces w
		JOIN workspace_members wm ON wm.workspace_id = w.id
		WHERE wm.user_id = $1
		ORDER BY w.created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Workspace, 0)
	for rows.Next() {
		var item Workspace
		if err := rows.Scan(&item.ID, &item.OwnerID, &item.Name, &item.Slug, &item.Kind, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *Service) Create(ctx context.Context, userID string, req WorkspaceRequest) (*Workspace, error) {
	if err := validate.Name(req.Name, 200); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)

	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "team"
	}
	if kind != "personal" && kind != "team" {
		return nil, fmt.Errorf("invalid workspace kind")
	}

	slug := slugify(req.Slug)
	if slug == "" {
		slug = slugify(name)
	}

	joinKey := ""
	if kind == "team" {
		var err error
		joinKey, err = generateJoinKey()
		if err != nil {
			return nil, err
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var workspace Workspace
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO workspaces (owner_user_id, name, slug, kind, join_key)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, owner_user_id, name, slug, kind, created_at, updated_at
	`, userID, name, slug, kind, joinKey).Scan(&workspace.ID, &workspace.OwnerID, &workspace.Name, &workspace.Slug, &workspace.Kind, &workspace.CreatedAt, &workspace.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrWorkspaceExists
		}
		return nil, err
	}
	workspace.JoinKey = joinKey

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, workspace.ID, userID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &workspace, nil
}

func (s *Service) Get(ctx context.Context, userID, workspaceID string) (*Workspace, error) {
	if !isValidUUID(workspaceID) {
		return nil, ErrWorkspaceMissing
	}
	var workspace Workspace
	if err := s.db.QueryRowContext(ctx, `
		SELECT w.id, w.owner_user_id, w.name, w.slug, w.kind, w.created_at, w.updated_at
		FROM workspaces w
		JOIN workspace_members wm ON wm.workspace_id = w.id
		WHERE w.id = $1::uuid AND wm.user_id = $2::uuid
	`, workspaceID, userID).Scan(&workspace.ID, &workspace.OwnerID, &workspace.Name, &workspace.Slug, &workspace.Kind, &workspace.CreatedAt, &workspace.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkspaceMissing
		}
		return nil, err
	}

	return &workspace, nil
}

func (s *Service) Update(ctx context.Context, userID, workspaceID string, req WorkspaceRequest) (*Workspace, error) {
	workspace, err := s.Get(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}

	isOwner, err := s.isOwner(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, ErrWorkspaceDenied
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = workspace.Name
	}

	slug := slugify(req.Slug)
	if slug == "" {
		slug = workspace.Slug
	}

	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = workspace.Kind
	}
	if kind != "personal" && kind != "team" {
		return nil, fmt.Errorf("invalid workspace kind")
	}

	if err := s.db.QueryRowContext(ctx, `
		UPDATE workspaces
		SET name = $1, slug = $2, kind = $3, updated_at = now()
		WHERE id = $4
		RETURNING id, owner_user_id, name, slug, kind, created_at, updated_at
	`, name, slug, kind, workspaceID).Scan(&workspace.ID, &workspace.OwnerID, &workspace.Name, &workspace.Slug, &workspace.Kind, &workspace.CreatedAt, &workspace.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrWorkspaceExists
		}
		return nil, err
	}

	return workspace, nil
}

func (s *Service) Delete(ctx context.Context, userID, workspaceID string) error {
	isOwner, err := s.isOwner(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !isOwner {
		return ErrWorkspaceDenied
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = $1`, workspaceID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrWorkspaceMissing
	}

	return nil
}

func (s *Service) Members(ctx context.Context, userID, workspaceID string) ([]Member, error) {
	if _, err := s.Get(ctx, userID, workspaceID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT wm.user_id, u.email, u.full_name, wm.role, wm.joined_at
		FROM workspace_members wm
		JOIN users u ON u.id = wm.user_id
		WHERE wm.workspace_id = $1::uuid
		ORDER BY wm.joined_at ASC
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]Member, 0)
	for rows.Next() {
		var member Member
		if err := rows.Scan(&member.UserID, &member.Email, &member.FullName, &member.Role, &member.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	return members, rows.Err()
}

func (s *Service) isOwner(ctx context.Context, workspaceID, userID string) (bool, error) {
	var role string
	if err := s.db.QueryRowContext(ctx, `
		SELECT role FROM workspace_members WHERE workspace_id = $1::uuid AND user_id = $2::uuid
	`, workspaceID, userID).Scan(&role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrWorkspaceMissing
		}
		return false, err
	}

	return role == "owner", nil
}

func (s *Service) JoinByKey(ctx context.Context, userID, key string) (*Workspace, error) {
	key = strings.TrimSpace(strings.ToUpper(key))
	if key == "" {
		return nil, ErrWorkspaceJoinKeyInvalid
	}

	var workspace Workspace
	err := s.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, name, slug, kind, created_at, updated_at
		FROM workspaces
		WHERE kind = 'team' AND join_key = $1
	`, key).Scan(&workspace.ID, &workspace.OwnerID, &workspace.Name, &workspace.Slug, &workspace.Kind, &workspace.CreatedAt, &workspace.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkspaceJoinKeyInvalid
		}
		return nil, err
	}

	var memberExists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM workspace_members WHERE workspace_id = $1::uuid AND user_id = $2::uuid)
	`, workspace.ID, userID).Scan(&memberExists); err != nil {
		return nil, err
	}
	if memberExists {
		return nil, ErrWorkspaceAlreadyMember
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO join_requests (workspace_id, user_id, status)
		VALUES ($1::uuid, $2::uuid, 'pending')
		ON CONFLICT (workspace_id, user_id) DO UPDATE SET status = 'pending', reviewed_at = NULL, reviewed_by_user_id = NULL
	`, workspace.ID, userID); err != nil {
		return nil, err
	}

	return &workspace, nil
}

func (s *Service) ListJoinRequests(ctx context.Context, userID, workspaceID string) ([]JoinRequest, error) {
	isOwner, err := s.isOwner(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, ErrWorkspaceDenied
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT jr.id, jr.workspace_id, jr.user_id, u.email, u.full_name, jr.status, jr.created_at, jr.reviewed_at, jr.reviewed_by_user_id
		FROM join_requests jr
		JOIN users u ON u.id = jr.user_id
		WHERE jr.workspace_id = $1::uuid
		ORDER BY jr.created_at DESC
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]JoinRequest, 0)
	for rows.Next() {
		var jr JoinRequest
		if err := rows.Scan(&jr.ID, &jr.WorkspaceID, &jr.UserID, &jr.UserEmail, &jr.UserFullName, &jr.Status, &jr.CreatedAt, &jr.ReviewedAt, &jr.ReviewedByID); err != nil {
			return nil, err
		}
		requests = append(requests, jr)
	}
	return requests, rows.Err()
}

func (s *Service) ApproveJoinRequest(ctx context.Context, ownerID, workspaceID, requesterUserID string) (*JoinRequest, error) {
	isOwner, err := s.isOwner(ctx, workspaceID, ownerID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, ErrWorkspaceDenied
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var jr JoinRequest
	err = tx.QueryRowContext(ctx, `
		SELECT id, workspace_id, user_id, status, created_at, reviewed_at
		FROM join_requests
		WHERE workspace_id = $1::uuid AND user_id = $2::uuid
		FOR UPDATE
	`, workspaceID, requesterUserID).Scan(&jr.ID, &jr.WorkspaceID, &jr.UserID, &jr.Status, &jr.CreatedAt, &jr.ReviewedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJoinRequestMissing
		}
		return nil, err
	}
	if jr.Status != "pending" {
		return nil, ErrJoinRequestAlreadyReviewed
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1::uuid, $2::uuid, 'member')
	`, workspaceID, requesterUserID); err != nil {
		return nil, err
	}

	now := time.Now()
	if _, err := tx.ExecContext(ctx, `
		UPDATE join_requests
		SET status = 'approved', reviewed_at = $1, reviewed_by_user_id = $2
		WHERE id = $3
	`, now, ownerID, jr.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	jr.Status = "approved"
	jr.ReviewedAt = &now
	jr.ReviewedByID = &ownerID
	return &jr, nil
}

func (s *Service) RejectJoinRequest(ctx context.Context, ownerID, workspaceID, requesterUserID string) (*JoinRequest, error) {
	isOwner, err := s.isOwner(ctx, workspaceID, ownerID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, ErrWorkspaceDenied
	}

	var jr JoinRequest
	err = s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, user_id, status, created_at, reviewed_at
		FROM join_requests
		WHERE workspace_id = $1::uuid AND user_id = $2::uuid
	`, workspaceID, requesterUserID).Scan(&jr.ID, &jr.WorkspaceID, &jr.UserID, &jr.Status, &jr.CreatedAt, &jr.ReviewedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJoinRequestMissing
		}
		return nil, err
	}
	if jr.Status != "pending" {
		return nil, ErrJoinRequestAlreadyReviewed
	}

	now := time.Now()
	if _, err := s.db.ExecContext(ctx, `
		UPDATE join_requests
		SET status = 'rejected', reviewed_at = $1, reviewed_by_user_id = $2
		WHERE id = $3
	`, now, ownerID, jr.ID); err != nil {
		return nil, err
	}

	jr.Status = "rejected"
	jr.ReviewedAt = &now
	jr.ReviewedByID = &ownerID
	return &jr, nil
}

func (s *Service) GetJoinKey(ctx context.Context, userID, workspaceID string) (*JoinKeyResponse, error) {
	if _, err := s.Get(ctx, userID, workspaceID); err != nil {
		return nil, err
	}

	var key string
	if err := s.db.QueryRowContext(ctx, `
		SELECT join_key FROM workspaces WHERE id = $1
	`, workspaceID).Scan(&key); err != nil {
		return nil, err
	}

	return &JoinKeyResponse{Key: key}, nil
}

func (s *Service) RegenerateJoinKey(ctx context.Context, userID, workspaceID string) (*JoinKeyResponse, error) {
	isOwner, err := s.isOwner(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, ErrWorkspaceDenied
	}

	newKey, err := generateJoinKey()
	if err != nil {
		return nil, err
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE workspaces SET join_key = $1 WHERE id = $2
	`, newKey, workspaceID); err != nil {
		return nil, err
	}

	return &JoinKeyResponse{Key: newKey}, nil
}

func generateJoinKey() (string, error) {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	raw := strings.ToUpper(hex.EncodeToString(bytes))
	key := joinKeyPrefix + raw[:4] + "-" + raw[4:8] + "-" + raw[8:]
	return key, nil
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

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "'", "")
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
