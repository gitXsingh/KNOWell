package project

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/ai"
	"github.com/gitXsingh/knowell/backend/internal/common/validate"
	gh "github.com/gitXsingh/knowell/backend/internal/github"
	"github.com/gitXsingh/knowell/backend/internal/knowledge"
	"github.com/gitXsingh/knowell/backend/internal/search"
	"github.com/gitXsingh/knowell/backend/internal/timeline"
	"github.com/gitXsingh/knowell/backend/internal/webhook"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Service struct {
	db        *sql.DB
	github    *gh.Service
	ai        *ai.Service
	webhook   *webhook.Service
	knowledge *knowledge.Service
	search    *search.Service
	timeline  *timeline.Service
}

type Project struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"workspace_id"`
	Name            string    `json:"name"`
	Slug            string    `json:"slug"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`
	CreatedByUserID string    `json:"created_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Sources         []Source  `json:"sources,omitempty"`
}

type Source struct {
	ID         string         `json:"id"`
	ProjectID  string         `json:"project_id"`
	SourceKey  string         `json:"source_key"`
	Enabled    bool           `json:"enabled"`
	ConfigJSON map[string]any `json:"config_json"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type ProjectRequest struct {
	Name             string   `json:"name"`
	Slug             string   `json:"slug"`
	Description      string   `json:"description"`
	Status           string   `json:"status"`
	KnowledgeSources []string `json:"knowledge_sources"`
}

type SourceSettingsRequest struct {
	KnowledgeSources []string `json:"knowledge_sources"`
	Status           string   `json:"status"`
}

type RepositoryConnectionRequest struct {
	Owner    string `json:"owner"`
	RepoName string `json:"repo_name"`
}

type RepositoryRecord struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Provider      string    `json:"provider"`
	Owner         string    `json:"owner"`
	RepoName      string    `json:"repo_name"`
	FullName      string    `json:"full_name"`
	DefaultBranch string    `json:"default_branch"`
	WebhookID     string    `json:"webhook_id"`
	ConnectedAt   time.Time `json:"connected_at"`
	Status        string    `json:"status"`
	WebhookURL    string    `json:"webhook_url,omitempty"`
}

var (
	ErrProjectExists      = errors.New("project already exists")
	ErrProjectDenied      = errors.New("project access denied")
	ErrProjectMissing     = errors.New("project not found")
	ErrProjectInvalid     = errors.New("invalid project")
	ErrProjectState       = errors.New("invalid project state")
	ErrProjectSource      = errors.New("invalid project source")
	ErrRepositoryDisabled = errors.New("github repository source is not enabled")
	ErrRepositoryMissing  = errors.New("repository not connected")
)

var allowedSourceKeys = map[string]struct{}{
	"github_repository":      {},
	"manual_notes":           {},
	"api_documentation":      {},
	"architecture_decisions": {},
	"meeting_notes":          {},
	"deployment_notes":       {},
	"database_schema":        {},
}

func NewService(database *sql.DB, githubService *gh.Service, aiService *ai.Service, webhookService *webhook.Service, knowledgeService *knowledge.Service, searchService *search.Service, timelineService *timeline.Service) *Service {
	return &Service{db: database, github: githubService, ai: aiService, webhook: webhookService, knowledge: knowledgeService, search: searchService, timeline: timelineService}
}

func (s *Service) Routes(router chi.Router, authMiddleware func(http.Handler) http.Handler) {
	router.With(authMiddleware).Get("/", s.handleList)
	router.With(authMiddleware).Post("/", s.handleCreate)
	router.With(authMiddleware).Get("/{projectID}", s.handleGet)
	router.With(authMiddleware).Patch("/{projectID}", s.handleUpdate)
	router.With(authMiddleware).Delete("/{projectID}", s.handleDelete)
	router.With(authMiddleware).Get("/{projectID}/settings", s.handleSettings)
	router.With(authMiddleware).Patch("/{projectID}/settings", s.handleUpdateSettings)
	router.With(authMiddleware).Get("/{projectID}/members", s.handleListMembers)
	router.With(authMiddleware).Post("/{projectID}/members", s.handleCreateMember)
	router.With(authMiddleware).Post("/{projectID}/members/accept", s.handleAcceptMemberInvitation)
	router.With(authMiddleware).Post("/{projectID}/members/invitations", s.handleCreateMemberInvitation)
	router.With(authMiddleware).Get("/{projectID}/members/invitations", s.handleListMemberInvitations)
	router.With(authMiddleware).Patch("/{projectID}/members/{memberUserID}", s.handleUpdateMember)
	router.With(authMiddleware).Delete("/{projectID}/members/{memberUserID}", s.handleRemoveMember)
	router.With(authMiddleware).Get("/{projectID}/repository/options", s.handleListRepositoryOptions)
	router.With(authMiddleware).Get("/{projectID}/repository", s.handleGetRepository)
	router.With(authMiddleware).Put("/{projectID}/repository", s.handleConnectRepository)
	router.With(authMiddleware).Delete("/{projectID}/repository", s.handleDeleteRepository)
	router.With(authMiddleware).Post("/{projectID}/repository/webhook/sync", s.handleSyncRepositoryWebhook)
	router.Route("/{projectID}/drafts", func(r chi.Router) {
		if s.ai != nil {
			s.ai.Routes(r, authMiddleware)
		}
	})
	router.With(authMiddleware).Get("/{projectID}/webhook-events", s.handleListWebhookEvents)
	router.With(authMiddleware).Post("/{projectID}/webhook-events/process", s.handleProcessWebhookEvents)
	router.Route("/{projectID}/knowledge-items", func(r chi.Router) {
		if s.knowledge != nil {
			s.knowledge.Routes(r, authMiddleware)
		}
	})
	router.Route("/{projectID}/search", func(r chi.Router) {
		if s.search != nil {
			s.search.Routes(r, authMiddleware)
		}
	})
	router.Route("/{projectID}/timeline", func(r chi.Router) {
		if s.timeline != nil {
			s.timeline.Routes(r, authMiddleware)
		}
	})
}

func (s *Service) List(ctx context.Context, userID, workspaceID string) ([]Project, error) {
	if !s.canAccessWorkspace(ctx, userID, workspaceID) {
		return nil, ErrProjectDenied
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workspace_id, name, slug, description, status, created_by_user_id, created_at, updated_at
		FROM projects
		WHERE workspace_id = $1
		ORDER BY created_at ASC
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.WorkspaceID, &project.Name, &project.Slug, &project.Description, &project.Status, &project.CreatedByUserID, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, rows.Err()
}

func (s *Service) Create(ctx context.Context, userID, workspaceID string, req ProjectRequest) (*Project, error) {
	if !s.canAccessWorkspace(ctx, userID, workspaceID) {
		return nil, ErrProjectDenied
	}

	if err := validate.Name(req.Name, 200); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)

	slug := slugify(req.Slug)
	if slug == "" {
		slug = slugify(name)
	}

	description := strings.TrimSpace(req.Description)
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "archived" && status != "completed" {
		return nil, ErrProjectState
	}

	sources, err := normalizeSources(req.KnowledgeSources)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var project Project
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO projects (workspace_id, name, slug, description, status, created_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, workspace_id, name, slug, description, status, created_by_user_id, created_at, updated_at
	`, workspaceID, name, slug, description, status, userID).Scan(&project.ID, &project.WorkspaceID, &project.Name, &project.Slug, &project.Description, &project.Status, &project.CreatedByUserID, &project.CreatedAt, &project.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrProjectExists
		}
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_members (project_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, project.ID, userID); err != nil {
		return nil, err
	}

	if err := s.upsertSourcesTx(ctx, tx, project.ID, sources); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	project.Sources = sources
	return &project, nil
}

