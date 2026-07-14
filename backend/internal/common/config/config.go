package config

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"strings"
	"time"
)

func generateRandomSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type Config struct {
	Environment             string
	Address                 string
	DatabaseURL             string
	MigrationsDir           string
	JWTSecret               string
	JWTAccessTTL            time.Duration
	GitHubClientID          string
	GitHubClientSecret      string
	GitHubRedirectURL       string
	GitHubFrontendCallbackURL string
	GitHubScopes            string
	GitHubStateTTL          time.Duration
	GitHubTokenSecret       string
	GitHubWebhookURL        string
	CORSAllowedOrigins      []string
	GeminiAPIKey            string
	GeminiModel             string
}

func Load() Config {
	loadEnvFile()

	accessTTL, err := time.ParseDuration(getEnv("JWT_ACCESS_TTL", "24h"))
	if err != nil {
		accessTTL = 24 * time.Hour
	}

	githubStateTTL, err := time.ParseDuration(getEnv("GITHUB_STATE_TTL", "10m"))
	if err != nil {
		githubStateTTL = 10 * time.Minute
	}

	originsRaw := getEnv("CORS_ALLOWED_ORIGINS", "")
	var origins []string
	if originsRaw != "" {
		for _, o := range strings.Split(originsRaw, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				origins = append(origins, o)
			}
		}
	}

	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		jwtSecret = generateRandomSecret()
		log.Printf("[WARN] JWT_SECRET not set — generated temporary random secret. Sessions will be invalidated on restart.")
	}
	githubTokenSecret := getEnv("GITHUB_TOKEN_SECRET", "")
	if githubTokenSecret == "" {
		githubTokenSecret = generateRandomSecret()
		log.Printf("[WARN] GITHUB_TOKEN_SECRET not set — generated temporary random secret. GitHub tokens will be unrecoverable on restart.")
	}
	if githubTokenSecret == jwtSecret {
		log.Printf("[WARN] GITHUB_TOKEN_SECRET equals JWT_SECRET — these should be different secrets.")
	}

	return Config{
		Environment:               getEnv("APP_ENV", "development"),
		Address:                   getEnv("APP_ADDR", ":8080"),
		DatabaseURL:               getEnv("DATABASE_URL", ""),
		MigrationsDir:             getEnv("MIGRATIONS_DIR", "./migrations"),
		JWTSecret:                 jwtSecret,
		JWTAccessTTL:              accessTTL,
		GitHubClientID:            getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:        getEnv("GITHUB_CLIENT_SECRET", ""),
		GitHubRedirectURL:         getEnv("GITHUB_REDIRECT_URL", ""),
		GitHubFrontendCallbackURL: getEnv("GITHUB_FRONTEND_CALLBACK_URL", ""),
		GitHubScopes:              getEnv("GITHUB_SCOPES", "read:user repo admin:repo_hook"),
		GitHubStateTTL:            githubStateTTL,
		GitHubTokenSecret:         githubTokenSecret,
		GitHubWebhookURL:          getEnv("GITHUB_WEBHOOK_URL", ""),
		CORSAllowedOrigins:       origins,
		GeminiAPIKey:              getEnv("GEMINI_API_KEY", ""),
		GeminiModel:               getEnv("GEMINI_MODEL", "gemini-2.0-flash"),
	}
}

func loadEnvFile() {
	for _, path := range []string{".env", "../.env"} {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			val = strings.TrimSpace(val)
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
