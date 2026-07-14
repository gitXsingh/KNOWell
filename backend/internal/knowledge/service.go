package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/auth"
	"github.com/gitXsingh/knowell/backend/internal/timeline"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	db       *sql.DB
	timeline *timeline.Service
}

type KnowledgeItemResponse struct {
	ID               string     `json:"id"`
	ProjectID        string     `json:"project_id"`
	RepositoryID     string     `json:"repository_id,omitempty"`
	CommitID         string     `json:"commit_id,omitempty"`
	PullRequestID    string     `json:"pull_request_id,omitempty"`
	Title            string     `json:"title"`
	Summary          string     `json:"summary"`
	Body             string     `json:"body"`
	DecisionBody     string     `json:"decision_body"`
	AgentsMd         string     `json:"agents_md"`
	Importance       int        `json:"importance"`
	Status           string     `json:"status"`
	CreatedByUserID  string     `json:"created_by_user_id,omitempty"`
	ApprovedByUserID string     `json:"approved_by_user_id,omitempty"`
	ApprovedAt       *time.Time `json:"approved_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type EditKnowledgeRequest struct {
	DecisionBody string `json:"decision_body"`
	AgentsMd     string `json:"agents_md"`
}

type PromoteRequest struct {
	DraftID string `json:"draft_id"`
}

var (
	ErrKnowledgeMissing = errors.New("knowledge item not found")
	ErrKnowledgeDenied  = errors.New("knowledge access denied")
	ErrDraftNotApproved = errors.New("draft must be approved before promotion")
)

func NewService(database *sql.DB, timelineService *timeline.Service) *Service {
	return &Service{db: database, timeline: timelineService}
}

func (s *Service) Routes(router chi.Router, authMiddleware func(http.Handler) http.Handler) {
	router.With(authMiddleware).Get("/", s.handleList)
	router.With(authMiddleware).Get("/{knowledgeID}", s.handleGet)
	router.With(authMiddleware).Patch("/{knowledgeID}", s.handleEdit)
	router.With(authMiddleware).Post("/promote", s.handlePromote)
}

func (s *Service) List(ctx context.Context, userID, projectID string) ([]KnowledgeItemResponse, error) {
	if !s.canAccessProject(ctx, userID, projectID) {
		return nil, ErrKnowledgeDenied
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, repository_id, commit_id, pull_request_id, title, summary, body, decision_body, agents_md, importance, status, created_by_user_id, approved_by_user_id, approved_at, created_at, updated_at
		FROM knowledge_items
		WHERE project_id = $1
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]KnowledgeItemResponse, 0)
	for rows.Next() {
		item, err := scanKnowledge(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Get(ctx context.Context, userID, projectID, knowledgeID string) (*KnowledgeItemResponse, error) {
	if !s.canAccessProject(ctx, userID, projectID) {
		return nil, ErrKnowledgeDenied
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, repository_id, commit_id, pull_request_id, title, summary, body, decision_body, agents_md, importance, status, created_by_user_id, approved_by_user_id, approved_at, created_at, updated_at
		FROM knowledge_items
		WHERE project_id = $1 AND id = $2
	`, projectID, knowledgeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrKnowledgeMissing
	}

	item, err := scanKnowledge(rows)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) Edit(ctx context.Context, userID, projectID, knowledgeID string, req EditKnowledgeRequest) (*KnowledgeItemResponse, error) {
	if !s.canReviewProject(ctx, userID, projectID) {
		return nil, ErrKnowledgeDenied
	}

	rows, err := s.db.QueryContext(ctx, `
		UPDATE knowledge_items
		SET decision_body = $1, agents_md = $2, updated_at = now()
		WHERE id = $3 AND project_id = $4
		RETURNING id, project_id, repository_id, commit_id, pull_request_id, title, summary, body, decision_body, agents_md, importance, status, created_by_user_id, approved_by_user_id, approved_at, created_at, updated_at
	`, strings.TrimSpace(req.DecisionBody), strings.TrimSpace(req.AgentsMd), knowledgeID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrKnowledgeMissing
	}
	item, err := scanKnowledge(rows)
	if err != nil {
		return nil, err
	}

	var workspaceID string
	_ = s.db.QueryRowContext(ctx, `SELECT workspace_id FROM projects WHERE id = $1`, projectID).Scan(&workspaceID)
	if s.timeline != nil {
		_ = s.timeline.Record(ctx, workspaceID, projectID, userID, "knowledge_edited", "knowledge_item", item.ID, map[string]any{
			"title": item.Title,
		}, "knowledge-edited:"+item.ID)
	}

	return &item, nil
}

