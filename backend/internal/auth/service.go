package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/common/config"
	"github.com/gitXsingh/knowell/backend/internal/common/validate"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Service struct {
	db  *sql.DB
	cfg config.Config
}

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WorkspaceSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Kind string `json:"kind"`
	Role string `json:"role"`
}

type SessionResponse struct {
	User       User               `json:"user"`
	Workspaces []WorkspaceSummary `json:"workspaces"`
	Token      string             `json:"token"`
}

type RegisterRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	FullName      string `json:"full_name"`
	WorkspaceName string `json:"workspace_name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailExists        = errors.New("email already exists")
)

func NewService(database *sql.DB, cfg config.Config) *Service {
	return &Service{db: database, cfg: cfg}
}

func (s *Service) Routes(router chi.Router) {
	router.Post("/register", s.handleRegister)
	router.Post("/login", s.handleLogin)
	router.Post("/logout", s.handleLogout)
	router.With(s.authMiddleware).Get("/me", s.handleMe)
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (*SessionResponse, error) {
	if err := validate.Email(req.Email); err != nil {
		return nil, err
	}
	if err := validate.Password(req.Password); err != nil {
		return nil, err
	}
	if err := validate.Name(req.FullName, 200); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.WorkspaceName) != "" {
		if err := validate.Name(req.WorkspaceName, 200); err != nil {
			return nil, err
		}
	}

	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	workspaceName := strings.TrimSpace(req.WorkspaceName)
	if workspaceName == "" {
		workspaceName = strings.TrimSpace(req.FullName) + "'s Workspace"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var userID string
	var createdAt time.Time
	var updatedAt time.Time
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, full_name, status)
		VALUES ($1, $2, $3, 'active')
		RETURNING id, created_at, updated_at
	`, strings.ToLower(strings.TrimSpace(req.Email)), passwordHash, strings.TrimSpace(req.FullName)).Scan(&userID, &createdAt, &updatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailExists
		}
		return nil, err
	}

	workspaceSlug := slugify(workspaceName)
	var workspaceID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO workspaces (owner_user_id, name, slug, kind)
		VALUES ($1, $2, $3, 'personal')
		RETURNING id
	`, userID, workspaceName, workspaceSlug).Scan(&workspaceID); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, workspaceID, userID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	user := User{
		ID:        userID,
		Email:     strings.ToLower(strings.TrimSpace(req.Email)),
		FullName:  strings.TrimSpace(req.FullName),
		Status:    "active",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	token, err := s.issueToken(userID, user.Email)
	if err != nil {
		return nil, err
	}

	workspaces, err := s.listWorkspaces(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &SessionResponse{User: user, Workspaces: workspaces, Token: token}, nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*SessionResponse, error) {
	var user User
	var passwordHash string
	query := `
		SELECT id, email, full_name, password_hash, status, created_at, updated_at
		FROM users
		WHERE lower(email) = lower($1)
	`
	if err := s.db.QueryRowContext(ctx, query, strings.TrimSpace(req.Email)).Scan(
		&user.ID, &user.Email, &user.FullName, &passwordHash, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := VerifyPassword(passwordHash, req.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.issueToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	workspaces, err := s.listWorkspaces(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &SessionResponse{User: user, Workspaces: workspaces, Token: token}, nil
}

func (s *Service) Me(ctx context.Context, userID string) (*SessionResponse, error) {
	var user User
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, email, full_name, status, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.FullName, &user.Status, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return nil, err
	}

	workspaces, err := s.listWorkspaces(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &SessionResponse{User: user, Workspaces: workspaces}, nil
}

func (s *Service) issueToken(userID, email string) (string, error) {
	return SignToken(s.cfg.JWTSecret, userID, email, s.cfg.JWTAccessTTL)
}

func (s *Service) listWorkspaces(ctx context.Context, userID string) ([]WorkspaceSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT w.id, w.name, w.slug, w.kind, wm.role
		FROM workspaces w
		JOIN workspace_members wm ON wm.workspace_id = w.id
		WHERE wm.user_id = $1
		ORDER BY w.created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	workspaces := make([]WorkspaceSummary, 0)
	for rows.Next() {
		var workspace WorkspaceSummary
		if err := rows.Scan(&workspace.ID, &workspace.Name, &workspace.Slug, &workspace.Kind, &workspace.Role); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, workspace)
	}

	return workspaces, rows.Err()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
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
