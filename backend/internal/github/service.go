package github

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/auth"
	"github.com/gitXsingh/knowell/backend/internal/common/config"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	githubAuthorizeURL   = "https://github.com/login/oauth/authorize"
	githubTokenURL       = "https://github.com/login/oauth/access_token"
	githubCurrentUserURL = "https://api.github.com/user"
	githubUserReposURL   = "https://api.github.com/user/repos"
)

type Service struct {
	db         *sql.DB
	cfg        config.Config
	httpClient *http.Client
	processor  WebhookProcessor
}

type WebhookProcessor interface {
	ProcessRepositoryDelivery(ctx context.Context, repositoryID, deliveryID string) error
}

type ConnectResponse struct {
	AuthorizationURL string `json:"authorization_url"`
}

type AccountStatusResponse struct {
	Configured   bool       `json:"configured"`
	Connected    bool       `json:"connected"`
	GitHubUserID int64      `json:"github_user_id,omitempty"`
	TokenScopes  []string   `json:"token_scopes"`
	ConnectedAt  *time.Time `json:"connected_at,omitempty"`
}

type accountRecord struct {
	GitHubUserID int64
	TokenScopes  string
	ConnectedAt  time.Time
}

type oauthState struct {
	UserID     string `json:"user_id"`
	RedirectTo string `json:"redirect_to"`
	ExpiresAt  int64  `json:"expires_at"`
	Nonce      string `json:"nonce"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

type currentUserResponse struct {
	ID int64 `json:"id"`
}

type RepositorySummary struct {
	Owner         string `json:"owner"`
	RepoName      string `json:"repo_name"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

type repositoryAPIResponse struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type webhookPayload struct {
	ID     int64  `json:"id"`
	Active bool   `json:"active"`
	Name   string `json:"name"`
	Config struct {
		URL string `json:"url"`
	} `json:"config"`
}

type webhookCreateRequest struct {
	Name   string            `json:"name"`
	Active bool              `json:"active"`
	Events []string          `json:"events"`
	Config map[string]string `json:"config"`
}

var (
	ErrGitHubNotConfigured        = errors.New("github oauth is not configured")
	ErrGitHubStateInvalid         = errors.New("github oauth state is invalid")
	ErrGitHubAccountLinked        = errors.New("github account is already linked")
	ErrGitHubAccountMissing       = errors.New("github account not found")
	ErrGitHubRepositoryMissing    = errors.New("github repository not found")
	ErrGitHubWebhookConfigMissing = errors.New("github webhook url is not configured")
)

func NewService(database *sql.DB, cfg config.Config) *Service {
	return &Service{
		db:  database,
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *Service) SetWebhookProcessor(processor WebhookProcessor) {
	s.processor = processor
}

func (s *Service) Routes(router chi.Router, authMiddleware func(http.Handler) http.Handler) {
	router.With(authMiddleware).Get("/connect", s.handleConnect)
	router.Get("/callback", s.handleCallback)
	router.With(authMiddleware).Get("/account", s.handleAccountStatus)
	router.With(authMiddleware).Delete("/account", s.handleDisconnect)
	router.Post("/webhook", s.handleWebhook)
}

func (s *Service) ConnectURL(userID, redirectTo string) (string, error) {
	if !s.isConfigured() {
		return "", ErrGitHubNotConfigured
	}

	if strings.TrimSpace(redirectTo) == "" {
		redirectTo = s.cfg.GitHubFrontendCallbackURL
	}

	if err := validateRedirectURL(redirectTo); err != nil {
		return "", err
	}

	stateValue, err := s.signState(oauthState{
		UserID:     userID,
		RedirectTo: redirectTo,
		ExpiresAt:  time.Now().Add(s.cfg.GitHubStateTTL).Unix(),
		Nonce:      randomNonce(),
	})
	if err != nil {
		return "", err
	}

	query := url.Values{}
	query.Set("client_id", s.cfg.GitHubClientID)
	query.Set("redirect_uri", s.cfg.GitHubRedirectURL)
	query.Set("scope", strings.TrimSpace(s.cfg.GitHubScopes))
	query.Set("state", stateValue)

	return githubAuthorizeURL + "?" + query.Encode(), nil
}

func (s *Service) HandleCallback(ctx context.Context, code, stateValue string) (string, error) {
	if !s.isConfigured() {
		return s.callbackRedirectURL(s.cfg.GitHubFrontendCallbackURL, "error", "GitHub OAuth is not configured"), ErrGitHubNotConfigured
	}

	state, err := s.parseState(stateValue)
	if err != nil {
		return s.callbackRedirectURL(s.cfg.GitHubFrontendCallbackURL, "error", "Invalid GitHub OAuth state"), ErrGitHubStateInvalid
	}

	token, scopes, err := s.exchangeCode(ctx, code, stateValue)
	if err != nil {
		return s.callbackRedirectURL(state.RedirectTo, "error", err.Error()), err
	}

	githubUserID, err := s.fetchGitHubUserID(ctx, token)
	if err != nil {
		return s.callbackRedirectURL(state.RedirectTo, "error", err.Error()), err
	}

	if err := s.upsertAccount(ctx, state.UserID, githubUserID, token, scopes); err != nil {
		if errors.Is(err, ErrGitHubAccountLinked) {
			return s.callbackRedirectURL(state.RedirectTo, "error", "GitHub account is already linked"), err
		}
		return s.callbackRedirectURL(state.RedirectTo, "error", "Failed to save GitHub account"), err
	}

	return s.callbackRedirectURL(state.RedirectTo, "success", "GitHub account connected"), nil
}

func (s *Service) AccountStatus(ctx context.Context, userID string) (*AccountStatusResponse, error) {
	record, err := s.getAccount(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &AccountStatusResponse{
				Configured:  s.isConfigured(),
				Connected:   false,
				TokenScopes: []string{},
			}, nil
		}
		return nil, err
	}

	connectedAt := record.ConnectedAt
	return &AccountStatusResponse{
		Configured:   s.isConfigured(),
		Connected:    true,
		GitHubUserID: record.GitHubUserID,
		TokenScopes:  splitScopes(record.TokenScopes),
		ConnectedAt:  &connectedAt,
	}, nil
}

func (s *Service) Disconnect(ctx context.Context, userID string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM github_accounts
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return err
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrGitHubAccountMissing
	}

	return nil
}

