package github

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (s *Service) ListRepositories(ctx context.Context, userID, query string) ([]RepositorySummary, error) {
	accessToken, err := s.accessToken(ctx, userID)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("sort", "updated")
	params.Set("per_page", "100")
	params.Set("affiliation", "owner,collaborator,organization_member")

	endpoint := githubUserReposURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	payload, err := s.doGitHubJSON(req, accessToken)
	if err != nil {
		return nil, err
	}

	var repos []repositoryAPIResponse
	if err := json.Unmarshal(payload, &repos); err != nil {
		return nil, fmt.Errorf("github repositories response is invalid")
	}

	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	results := make([]RepositorySummary, 0, len(repos))
	for _, repo := range repos {
		summary := mapRepositorySummary(repo)
		if normalizedQuery != "" {
			candidate := strings.ToLower(summary.FullName + " " + summary.Owner + " " + summary.RepoName)
			if !strings.Contains(candidate, normalizedQuery) {
				continue
			}
		}
		results = append(results, summary)
	}

	return results, nil
}

func (s *Service) GetRepository(ctx context.Context, userID, owner, repo string) (*RepositorySummary, error) {
	accessToken, err := s.accessToken(ctx, userID)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubRepositoryURL(owner, repo), nil)
	if err != nil {
		return nil, err
	}

	payload, err := s.doGitHubJSON(req, accessToken)
	if err != nil {
		return nil, err
	}

	var response repositoryAPIResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("github repository response is invalid")
	}

	if strings.TrimSpace(response.Name) == "" || strings.TrimSpace(response.Owner.Login) == "" {
		return nil, ErrGitHubRepositoryMissing
	}

	summary := mapRepositorySummary(response)
	return &summary, nil
}

func (s *Service) EnsureWebhook(ctx context.Context, userID, owner, repo, secret string) (string, error) {
	if strings.TrimSpace(s.cfg.GitHubWebhookURL) == "" {
		return "", ErrGitHubWebhookConfigMissing
	}

	accessToken, err := s.accessToken(ctx, userID)
	if err != nil {
		return "", err
	}

	hooks, err := s.listHooks(ctx, accessToken, owner, repo)
	if err != nil {
		return "", err
	}

	callbackURL := strings.TrimSpace(s.cfg.GitHubWebhookURL)
	for _, hook := range hooks {
		if hook.Name != "web" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(hook.Config.URL), callbackURL) {
			if err := s.updateHook(ctx, accessToken, owner, repo, hook.ID, secret); err != nil {
				return "", err
			}
			return fmt.Sprintf("%d", hook.ID), nil
		}
	}

	hookID, err := s.createHook(ctx, accessToken, owner, repo, secret)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d", hookID), nil
}

func (s *Service) accessToken(ctx context.Context, userID string) (string, error) {
	var encrypted string
	if err := s.db.QueryRowContext(ctx, `
		SELECT access_token_encrypted
		FROM github_accounts
		WHERE user_id = $1
	`, userID).Scan(&encrypted); err != nil {
		if err == sql.ErrNoRows {
			return "", ErrGitHubAccountMissing
		}
		return "", err
	}

	return decryptToken(s.cfg.GitHubTokenSecret, encrypted)
}

func (s *Service) listHooks(ctx context.Context, accessToken, owner, repo string) ([]webhookPayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubRepositoryURL(owner, repo)+"/hooks", nil)
	if err != nil {
		return nil, err
	}

	payload, err := s.doGitHubJSON(req, accessToken)
	if err != nil {
		return nil, err
	}

	var hooks []webhookPayload
	if err := json.Unmarshal(payload, &hooks); err != nil {
		return nil, fmt.Errorf("github hook response is invalid")
	}

	return hooks, nil
}

func (s *Service) createHook(ctx context.Context, accessToken, owner, repo, secret string) (int64, error) {
	body, err := json.Marshal(s.webhookRequest(secret))
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubRepositoryURL(owner, repo)+"/hooks", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	payload, err := s.doGitHubJSON(req, accessToken)
	if err != nil {
		return 0, err
	}

	var hook webhookPayload
	if err := json.Unmarshal(payload, &hook); err != nil {
		return 0, fmt.Errorf("github created hook response is invalid")
	}

	return hook.ID, nil
}

func (s *Service) updateHook(ctx context.Context, accessToken, owner, repo string, hookID int64, secret string) error {
	body, err := json.Marshal(s.webhookRequest(secret))
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, fmt.Sprintf("%s/hooks/%d", githubRepositoryURL(owner, repo), hookID), bytes.NewReader(body))
	if err != nil {
		return err
	}

	_, err = s.doGitHubJSON(req, accessToken)
	return err
}

func (s *Service) webhookRequest(secret string) webhookCreateRequest {
	return webhookCreateRequest{
		Name:   "web",
		Active: true,
		Events: []string{"push", "pull_request"},
		Config: map[string]string{
			"url":          strings.TrimSpace(s.cfg.GitHubWebhookURL),
			"content_type": "json",
			"insecure_ssl": "0",
			"secret":       secret,
		},
	}
}

func (s *Service) doGitHubJSON(req *http.Request, accessToken string) ([]byte, error) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github request failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrGitHubRepositoryMissing
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("github request failed")
	}

	return body, nil
}

func githubRepositoryURL(owner, repo string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s", url.PathEscape(strings.TrimSpace(owner)), url.PathEscape(strings.TrimSpace(repo)))
}

func mapRepositorySummary(repo repositoryAPIResponse) RepositorySummary {
	return RepositorySummary{
		Owner:         strings.TrimSpace(repo.Owner.Login),
		RepoName:      strings.TrimSpace(repo.Name),
		FullName:      strings.TrimSpace(repo.FullName),
		DefaultBranch: strings.TrimSpace(repo.DefaultBranch),
		Private:       repo.Private,
	}
}
