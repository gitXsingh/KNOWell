package github

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (s *Service) signState(state oauthState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}

	payloadEncoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	_, _ = mac.Write([]byte(payloadEncoded))
	signatureEncoded := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return payloadEncoded + "." + signatureEncoded, nil
}

func (s *Service) parseState(raw string) (*oauthState, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return nil, ErrGitHubStateInvalid
	}

	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	_, _ = mac.Write([]byte(parts[0]))
	expectedSignature := mac.Sum(nil)

	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, expectedSignature) {
		return nil, ErrGitHubStateInvalid
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrGitHubStateInvalid
	}

	var state oauthState
	if err := json.Unmarshal(payload, &state); err != nil {
		return nil, ErrGitHubStateInvalid
	}
	if state.UserID == "" || state.RedirectTo == "" || state.ExpiresAt == 0 {
		return nil, ErrGitHubStateInvalid
	}
	if time.Now().Unix() > state.ExpiresAt {
		return nil, ErrGitHubStateInvalid
	}

	return &state, nil
}

func (s *Service) exchangeCode(ctx context.Context, code, state string) (string, string, error) {
	form := url.Values{}
	form.Set("client_id", s.cfg.GitHubClientID)
	form.Set("client_secret", s.cfg.GitHubClientSecret)
	form.Set("code", strings.TrimSpace(code))
	form.Set("redirect_uri", s.cfg.GitHubRedirectURL)
	form.Set("state", state)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("github token exchange failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var payload tokenResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", fmt.Errorf("github token response is invalid")
	}
	if payload.Error != "" {
		return "", "", fmt.Errorf("github token exchange was rejected")
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", "", fmt.Errorf("github access token is missing")
	}

	return payload.AccessToken, payload.Scope, nil
}

func (s *Service) fetchGitHubUserID(ctx context.Context, accessToken string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubCurrentUserURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("github user lookup failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("github user lookup failed")
	}

	var payload currentUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, fmt.Errorf("github user response is invalid")
	}
	if payload.ID == 0 {
		return 0, fmt.Errorf("github user id is missing")
	}

	return payload.ID, nil
}

func randomNonce() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}
