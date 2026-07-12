package github

import (
	"testing"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/common/config"
)

func TestSignStateAndParseStateRoundTrip(t *testing.T) {
	service := &Service{cfg: config.Config{JWTSecret: "secret"}}
	state := oauthState{
		UserID:     "user-123",
		RedirectTo: "http://localhost:3000/github/callback",
		ExpiresAt:  time.Now().Add(time.Minute).Unix(),
		Nonce:      "nonce",
	}

	signed, err := service.signState(state)
	if err != nil {
		t.Fatalf("signState returned error: %v", err)
	}

	parsed, err := service.parseState(signed)
	if err != nil {
		t.Fatalf("parseState returned error: %v", err)
	}
	if parsed.UserID != state.UserID || parsed.RedirectTo != state.RedirectTo || parsed.Nonce != state.Nonce {
		t.Fatalf("parseState returned wrong state: got %#v want %#v", parsed, state)
	}
}

func TestParseStateRejectsExpiredState(t *testing.T) {
	service := &Service{cfg: config.Config{JWTSecret: "secret"}}
	state := oauthState{
		UserID:     "user-123",
		RedirectTo: "http://localhost:3000/github/callback",
		ExpiresAt:  time.Now().Add(-time.Minute).Unix(),
		Nonce:      "nonce",
	}

	signed, err := service.signState(state)
	if err != nil {
		t.Fatalf("signState returned error: %v", err)
	}

	if _, err := service.parseState(signed); err != ErrGitHubStateInvalid {
		t.Fatalf("parseState returned wrong error: got %v want %v", err, ErrGitHubStateInvalid)
	}
}

func TestParseStateRejectsTamperedState(t *testing.T) {
	service := &Service{cfg: config.Config{JWTSecret: "secret"}}
	state := oauthState{
		UserID:     "user-123",
		RedirectTo: "http://localhost:3000/github/callback",
		ExpiresAt:  time.Now().Add(time.Minute).Unix(),
		Nonce:      "nonce",
	}

	signed, err := service.signState(state)
	if err != nil {
		t.Fatalf("signState returned error: %v", err)
	}

	tampered := signed[:len(signed)-1] + "x"
	if _, err := service.parseState(tampered); err != ErrGitHubStateInvalid {
		t.Fatalf("parseState returned wrong error: got %v want %v", err, ErrGitHubStateInvalid)
	}
}

func TestSplitScopes(t *testing.T) {
	commaSeparated := splitScopes("repo, read:user, admin:repo_hook")
	if len(commaSeparated) != 3 {
		t.Fatalf("splitScopes returned wrong comma-separated count: got %d want 3", len(commaSeparated))
	}

	spaceSeparated := splitScopes("repo read:user admin:repo_hook")
	if len(spaceSeparated) != 3 {
		t.Fatalf("splitScopes returned wrong space-separated count: got %d want 3", len(spaceSeparated))
	}
}

func TestValidateRedirectURL(t *testing.T) {
	if err := validateRedirectURL("http://localhost:3000/github/callback"); err != nil {
		t.Fatalf("validateRedirectURL returned error for valid URL: %v", err)
	}

	if err := validateRedirectURL("ftp://localhost/callback"); err == nil {
		t.Fatal("validateRedirectURL returned nil error for invalid scheme")
	}
}
