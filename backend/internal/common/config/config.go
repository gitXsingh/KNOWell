package config

import (
	"bufio"
	"log"
	"os"
	"strings"
	"time"
)

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

	jwtSecret, ok := os.LookupEnv("JWT_SECRET")
	if !ok || jwtSecret == "" {
		log.Fatal("[FATAL] JWT_SECRET is required. Set it in the environment or .env file.")
	}
	githubTokenSecret, ok := os.LookupEnv("GITHUB_TOKEN_SECRET")
	if !ok || githubTokenSecret == "" {
		log.Fatal("[FATAL] GITHUB_TOKEN_SECRET is required. Set it in the environment or .env file.")
	}
	if githubTokenSecret == jwtSecret {
		log.Fatal("[FATAL] GITHUB_TOKEN_SECRET and JWT_SECRET must be different secrets.")
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
