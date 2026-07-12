package ai

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
	"github.com/gitXsingh/knowell/backend/internal/common/config"
	"github.com/gitXsingh/knowell/backend/internal/timeline"
	"github.com/go-chi/chi/v5"
)

type Provider interface {
	Name() string
	GenerateCommitDraft(ctx context.Context, input CommitInput) (*DraftOutput, error)
	GeneratePullRequestDraft(ctx context.Context, input PullRequestInput) (*DraftOutput, error)
}

type Service struct {
	db       *sql.DB
	provider Provider
	timeline *timeline.Service
}

type Draft struct {
	ID               string         `json:"id"`
	ProjectID        string         `json:"project_id"`
	RepositoryID     string         `json:"repository_id"`
	CommitID         sql.NullString `json:"-"`
	PullRequestID    sql.NullString `json:"-"`
	SourceType       string         `json:"source_type"`
	Status           string         `json:"status"`
	SuggestedTitle   string         `json:"suggested_title"`
	Summary          string         `json:"summary"`
	Importance       int            `json:"importance"`
	Reason           string         `json:"reason"`
	AIProvider       string         `json:"ai_provider"`
	Version          int            `json:"version"`
	CreatedByUserID  sql.NullString `json:"-"`
	ReviewedByUserID sql.NullString `json:"-"`
	ReviewedAt       sql.NullTime   `json:"-"`
	RawInputJSON     map[string]any `json:"raw_input_json"`
}

type DraftResponse struct {
	ID               string         `json:"id"`
	ProjectID        string         `json:"project_id"`
	RepositoryID     string         `json:"repository_id"`
	CommitID         string         `json:"commit_id,omitempty"`
	PullRequestID    string         `json:"pull_request_id,omitempty"`
	SourceType       string         `json:"source_type"`
	Status           string         `json:"status"`
	SuggestedTitle   string         `json:"suggested_title"`
	Summary          string         `json:"summary"`
	Importance       int            `json:"importance"`
	Reason           string         `json:"reason"`
	DecisionBody     string         `json:"decision_body"`
	AgentsMd         string         `json:"agents_md"`
	AIProvider       string         `json:"ai_provider"`
	Version          int            `json:"version"`
	ReviewedByUserID string         `json:"reviewed_by_user_id,omitempty"`
	ReviewedAt       *time.Time     `json:"reviewed_at,omitempty"`
	RawInputJSON     map[string]any `json:"raw_input_json"`
}

type ReviewRequest struct {
	Status string `json:"status"`
}

type CommitInput struct {
	ProjectID    string
	RepositoryID string
	CommitID     string
	SHA          string
	Message      string
	AuthorName   string
	AuthorEmail  string
}

type PullRequestInput struct {
	ProjectID     string
	RepositoryID  string
	PullRequestID string
	Number        int
	Title         string
	Description   string
	State         string
	BaseBranch    string
	HeadSHA       string
	MergedByName  string
}

type DraftOutput struct {
	SuggestedTitle string
	Summary        string
	Importance     int
	Reason         string
	DecisionBody   string
	AgentsMd       string
	RawInputJSON   map[string]any
}

var (
	ErrDraftMissing = errors.New("draft not found")
	ErrDraftDenied  = errors.New("draft access denied")
	ErrDraftState   = errors.New("invalid draft state")
)

func NewService(database *sql.DB, cfg config.Config, timelineService *timeline.Service) *Service {
	return &Service{
		db:       database,
		provider: newProvider(cfg),
		timeline: timelineService,
	}
}

func (s *Service) Routes(router chi.Router, authMiddleware func(http.Handler) http.Handler) {
	router.With(authMiddleware).Get("/", s.handleList)
	router.With(authMiddleware).Get("/{draftID}", s.handleGet)
	router.With(authMiddleware).Patch("/{draftID}", s.handleReview)
}

func (s *Service) List(ctx context.Context, userID, projectID string) ([]DraftResponse, error) {
	if !s.canAccessProject(ctx, userID, projectID) {
		return nil, ErrDraftDenied
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, repository_id, commit_id, pull_request_id, source_type, status, suggested_title, summary, importance, reason, decision_body, agents_md, ai_provider, version, reviewed_by_user_id, reviewed_at, raw_input_json
		FROM ai_drafts
		WHERE project_id = $1
		ORDER BY id DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	drafts := make([]DraftResponse, 0)
	for rows.Next() {
		draft, err := scanDraft(rows)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, draft)
	}

	return drafts, rows.Err()
}