func (s *Service) Get(ctx context.Context, userID, projectID string) (*Project, error) {
	var project Project
	if err := s.db.QueryRowContext(ctx, `
		SELECT p.id, p.workspace_id, p.name, p.slug, p.description, p.status, p.created_by_user_id, p.created_at, p.updated_at
		FROM projects p
		JOIN workspace_members wm ON wm.workspace_id = p.workspace_id
		WHERE p.id = $1 AND wm.user_id = $2
	`, projectID, userID).Scan(&project.ID, &project.WorkspaceID, &project.Name, &project.Slug, &project.Description, &project.Status, &project.CreatedByUserID, &project.CreatedAt, &project.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectMissing
		}
		return nil, err
	}

	sources, err := s.listSources(ctx, projectID)
	if err != nil {
		return nil, err
	}
	project.Sources = sources

	return &project, nil
}

func (s *Service) Update(ctx context.Context, userID, projectID string, req ProjectRequest) (*Project, error) {
	project, err := s.Get(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}

	if !s.canEditProject(ctx, userID, projectID) {
		return nil, ErrProjectDenied
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = project.Name
	}

	slug := slugify(req.Slug)
	if slug == "" {
		slug = project.Slug
	}

	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = project.Description
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = project.Status
	}
	if status != "active" && status != "archived" && status != "completed" {
		return nil, ErrProjectState
	}

	if err := s.db.QueryRowContext(ctx, `
		UPDATE projects
		SET name = $1, slug = $2, description = $3, status = $4, updated_at = now()
		WHERE id = $5
		RETURNING id, workspace_id, name, slug, description, status, created_by_user_id, created_at, updated_at
	`, name, slug, description, status, projectID).Scan(&project.ID, &project.WorkspaceID, &project.Name, &project.Slug, &project.Description, &project.Status, &project.CreatedByUserID, &project.CreatedAt, &project.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrProjectExists
		}
		return nil, err
	}

	return project, nil
}

