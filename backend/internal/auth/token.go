package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type jwtClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type contextKey string

const userIDContextKey contextKey = "authUserID"

const sessionCookieName = "knowell_session"

func SignToken(secret, userID, email string, ttl time.Duration) (string, error) {
	claims := jwtClaims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(ttl)),
			NotBefore: jwt.NewNumericDate(time.Now().UTC()),
			Issuer:    "knowell",
			Audience:  []string{"knowell-web"},
			ID:        strconv.FormatInt(time.Now().UnixNano(), 10),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(secret, rawToken string) (string, error) {
	token, err := jwt.ParseWithClaims(rawToken, &jwtClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token")
	}
	if claims.Issuer != "knowell" {
		return "", errors.New("invalid token issuer")
	}
	if len(claims.Audience) == 0 || claims.Audience[0] != "knowell-web" {
		return "", errors.New("invalid token audience")
	}

	return claims.Subject, nil
}

func (s *Service) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}

		userID, err := ParseToken(s.cfg.JWTSecret, cookie.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return s.authMiddleware(next)
}

func userIDFromContext(ctx context.Context) (string, bool) {
	value := ctx.Value(userIDContextKey)
	userID, ok := value.(string)
	return userID, ok
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	return userIDFromContext(ctx)
}

func (s *Service) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.Environment == "production",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.cfg.JWTAccessTTL.Seconds()),
	})
}

func (s *Service) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.Environment == "production",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":{"code":%q,"message":%q}}`, code, message)
}