func (s *Service) getAccount(ctx context.Context, userID string) (*accountRecord, error) {
	var record accountRecord
	if err := s.db.QueryRowContext(ctx, `
		SELECT github_user_id, token_scopes, connected_at
		FROM github_accounts
		WHERE user_id = $1
	`, userID).Scan(&record.GitHubUserID, &record.TokenScopes, &record.ConnectedAt); err != nil {
		return nil, err
	}

	return &record, nil
}

func (s *Service) upsertAccount(ctx context.Context, userID string, githubUserID int64, accessToken, scopes string) error {
	encryptedToken, err := encryptToken(s.cfg.GitHubTokenSecret, accessToken)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO github_accounts (
			user_id,
			github_user_id,
			access_token_encrypted,
			token_scopes,
			connected_at,
			revoked_at
		)
		VALUES ($1, $2, $3, $4, now(), NULL)
		ON CONFLICT (user_id) DO UPDATE
		SET github_user_id = EXCLUDED.github_user_id,
			access_token_encrypted = EXCLUDED.access_token_encrypted,
			token_scopes = EXCLUDED.token_scopes,
			connected_at = now(),
			revoked_at = NULL
	`, userID, githubUserID, encryptedToken, strings.TrimSpace(scopes))
	if err != nil {
		if isUniqueViolation(err) {
			return ErrGitHubAccountLinked
		}
		return err
	}

	return nil
}

func (s *Service) callbackRedirectURL(baseURL, status, message string) string {
	target, err := url.Parse(baseURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		target, _ = url.Parse(s.cfg.GitHubFrontendCallbackURL)
	}

	values := target.Query()
	values.Set("status", status)
	values.Set("message", message)
	target.RawQuery = values.Encode()

	return target.String()
}

func (s *Service) isConfigured() bool {
	return strings.TrimSpace(s.cfg.GitHubClientID) != "" &&
		strings.TrimSpace(s.cfg.GitHubClientSecret) != "" &&
		strings.TrimSpace(s.cfg.GitHubRedirectURL) != ""
}

func (s *Service) ConfiguredWebhookURL() string {
	return strings.TrimSpace(s.cfg.GitHubWebhookURL)
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

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func splitScopes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}

	if strings.Contains(raw, ",") {
		parts := strings.Split(raw, ",")
		scopes := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				scopes = append(scopes, part)
			}
		}
		return scopes
	}

	return strings.Fields(raw)
}

func validateRedirectURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("redirect_to is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("redirect_to must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("redirect_to host is required")
	}
	return nil
}

func currentUserID(r *http.Request) (string, bool) {
	return auth.UserIDFromContext(r.Context())
}