func (s *Service) Get(ctx context.Context, userID, projectID, draftID string) (*DraftResponse, error) {
	if !s.canAccessProject(ctx, userID, projectID) {
		return nil, ErrDraftDenied
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, repository_id, commit_id, pull_request_id, source_type, status, suggested_title, summary, importance, reason, decision_body, agents_md, ai_provider, version, reviewed_by_user_id, reviewed_at, raw_input_json
		FROM ai_drafts
		WHERE project_id = $1 AND id = $2
	`, projectID, draftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrDraftMissing
	}

	draft, err := scanDraft(rows)
	if err != nil {
		return nil, err
	}
	return &draft, nil
}

func (s *Service) Review(ctx context.Context, userID, projectID, draftID string, req ReviewRequest) (*DraftResponse, error) {
	if !s.canReviewProject(ctx, userID, projectID) {
		return nil, ErrDraftDenied
	}

	status := strings.TrimSpace(req.Status)
	switch status {
	case "draft", "in_review", "approved", "rejected", "archived":
	default:
		return nil, ErrDraftState
	}

	rows, err := s.db.QueryContext(ctx, `
		UPDATE ai_drafts
		SET status = $1, reviewed_by_user_id = $2, reviewed_at = now()
		WHERE project_id = $3 AND id = $4
		RETURNING id, project_id, repository_id, commit_id, pull_request_id, source_type, status, suggested_title, summary, importance, reason, decision_body, agents_md, ai_provider, version, reviewed_by_user_id, reviewed_at, raw_input_json
	`, status, userID, projectID, draftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrDraftMissing
	}

	draft, err := scanDraft(rows)
	if err != nil {
		return nil, err
	}
	return &draft, nil
}

func (s *Service) GenerateCommitDraft(ctx context.Context, commitID string) error {
	if existing, err := s.hasExistingDraft(ctx, "commit", commitID); err != nil {
		return err
	} else if existing {
		return nil
	}

	var input CommitInput
	if err := s.db.QueryRowContext(ctx, `
		SELECT project_id, repository_id, id, sha, message, author_name, author_email
		FROM commits
		WHERE id = $1
	`, commitID).Scan(&input.ProjectID, &input.RepositoryID, &input.CommitID, &input.SHA, &input.Message, &input.AuthorName, &input.AuthorEmail); err != nil {
		return err
	}

	output, err := s.provider.GenerateCommitDraft(ctx, input)
	if err != nil {
		return err
	}

	if output.Importance == 0 {
		return nil
	}

	rawInput := output.RawInputJSON
	if rawInput == nil {
		rawInput = map[string]any{}
	}
	rawInput["sha"] = input.SHA
	rawInput["message"] = input.Message
	rawInput["author_name"] = input.AuthorName
	rawInput["author_email"] = input.AuthorEmail

	payload, err := json.Marshal(rawInput)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO ai_drafts (
			project_id,
			repository_id,
			commit_id,
			source_type,
			status,
			suggested_title,
			summary,
			importance,
			reason,
			decision_body,
			agents_md,
			raw_input_json,
			ai_provider,
			version
		)
		VALUES ($1, $2, $3, 'commit', 'draft', $4, $5, $6, $7, $8, $9, $10, $11, 1)
	`, input.ProjectID, input.RepositoryID, input.CommitID, output.SuggestedTitle, output.Summary, output.Importance, output.Reason, output.DecisionBody, output.AgentsMd, payload, s.provider.Name())
	if err != nil {
		return err
	}

	if s.timeline != nil {
		var workspaceID string
		_ = s.db.QueryRowContext(ctx, `SELECT workspace_id FROM projects WHERE id = $1`, input.ProjectID).Scan(&workspaceID)
		_ = s.timeline.Record(ctx, workspaceID, input.ProjectID, "", "draft_generated", "commit", input.CommitID, map[string]any{
			"sha":   input.SHA,
			"title": output.SuggestedTitle,
		}, "draft-generated:commit:"+input.CommitID)
	}
	return nil
}