func (s *Service) PromoteApprovedDraft(ctx context.Context, userID, projectID, draftID string) (*KnowledgeItemResponse, error) {
	if !s.canReviewProject(ctx, userID, projectID) {
		return nil, ErrKnowledgeDenied
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		repositoryID  sql.NullString
		commitID      sql.NullString
		pullRequestID sql.NullString
		status        string
		title         string
		summary       string
		importance    int
		reason        string
		decisionBody  string
		agentsMd      string
	)
	if err := tx.QueryRowContext(ctx, `
		SELECT repository_id, commit_id, pull_request_id, status, suggested_title, summary, importance, reason, decision_body, agents_md
		FROM ai_drafts
		WHERE id = $1 AND project_id = $2
	`, draftID, projectID).Scan(&repositoryID, &commitID, &pullRequestID, &status, &title, &summary, &importance, &reason, &decisionBody, &agentsMd); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrKnowledgeMissing
		}
		return nil, err
	}

	if status != "approved" {
		return nil, ErrDraftNotApproved
	}

	var existingID string
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM knowledge_items
		WHERE project_id = $1
		  AND ((commit_id IS NOT NULL AND commit_id = $2) OR (pull_request_id IS NOT NULL AND pull_request_id = $3))
	`, projectID, commitID, pullRequestID).Scan(&existingID)
	if err == nil {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, project_id, repository_id, commit_id, pull_request_id, title, summary, body, decision_body, agents_md, importance, status, created_by_user_id, approved_by_user_id, approved_at, created_at, updated_at
			FROM knowledge_items
			WHERE id = $1
		`, existingID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		if rows.Next() {
			item, err := scanKnowledge(rows)
			if err != nil {
				return nil, err
			}
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return &item, nil
		}
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		var kiID string
		_ = s.db.QueryRowContext(ctx, `
			SELECT al.entity_id
			FROM activity_logs al
			WHERE al.dedupe_key = 'draft-approved:' || $1 || ':knowledge'
			LIMIT 1
		`, draftID).Scan(&kiID)
		if kiID != "" {
			existingID = kiID
			err = nil
		}
	}

	body := summary
	if strings.TrimSpace(reason) != "" {
		body += "\n\nReason: " + strings.TrimSpace(reason)
	}

	rows, err := tx.QueryContext(ctx, `
		INSERT INTO knowledge_items (
			project_id,
			repository_id,
			commit_id,
			pull_request_id,
			title,
			summary,
			body,
			decision_body,
			agents_md,
			importance,
			status,
			created_by_user_id,
			approved_by_user_id,
			approved_at,
			search_vector
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'active', $11, $11, now(),
			to_tsvector('english', coalesce($5, '') || ' ' || coalesce($6, '') || ' ' || coalesce($7, ''))
		)
		RETURNING id, project_id, repository_id, commit_id, pull_request_id, title, summary, body, decision_body, agents_md, importance, status, created_by_user_id, approved_by_user_id, approved_at, created_at, updated_at
	`, projectID, repositoryID, commitID, pullRequestID, title, summary, body, decisionBody, agentsMd, importance, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrKnowledgeMissing
	}
	item, err := scanKnowledge(rows)
	if err != nil {
		return nil, err
	}

	var workspaceID string
	_ = tx.QueryRowContext(ctx, `SELECT workspace_id FROM projects WHERE id = $1`, projectID).Scan(&workspaceID)
	if s.timeline != nil {
		if err := s.timeline.Record(ctx, workspaceID, projectID, userID, "draft_approved", "knowledge_item", item.ID, map[string]any{
			"draft_id": draftID,
			"title":    item.Title,
		}, "draft-approved:"+draftID+":knowledge"); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) canAccessProject(ctx context.Context, userID, projectID string) bool {
	var exists bool
	_ = s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2
		)
	`, projectID, userID).Scan(&exists)
	return exists
}

func (s *Service) canReviewProject(ctx context.Context, userID, projectID string) bool {
	var role string
	if err := s.db.QueryRowContext(ctx, `
		SELECT role
		FROM project_members
		WHERE project_id = $1 AND user_id = $2
	`, projectID, userID).Scan(&role); err != nil {
		return false
	}
	return role == "owner" || role == "tech_lead" || role == "developer"
}

func scanKnowledge(scanner interface{ Scan(...any) error }) (KnowledgeItemResponse, error) {
	var (
		id               string
		projectID        string
		repositoryID     sql.NullString
		commitID         sql.NullString
		pullRequestID    sql.NullString
		title            string
		summary          string
		body             string
		decisionBody     string
		agentsMd         string
		importance       int
		status           string
		createdByUserID  sql.NullString
		approvedByUserID sql.NullString
		approvedAt       sql.NullTime
		createdAt        time.Time
		updatedAt        time.Time
	)

	if err := scanner.Scan(&id, &projectID, &repositoryID, &commitID, &pullRequestID, &title, &summary, &body, &decisionBody, &agentsMd, &importance, &status, &createdByUserID, &approvedByUserID, &approvedAt, &createdAt, &updatedAt); err != nil {
		return KnowledgeItemResponse{}, err
	}

	item := KnowledgeItemResponse{
		ID:           id,
		ProjectID:    projectID,
		Title:        title,
		Summary:      summary,
		Body:         body,
		DecisionBody: decisionBody,
		AgentsMd:     agentsMd,
		Importance:   importance,
		Status:       status,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
	if repositoryID.Valid {
		item.RepositoryID = repositoryID.String
	}
	if commitID.Valid {
		item.CommitID = commitID.String
	}
	if pullRequestID.Valid {
		item.PullRequestID = pullRequestID.String
	}
	if createdByUserID.Valid {
		item.CreatedByUserID = createdByUserID.String
	}
	if approvedByUserID.Valid {
		item.ApprovedByUserID = approvedByUserID.String
	}
	if approvedAt.Valid {
		value := approvedAt.Time
		item.ApprovedAt = &value
	}
	return item, nil
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	projectID := chi.URLParam(r, "projectID")
	items, err := s.List(r.Context(), userID, projectID)
	if err != nil {
		handleKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	projectID := chi.URLParam(r, "projectID")
	knowledgeID := chi.URLParam(r, "knowledgeID")
	item, err := s.Get(r.Context(), userID, projectID, knowledgeID)
	if err != nil {
		handleKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Service) handleEdit(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var req EditKnowledgeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}
	projectID := chi.URLParam(r, "projectID")
	knowledgeID := chi.URLParam(r, "knowledgeID")
	item, err := s.Edit(r.Context(), userID, projectID, knowledgeID, req)
	if err != nil {
		handleKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Service) handlePromote(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var req PromoteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}
	projectID := chi.URLParam(r, "projectID")
	item, err := s.PromoteApprovedDraft(r.Context(), userID, projectID, req.DraftID)
	if err != nil {
		handleKnowledgeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func handleKnowledgeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrKnowledgeDenied):
		writeError(w, http.StatusForbidden, "forbidden", "You do not have access to this knowledge base")
	case errors.Is(err, ErrKnowledgeMissing):
		writeError(w, http.StatusNotFound, "knowledge_not_found", "Knowledge item not found")
	case errors.Is(err, ErrDraftNotApproved):
		writeError(w, http.StatusUnprocessableEntity, "draft_not_approved", "Draft must be approved before promotion")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong")
	}
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
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
