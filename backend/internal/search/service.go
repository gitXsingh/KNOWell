package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/auth"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	db *sql.DB
}

type Result struct {
	ID           string     `json:"id"`
	ProjectID    string     `json:"project_id"`
	RepositoryID string     `json:"repository_id,omitempty"`
	Title        string     `json:"title"`
	Summary      string     `json:"summary"`
	Importance   int        `json:"importance"`
	Status       string     `json:"status"`
	ApprovedAt   *time.Time `json:"approved_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func NewService(database *sql.DB) *Service {
	return &Service{db: database}
}

func (s *Service) Routes(router chi.Router, authMiddleware func(http.Handler) http.Handler) {
	router.With(authMiddleware).Get("/", s.handleSearch)
}

func (s *Service) Search(ctx context.Context, userID, projectID, keyword, repositoryID, dateFrom, dateTo, sort string) ([]Result, error) {
	if !s.canAccessProject(ctx, userID, projectID) {
		return nil, fmt.Errorf("forbidden")
	}

	var (
		args  []any
		where []string
	)
	args = append(args, projectID)
	where = append(where, "project_id = $1")
	where = append(where, "status = 'active'")

	if strings.TrimSpace(keyword) != "" {
		args = append(args, strings.TrimSpace(keyword))
		where = append(where, fmt.Sprintf("search_vector @@ plainto_tsquery('english', $%d)", len(args)))
	}
	if strings.TrimSpace(repositoryID) != "" {
		args = append(args, strings.TrimSpace(repositoryID))
		where = append(where, fmt.Sprintf("repository_id = $%d", len(args)))
	}
	if strings.TrimSpace(dateFrom) != "" {
		args = append(args, strings.TrimSpace(dateFrom))
		where = append(where, fmt.Sprintf("created_at >= $%d::timestamptz", len(args)))
	}
	if strings.TrimSpace(dateTo) != "" {
		args = append(args, strings.TrimSpace(dateTo))
		where = append(where, fmt.Sprintf("created_at <= $%d::timestamptz", len(args)))
	}

	orderBy := "created_at DESC"
	switch strings.TrimSpace(sort) {
	case "oldest":
		orderBy = "created_at ASC"
	case "importance":
		orderBy = "importance DESC, created_at DESC"
	case "repository":
		orderBy = "repository_id ASC NULLS LAST, created_at DESC"
	}

	query := `
		SELECT id, project_id, repository_id, title, summary, importance, status, approved_at, created_at
		FROM knowledge_items
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY ` + orderBy

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]Result, 0)
	for rows.Next() {
		var (
			result       Result
			repositoryID sql.NullString
			approvedAt   sql.NullTime
		)
		if err := rows.Scan(&result.ID, &result.ProjectID, &repositoryID, &result.Title, &result.Summary, &result.Importance, &result.Status, &approvedAt, &result.CreatedAt); err != nil {
			return nil, err
		}
		if repositoryID.Valid {
			result.RepositoryID = repositoryID.String
		}
		if approvedAt.Valid {
			value := approvedAt.Time
			result.ApprovedAt = &value
		}
		results = append(results, result)
	}
	return results, rows.Err()
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

func (s *Service) handleSearch(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	projectID := chi.URLParam(r, "projectID")
	results, err := s.Search(
		r.Context(),
		userID,
		projectID,
		r.URL.Query().Get("keyword"),
		r.URL.Query().Get("repository_id"),
		r.URL.Query().Get("date_from"),
		r.URL.Query().Get("date_to"),
		r.URL.Query().Get("sort"),
	)
	if err != nil {
		writeError(w, http.StatusForbidden, "forbidden", "You do not have access to this search")
		return
	}
	writeJSON(w, http.StatusOK, results)
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