func (s *Service) Delete(ctx context.Context, userID, projectID string) error {
	if !s.canEditProject(ctx, userID, projectID) {
		return ErrProjectDenied
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = $1`, projectID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrProjectMissing
	}

	return nil
}

func (s *Service) Settings(ctx context.Context, userID, projectID string) (*Project, error) {
	return s.Get(ctx, userID, projectID)
}

func (s *Service) UpdateSettings(ctx context.Context, userID, projectID string, req SourceSettingsRequest) (*Project, error) {
	project, err := s.Get(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	if !s.canEditProject(ctx, userID, projectID) {
		return nil, ErrProjectDenied
	}

	sources, err := normalizeSources(req.KnowledgeSources)
	if err != nil {
		return nil, err
	}

	status := strings.TrimSpace(req.Status)
	if status != "" {
		if status != "active" && status != "archived" && status != "completed" {
			return nil, ErrProjectState
		}
		project.Status = status
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if status != "" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE projects
			SET status = $1, updated_at = now()
			WHERE id = $2
		`, status, projectID); err != nil {
			return nil, err
		}
	}

	if err := s.upsertSourcesTx(ctx, tx, projectID, sources); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	project.Sources = sources
	if status != "" {
		project.Status = status
	}

	return project, nil
}

func (s *Service) upsertSourcesTx(ctx context.Context, tx *sql.Tx, projectID string, sources []Source) error {
	existing, err := s.listSourcesTx(ctx, tx, projectID)
	if err != nil {
		return err
	}

	existingMap := make(map[string]Source, len(existing))
	for _, source := range existing {
		existingMap[source.SourceKey] = source
	}

	for _, source := range sources {
		payload, err := json.Marshal(source.ConfigJSON)
		if err != nil {
			return err
		}

		if _, ok := existingMap[source.SourceKey]; ok {
			if _, err := tx.ExecContext(ctx, `
				UPDATE project_sources
				SET enabled = $1, config_json = $2, updated_at = now()
				WHERE project_id = $3 AND source_key = $4
			`, source.Enabled, payload, projectID, source.SourceKey); err != nil {
				return err
			}
			delete(existingMap, source.SourceKey)
			continue
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO project_sources (project_id, source_key, enabled, config_json)
			VALUES ($1, $2, $3, $4)
		`, projectID, source.SourceKey, source.Enabled, payload); err != nil {
			return err
		}
	}

	for key := range existingMap {
		if _, err := tx.ExecContext(ctx, `DELETE FROM project_sources WHERE project_id = $1 AND source_key = $2`, projectID, key); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) listSources(ctx context.Context, projectID string) ([]Source, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, source_key, enabled, config_json, created_at, updated_at
		FROM project_sources
		WHERE project_id = $1
		ORDER BY source_key ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSources(rows)
}

func (s *Service) listSourcesTx(ctx context.Context, tx *sql.Tx, projectID string) ([]Source, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, project_id, source_key, enabled, config_json, created_at, updated_at
		FROM project_sources
		WHERE project_id = $1
		ORDER BY source_key ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSources(rows)
}

func scanSources(rows *sql.Rows) ([]Source, error) {
	sources := make([]Source, 0)
	for rows.Next() {
		var source Source
		var payload []byte
		if err := rows.Scan(&source.ID, &source.ProjectID, &source.SourceKey, &source.Enabled, &payload, &source.CreatedAt, &source.UpdatedAt); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &source.ConfigJSON); err != nil {
				return nil, err
			}
		}
		if source.ConfigJSON == nil {
			source.ConfigJSON = map[string]any{}
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func normalizeSources(keys []string) ([]Source, error) {
	seen := make(map[string]struct{}, len(keys))
	sources := make([]Source, 0, len(keys))
	for _, key := range keys {
		normalized := strings.ToLower(strings.TrimSpace(key))
		normalized = strings.ReplaceAll(normalized, "-", "_")
		if normalized == "" {
			continue
		}
		if _, ok := allowedSourceKeys[normalized]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrProjectSource, normalized)
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		sources = append(sources, Source{SourceKey: normalized, Enabled: true, ConfigJSON: map[string]any{}})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].SourceKey < sources[j].SourceKey })
	return sources, nil
}

func (s *Service) canAccessWorkspace(ctx context.Context, userID, workspaceID string) bool {
	var exists bool
	_ = s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2
		)
	`, workspaceID, userID).Scan(&exists)
	return exists
}

func (s *Service) canEditProject(ctx context.Context, userID, projectID string) bool {
	var role string
	if err := s.db.QueryRowContext(ctx, `
		SELECT pm.role
		FROM project_members pm
		WHERE pm.project_id = $1 AND pm.user_id = $2
	`, projectID, userID).Scan(&role); err != nil {
		return false
	}
	return role == "owner" || role == "tech_lead"
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "'", "")
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func generateWebhookSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
