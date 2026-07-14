package project

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	gh "github.com/gitXsingh/knowell/backend/internal/github"
)

func (s *Service) ListRepositoryOptions(ctx context.Context, userID, projectID, query string) ([]gh.RepositorySummary, error) {
	if !s.canAccessProject(ctx, userID, projectID) {
		return nil, ErrProjectDenied
	}

	_ = s.ensureSourceEnabled(ctx, projectID, "github_repository")
	if s.github == nil {
		return nil, gh.ErrGitHubNotConfigured
	}

	return s.github.ListRepositories(ctx, userID, query)
}

func (s *Service) GetRepository(ctx context.Context, userID, projectID string) (*RepositoryRecord, error) {
	if !s.canAccessProject(ctx, userID, projectID) {
		return nil, ErrProjectDenied
	}

	return s.getRepositoryRecord(ctx, projectID)
}

func (s *Service) ConnectRepository(ctx context.Context, userID, projectID string, req RepositoryConnectionRequest) (*RepositoryRecord, error) {
	if !s.canEditProject(ctx, userID, projectID) {
		return nil, ErrProjectDenied
	}

	_ = s.ensureSourceEnabled(ctx, projectID, "github_repository")
	if s.github == nil {
		return nil, gh.ErrGitHubNotConfigured
	}

	owner := strings.TrimSpace(req.Owner)
	repoName := strings.TrimSpace(req.RepoName)
	if owner == "" || repoName == "" {
		return nil, ErrProjectInvalid
	}

	repoSummary, err := s.github.GetRepository(ctx, userID, owner, repoName)
	if err != nil {
		return nil, err
	}

	repository, err := s.getRepositoryRecordOrNil(ctx, projectID)
	if err != nil {
		return nil, err
	}

	webhookSecret := ""
	if repository != nil && strings.EqualFold(repository.Owner, repoSummary.Owner) && strings.EqualFold(repository.RepoName, repoSummary.RepoName) {
		var existingSecret string
		if err := s.db.QueryRowContext(ctx, `SELECT webhook_secret FROM repositories WHERE project_id = $1`, projectID).Scan(&existingSecret); err == nil {
			webhookSecret = existingSecret
		}
	}
	if webhookSecret == "" {
		webhookSecret, err = generateWebhookSecret()
		if err != nil {
			return nil, err
		}
	}

	webhookID, err := s.github.EnsureWebhook(ctx, userID, repoSummary.Owner, repoSummary.RepoName, webhookSecret)
	if err != nil {
		return nil, err
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO repositories (
			project_id,
			provider,
			owner,
			repo_name,
			default_branch,
			webhook_id,
			webhook_secret,
			status
		)
		VALUES ($1, 'github', $2, $3, $4, $5, $6, 'connected')
		ON CONFLICT (project_id) DO UPDATE
		SET owner = EXCLUDED.owner,
			repo_name = EXCLUDED.repo_name,
			default_branch = EXCLUDED.default_branch,
			webhook_id = EXCLUDED.webhook_id,
			webhook_secret = EXCLUDED.webhook_secret,
			status = 'connected',
			connected_at = now()
	`, projectID, repoSummary.Owner, repoSummary.RepoName, repoSummary.DefaultBranch, webhookID, webhookSecret); err != nil {
		return nil, err
	}

	return s.getRepositoryRecord(ctx, projectID)
}

func (s *Service) DisconnectRepository(ctx context.Context, userID, projectID string) error {
	if !s.canEditProject(ctx, userID, projectID) {
		return ErrProjectDenied
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM repositories WHERE project_id = $1`, projectID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrRepositoryMissing
	}

	return nil
}

func (s *Service) SyncRepositoryWebhook(ctx context.Context, userID, projectID string) (*RepositoryRecord, error) {
	if !s.canEditProject(ctx, userID, projectID) {
		return nil, ErrProjectDenied
	}
	if s.github == nil {
		return nil, gh.ErrGitHubNotConfigured
	}

	record, err := s.getRepositoryRecord(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var secret string
	if err := s.db.QueryRowContext(ctx, `SELECT webhook_secret FROM repositories WHERE project_id = $1`, projectID).Scan(&secret); err != nil {
		return nil, err
	}
	if secret == "" {
		secret, err = generateWebhookSecret()
		if err != nil {
			return nil, err
		}
	}

	webhookID, err := s.github.EnsureWebhook(ctx, userID, record.Owner, record.RepoName, secret)
	if err != nil {
		return nil, err
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE repositories
		SET webhook_id = $1, webhook_secret = $2, status = 'connected'
		WHERE project_id = $3
	`, webhookID, secret, projectID); err != nil {
		return nil, err
	}

	return s.getRepositoryRecord(ctx, projectID)
}

func (s *Service) getRepositoryRecord(ctx context.Context, projectID string) (*RepositoryRecord, error) {
	record, err := s.getRepositoryRecordOrNil(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrRepositoryMissing
	}
	return record, nil
}

func (s *Service) getRepositoryRecordOrNil(ctx context.Context, projectID string) (*RepositoryRecord, error) {
	var record RepositoryRecord
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, provider, owner, repo_name, default_branch, webhook_id, connected_at, status
		FROM repositories
		WHERE project_id = $1
	`, projectID).Scan(
		&record.ID,
		&record.ProjectID,
		&record.Provider,
		&record.Owner,
		&record.RepoName,
		&record.DefaultBranch,
		&record.WebhookID,
		&record.ConnectedAt,
		&record.Status,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	record.FullName = record.Owner + "/" + record.RepoName
	if s.github != nil {
		record.WebhookURL = s.github.ConfiguredWebhookURL()
	}
	return &record, nil
}

func (s *Service) isSourceEnabled(ctx context.Context, projectID, sourceKey string) bool {
	var enabled bool
	err := s.db.QueryRowContext(ctx, `
		SELECT enabled
		FROM project_sources
		WHERE project_id = $1 AND source_key = $2
	`, projectID, sourceKey).Scan(&enabled)
	if err != nil {
		return false
	}
	return enabled
}

func (s *Service) ensureSourceEnabled(ctx context.Context, projectID, sourceKey string) error {
	if _, ok := allowedSourceKeys[sourceKey]; !ok {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO project_sources (project_id, source_key, enabled, config_json)
		VALUES ($1, $2, true, '{}'::jsonb)
		ON CONFLICT (project_id, source_key) DO UPDATE
		SET enabled = true, updated_at = now()
	`, projectID, sourceKey)
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
