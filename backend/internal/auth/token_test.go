package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSignTokenAndParseTokenRoundTrip(t *testing.T) {
	token, err := SignToken("secret", "user-123", "person@example.com", time.Hour)
	if err != nil {
		t.Fatalf("SignToken returned error: %v", err)
	}

	userID, err := ParseToken("secret", token)
	if err != nil {
		t.Fatalf("ParseToken returned error: %v", err)
	}
	if userID != "user-123" {
		t.Fatalf("ParseToken returned wrong subject: got %q want %q", userID, "user-123")
	}
}

func TestParseTokenRejectsWrongSecret(t *testing.T) {
	token, err := SignToken("secret", "user-123", "person@example.com", time.Hour)
	if err != nil {
		t.Fatalf("SignToken returned error: %v", err)
	}

	if _, err := ParseToken("different-secret", token); err == nil {
		t.Fatal("ParseToken returned nil error for wrong secret")
	}
}

func TestParseTokenRejectsUnexpectedIssuer(t *testing.T) {
	claims := jwtClaims{
		Email: "person@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().UTC()),
			Issuer:    "another-issuer",
			Audience:  []string{"knowell-web"},
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("SignedString returned error: %v", err)
	}

	_, err = ParseToken("secret", token)
	if err == nil {
		t.Fatal("ParseToken returned nil error for unexpected issuer")
	}
	if !errors.Is(err, jwt.ErrTokenInvalidIssuer) && err.Error() != "invalid token issuer" {
		t.Fatalf("ParseToken returned unexpected error: %v", err)
	}
}

func TestParseTokenRejectsUnexpectedAudience(t *testing.T) {
	claims := jwtClaims{
		Email: "person@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().UTC()),
			Issuer:    "knowell",
			Audience:  []string{"other-audience"},
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("SignedString returned error: %v", err)
	}

	_, err = ParseToken("secret", token)
	if err == nil {
		t.Fatal("ParseToken returned nil error for unexpected audience")
	}
	if err.Error() != "invalid token audience" {
		t.Fatalf("ParseToken returned unexpected error: %v", err)
	}
}
