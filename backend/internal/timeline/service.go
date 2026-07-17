package timeline

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/auth"
	"github.com/gitXsingh/knowell/backend/internal/common/pagination"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	db *sql.DB
}

type Event struct {
	ID          string         `json:"id"`
	WorkspaceID sql.NullString `json:"-"`
	ProjectID   sql.NullString `json:"-"`
	ActorID     sql.NullString `json:"-"`
	EventType   string         `json:"event_type"`
	EntityType  string         `json:"entity_type"`
	EntityID    string         `json:"entity_id"`
	Payload     map[string]any `json:"payload"`
	CreatedAt   time.Time      `json:"created_at"`
}

type EventResponse struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id,omitempty"`
	ProjectID   string         `json:"project_id,omitempty"`
	ActorID     string         `json:"actor_id,omitempty"`
	EventType   string         `json:"event_type"`
	EntityType  string         `json:"entity_type"`
	EntityID    string         `json:"entity_id"`
	Payload     map[string]any `json:"payload"`
	CreatedAt   time.Time      `json:"created_at"`
}

func NewService(database *sql.DB) *Service {
	return &Service{db: database}
}

func (s *Service) Routes(router chi.Router, authMiddleware func(http.Handler) http.Handler) {
	router.With(authMiddleware).Get("/", s.handleList)
}

func (s *Service) Record(ctx context.Context, workspaceID, projectID, actorID, eventType, entityType, entityID string, payload map[string]any, dedupeKey string) error {
	if payload == nil {
		payload = map[string]any{}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO activity_logs (
			workspace_id,
			project_id,
			actor_id,
			event_type,
			entity_type,
			entity_id,
			payload_json,
			dedupe_key
		)
		VALUES ($1::uuid, $2::uuid, NULLIF($3, '')::uuid, $4, $5, $6::uuid, $7, $8)
		ON CONFLICT (dedupe_key) DO NOTHING
	`, nullIfBlank(workspaceID), nullIfBlank(projectID), strings.TrimSpace(actorID), eventType, entityType, entityID, body, strings.TrimSpace(dedupeKey))
	return err
}

func (s *Service) ListProjectEvents(ctx context.Context, userID, projectID string, p pagination.Params) ([]EventResponse, error) {
	if !s.canAccessProject(ctx, userID, projectID) {
		return nil, fmt.Errorf("forbidden")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workspace_id, project_id, actor_id, event_type, entity_type, entity_id, payload_json, created_at
		FROM activity_logs
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, projectID, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]EventResponse, 0, p.Limit)
	for rows.Next() {
		var (
			record     Event
			rawPayload []byte
		)
		if err := rows.Scan(&record.ID, &record.WorkspaceID, &record.ProjectID, &record.ActorID, &record.EventType, &record.EntityType, &record.EntityID, &rawPayload, &record.CreatedAt); err != nil {
			return nil, err
		}
		payload := map[string]any{}
		if len(rawPayload) > 0 {
			if err := json.Unmarshal(rawPayload, &payload); err != nil {
				return nil, err
			}
		}
		event := EventResponse{
			ID:         record.ID,
			EventType:  record.EventType,
			EntityType: record.EntityType,
			EntityID:   record.EntityID,
			Payload:    payload,
			CreatedAt:  record.CreatedAt,
		}
		if record.WorkspaceID.Valid {
			event.WorkspaceID = record.WorkspaceID.String
		}
		if record.ProjectID.Valid {
			event.ProjectID = record.ProjectID.String
		}
		if record.ActorID.Valid {
			event.ActorID = record.ActorID.String
		}
		events = append(events, event)
	}

	return events, rows.Err()
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

func nullIfBlank(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	projectID := chi.URLParam(r, "projectID")
	events, err := s.ListProjectEvents(r.Context(), userID, projectID, pagination.FromRequest(r))
	if err != nil {
		writeError(w, http.StatusForbidden, "forbidden", "You do not have access to this timeline")
		return
	}

	writeJSON(w, http.StatusOK, events)
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
