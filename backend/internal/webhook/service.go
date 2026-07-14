package webhook

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/timeline"
)

type AIDraftGenerator interface {
	GenerateCommitDraft(ctx context.Context, commitID string) error
	GeneratePullRequestDraft(ctx context.Context, pullRequestID string) error
}

type Service struct {
	db       *sql.DB
	ai       AIDraftGenerator
	timeline *timeline.Service
}

type EventRecord struct {
	ID               string       `json:"id"`
	RepositoryID     string       `json:"repository_id"`
	GitHubDeliveryID string       `json:"github_delivery_id"`
	EventType        string       `json:"event_type"`
	Action           string       `json:"action"`
	ReceivedAt       time.Time    `json:"received_at"`
	ProcessedAt      sql.NullTime `json:"processed_at"`
	ProcessingStatus string       `json:"processing_status"`
	ErrorMessage     string       `json:"error_message"`
}

type pushPayload struct {
	Ref     string `json:"ref"`
	Commits []struct {
		ID        string   `json:"id"`
		Message   string   `json:"message"`
		Timestamp string   `json:"timestamp"`
		Added     []string `json:"added"`
		Removed   []string `json:"removed"`
		Modified  []string `json:"modified"`
		Author    struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commits"`
}

type pullRequestPayload struct {
	Number      int    `json:"number"`
	Action      string `json:"action"`
	PullRequest struct {
		Title    string `json:"title"`
		Body     string `json:"body"`
		MergedAt string `json:"merged_at"`
		Head     struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		MergedBy struct {
			Login string `json:"login"`
		} `json:"merged_by"`
	} `json:"pull_request"`
}

var (
	ErrEventMissing = errors.New("webhook event not found")
	ErrEventDenied  = errors.New("webhook event access denied")
)

func NewService(database *sql.DB, aiService AIDraftGenerator, timelineService *timeline.Service) *Service {
	return &Service{db: database, ai: aiService, timeline: timelineService}
}

func (s *Service) ProcessRepositoryDelivery(ctx context.Context, repositoryID, deliveryID string) error {
	record, payload, projectID, err := s.loadDelivery(ctx, repositoryID, deliveryID)
	if err != nil {
		return err
	}

	if record.ProcessingStatus == "ignored" {
		return s.markEvent(ctx, record.ID, "ignored", "")
	}

	if err := s.markEvent(ctx, record.ID, "processing", ""); err != nil {
		return err
	}

	var processErr error
	switch record.EventType {
	case "push":
		processErr = s.processPush(ctx, projectID, repositoryID, payload)
	case "pull_request":
		processErr = s.processPullRequest(ctx, projectID, repositoryID, record.Action, payload)
	default:
		processErr = nil
	}

	if processErr != nil {
		_ = s.markEvent(ctx, record.ID, "failed", processErr.Error())
		return processErr
	}

	status := "processed"
	if record.EventType == "pull_request" && record.Action != "opened" && record.Action != "merged" {
		status = "ignored"
	}

	return s.markEvent(ctx, record.ID, status, "")
}

func (s *Service) ProcessPendingProjectEvents(ctx context.Context, projectID string) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT we.repository_id, we.github_delivery_id
		FROM webhook_events we
		JOIN repositories r ON r.id = we.repository_id
		WHERE r.project_id = $1
		  AND we.processing_status IN ('received', 'failed')
		ORDER BY we.received_at ASC
	`, projectID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var repositoryID, deliveryID string
		if err := rows.Scan(&repositoryID, &deliveryID); err != nil {
			return err
		}
		if err := s.ProcessRepositoryDelivery(ctx, repositoryID, deliveryID); err != nil {
			return err
		}
	}

	return rows.Err()
}

func (s *Service) ListProjectEvents(ctx context.Context, userID, projectID string) ([]EventRecord, error) {
	if !s.canAccessProject(ctx, userID, projectID) {
		return nil, ErrEventDenied
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT we.id, we.repository_id, we.github_delivery_id, we.event_type, we.action, we.received_at, we.processed_at, we.processing_status, we.error_message
		FROM webhook_events we
		JOIN repositories r ON r.id = we.repository_id
		WHERE r.project_id = $1
		ORDER BY we.received_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]EventRecord, 0)
	for rows.Next() {
		var record EventRecord
		if err := rows.Scan(&record.ID, &record.RepositoryID, &record.GitHubDeliveryID, &record.EventType, &record.Action, &record.ReceivedAt, &record.ProcessedAt, &record.ProcessingStatus, &record.ErrorMessage); err != nil {
			return nil, err
		}
		events = append(events, record)
	}

	return events, rows.Err()
}

func (s *Service) loadDelivery(ctx context.Context, repositoryID, deliveryID string) (*EventRecord, []byte, string, error) {
	var (
		record    EventRecord
		projectID string
		payload   []byte
	)

	if err := s.db.QueryRowContext(ctx, `
		SELECT we.id, we.repository_id, we.github_delivery_id, we.event_type, we.action, we.received_at, we.processed_at, we.processing_status, we.error_message, we.payload_json, r.project_id
		FROM webhook_events we
		JOIN repositories r ON r.id = we.repository_id
		WHERE we.repository_id = $1 AND we.github_delivery_id = $2
	`, repositoryID, deliveryID).Scan(
		&record.ID,
		&record.RepositoryID,
		&record.GitHubDeliveryID,
		&record.EventType,
		&record.Action,
		&record.ReceivedAt,
		&record.ProcessedAt,
		&record.ProcessingStatus,
		&record.ErrorMessage,
		&payload,
		&projectID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, "", ErrEventMissing
		}
		return nil, nil, "", err
	}

	return &record, payload, projectID, nil
}

func (s *Service) processPush(ctx context.Context, projectID, repositoryID string, payload []byte) error {
	var event pushPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	for _, commit := range event.Commits {
		committedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(commit.Timestamp))
		if err != nil {
			committedAt = time.Now().UTC()
		}

		diffSummaryPayload, err := json.Marshal(map[string]any{
			"added":    commit.Added,
			"removed":  commit.Removed,
			"modified": commit.Modified,
		})
		if err != nil {
			return err
		}

		var commitID string
		if err := s.db.QueryRowContext(ctx, `
			INSERT INTO commits (
				project_id,
				repository_id,
				sha,
				message,
				author_name,
				author_email,
				committed_at,
				diff_summary_json,
				raw_payload_json
			)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (repository_id, sha) DO UPDATE
			SET message = EXCLUDED.message,
				author_name = EXCLUDED.author_name,
				author_email = EXCLUDED.author_email,
				committed_at = EXCLUDED.committed_at,
				diff_summary_json = EXCLUDED.diff_summary_json,
				raw_payload_json = EXCLUDED.raw_payload_json
			RETURNING id
		`, projectID, repositoryID, strings.TrimSpace(commit.ID), strings.TrimSpace(commit.Message), strings.TrimSpace(commit.Author.Name), strings.TrimSpace(commit.Author.Email), committedAt, diffSummaryPayload, payload).Scan(&commitID); err != nil {
			return err
		}

		if s.timeline != nil {
			var workspaceID string
			_ = s.db.QueryRowContext(ctx, `SELECT workspace_id FROM projects WHERE id = $1::uuid`, projectID).Scan(&workspaceID)
			_ = s.timeline.Record(ctx, workspaceID, projectID, "", "commit", "commit", commitID, map[string]any{
				"sha":     strings.TrimSpace(commit.ID),
				"message": strings.TrimSpace(commit.Message),
			}, "commit:"+repositoryID+":"+strings.TrimSpace(commit.ID))
		}

		if s.ai != nil {
			if err := s.ai.GenerateCommitDraft(ctx, commitID); err != nil {
				log.Printf("[webhook] ai commit draft generation skipped (non-blocking): %v", err)
			}
		}
	}

	return nil
}

func (s *Service) processPullRequest(ctx context.Context, projectID, repositoryID, action string, payload []byte) error {
	if action != "opened" && action != "merged" {
		return nil
	}

	var event pullRequestPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	state := "opened"
	var mergedAt sql.NullTime
	mergedByName := ""
	if action == "merged" {
		state = "merged"
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(event.PullRequest.MergedAt)); err == nil {
			mergedAt = sql.NullTime{Time: parsed, Valid: true}
		}
		mergedByName = strings.TrimSpace(event.PullRequest.MergedBy.Login)
	}

	var pullRequestID string
	if err := s.db.QueryRowContext(ctx, `
		INSERT INTO pull_requests (
			project_id,
			repository_id,
			number,
			title,
			description,
			state,
			merged_at,
			head_sha,
			base_branch,
			merged_by_name,
			raw_payload_json
		)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (repository_id, number) DO UPDATE
		SET title = EXCLUDED.title,
			description = EXCLUDED.description,
			state = EXCLUDED.state,
			merged_at = EXCLUDED.merged_at,
			head_sha = EXCLUDED.head_sha,
			base_branch = EXCLUDED.base_branch,
			merged_by_name = EXCLUDED.merged_by_name,
			raw_payload_json = EXCLUDED.raw_payload_json
		RETURNING id
	`, projectID, repositoryID, event.Number, strings.TrimSpace(event.PullRequest.Title), strings.TrimSpace(event.PullRequest.Body), state, mergedAt, strings.TrimSpace(event.PullRequest.Head.SHA), strings.TrimSpace(event.PullRequest.Base.Ref), mergedByName, payload).Scan(&pullRequestID); err != nil {
		return err
	}

	if s.timeline != nil {
		var workspaceID string
		_ = s.db.QueryRowContext(ctx, `SELECT workspace_id FROM projects WHERE id = $1::uuid`, projectID).Scan(&workspaceID)
		eventType := "pr_created"
		if action == "merged" {
			eventType = "pr_merged"
		}
		_ = s.timeline.Record(ctx, workspaceID, projectID, "", eventType, "pull_request", pullRequestID, map[string]any{
			"number": event.Number,
			"title":  strings.TrimSpace(event.PullRequest.Title),
			"state":  state,
		}, eventType+":"+repositoryID+":"+strings.TrimSpace(event.PullRequest.Head.SHA))
	}

	if s.ai != nil {
		if err := s.ai.GeneratePullRequestDraft(ctx, pullRequestID); err != nil {
			log.Printf("[webhook] ai pr draft generation skipped (non-blocking): %v", err)
		}
	}

	return nil
}

func (s *Service) markEvent(ctx context.Context, eventID, status, errorMessage string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE webhook_events
		SET processing_status = $1::webhook_processing_status,
			error_message = $2,
			processed_at = CASE WHEN $1::text IN ('processed', 'failed', 'ignored') THEN now() ELSE processed_at END
		WHERE id = $3::uuid
	`, status, strings.TrimSpace(errorMessage), eventID)
	return err
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