func (s *Service) GeneratePullRequestDraft(ctx context.Context, pullRequestID string) error {
	if existing, err := s.hasExistingDraft(ctx, "pull_request", pullRequestID); err != nil {
		return err
	} else if existing {
		return nil
	}

	var input PullRequestInput
	if err := s.db.QueryRowContext(ctx, `
		SELECT project_id, repository_id, id, number, title, description, state, base_branch, head_sha, merged_by_name
		FROM pull_requests
		WHERE id = $1
	`, pullRequestID).Scan(&input.ProjectID, &input.RepositoryID, &input.PullRequestID, &input.Number, &input.Title, &input.Description, &input.State, &input.BaseBranch, &input.HeadSHA, &input.MergedByName); err != nil {
		return err
	}

	output, err := s.provider.GeneratePullRequestDraft(ctx, input)
	if err != nil {
		return err
	}

	if output.Importance == 0 {
		return nil
	}

	rawInput := output.RawInputJSON
	if rawInput == nil {
		rawInput = map[string]any{}
	}
	rawInput["number"] = input.Number
	rawInput["title"] = input.Title
	rawInput["description"] = input.Description
	rawInput["state"] = input.State
	rawInput["base_branch"] = input.BaseBranch
	rawInput["head_sha"] = input.HeadSHA

	payload, err := json.Marshal(rawInput)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO ai_drafts (
			project_id,
			repository_id,
			pull_request_id,
			source_type,
			status,
			suggested_title,
			summary,
			importance,
			reason,
			decision_body,
			agents_md,
			raw_input_json,
			ai_provider,
			version
		)
		VALUES ($1, $2, $3, 'pull_request', 'draft', $4, $5, $6, $7, $8, $9, $10, $11, 1)
	`, input.ProjectID, input.RepositoryID, input.PullRequestID, output.SuggestedTitle, output.Summary, output.Importance, output.Reason, output.DecisionBody, output.AgentsMd, payload, s.provider.Name())
	if err != nil {
		return err
	}

	if s.timeline != nil {
		var workspaceID string
		_ = s.db.QueryRowContext(ctx, `SELECT workspace_id FROM projects WHERE id = $1`, input.ProjectID).Scan(&workspaceID)
		_ = s.timeline.Record(ctx, workspaceID, input.ProjectID, "", "draft_generated", "pull_request", input.PullRequestID, map[string]any{
			"number": input.Number,
			"title":  output.SuggestedTitle,
		}, "draft-generated:pull-request:"+input.PullRequestID)
	}
	return nil
}

func (s *Service) hasExistingDraft(ctx context.Context, sourceType, sourceID string) (bool, error) {
	var query string
	switch sourceType {
	case "commit":
		query = `SELECT EXISTS(SELECT 1 FROM ai_drafts WHERE source_type = 'commit' AND commit_id = $1)`
	case "pull_request":
		query = `SELECT EXISTS(SELECT 1 FROM ai_drafts WHERE source_type = 'pull_request' AND pull_request_id = $1)`
	default:
		return false, fmt.Errorf("unsupported source type: %s", sourceType)
	}

	var exists bool
	if err := s.db.QueryRowContext(ctx, query, sourceID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
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

func scanDraft(scanner interface{ Scan(...any) error }) (DraftResponse, error) {
	var (
		id               string
		projectID        string
		repositoryID     sql.NullString
		commitID         sql.NullString
		pullRequestID    sql.NullString
		sourceType       string
		status           string
		suggestedTitle   string
		summary          string
		importance       int
		reason           string
		decisionBody     string
		agentsMd         string
		aiProvider       string
		version          int
		reviewedByUserID sql.NullString
		reviewedAt       sql.NullTime
		rawPayload       []byte
	)

	if err := scanner.Scan(&id, &projectID, &repositoryID, &commitID, &pullRequestID, &sourceType, &status, &suggestedTitle, &summary, &importance, &reason, &decisionBody, &agentsMd, &aiProvider, &version, &reviewedByUserID, &reviewedAt, &rawPayload); err != nil {
		return DraftResponse{}, err
	}

	rawInput := map[string]any{}
	if len(rawPayload) > 0 {
		if err := json.Unmarshal(rawPayload, &rawInput); err != nil {
			return DraftResponse{}, err
		}
	}

	response := DraftResponse{
		ID:             id,
		ProjectID:      projectID,
		SourceType:     sourceType,
		Status:         status,
		SuggestedTitle: suggestedTitle,
		Summary:        summary,
		Importance:     importance,
		Reason:         reason,
		DecisionBody:   decisionBody,
		AgentsMd:       agentsMd,
		AIProvider:     aiProvider,
		Version:        version,
		RawInputJSON:   rawInput,
	}
	if repositoryID.Valid {
		response.RepositoryID = repositoryID.String
	}
	if commitID.Valid {
		response.CommitID = commitID.String
	}
	if pullRequestID.Valid {
		response.PullRequestID = pullRequestID.String
	}
	if reviewedByUserID.Valid {
		response.ReviewedByUserID = reviewedByUserID.String
	}
	if reviewedAt.Valid {
		response.ReviewedAt = &reviewedAt.Time
	}
	if commitID.Valid {
		response.CommitID = commitID.String
	}
	if pullRequestID.Valid {
		response.PullRequestID = pullRequestID.String
	}
	if reviewedByUserID.Valid {
		response.ReviewedByUserID = reviewedByUserID.String
	}
	if reviewedAt.Valid {
		value := reviewedAt.Time
		response.ReviewedAt = &value
	}
	return response, nil
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	drafts, err := s.List(r.Context(), userID, projectID)
	if err != nil {
		handleAIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, drafts)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	draftID := chi.URLParam(r, "draftID")
	draft, err := s.Get(r.Context(), userID, projectID, draftID)
	if err != nil {
		handleAIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, draft)
}

func (s *Service) handleReview(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req ReviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	draftID := chi.URLParam(r, "draftID")
	draft, err := s.Review(r.Context(), userID, projectID, draftID, req)
	if err != nil {
		handleAIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, draft)
}

func handleAIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrDraftDenied):
		writeError(w, http.StatusForbidden, "forbidden", "You do not have access to these drafts")
	case errors.Is(err, ErrDraftMissing):
		writeError(w, http.StatusNotFound, "draft_not_found", "Draft not found")
	case errors.Is(err, ErrDraftState):
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "Draft status is invalid")
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
	return nil
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
