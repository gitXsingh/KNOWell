package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/ai"
	"github.com/gitXsingh/knowell/backend/internal/auth"
	"github.com/gitXsingh/knowell/backend/internal/common/config"
	"github.com/gitXsingh/knowell/backend/internal/common/ratelimit"
	"github.com/gitXsingh/knowell/backend/internal/github"
	"github.com/gitXsingh/knowell/backend/internal/knowledge"
	"github.com/gitXsingh/knowell/backend/internal/project"
	"github.com/gitXsingh/knowell/backend/internal/search"
	"github.com/gitXsingh/knowell/backend/internal/timeline"
	"github.com/gitXsingh/knowell/backend/internal/webhook"
	"github.com/gitXsingh/knowell/backend/internal/workspace"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	cfg    config.Config
	db     *sql.DB
	http   *http.Server
	router chi.Router
}

func New(cfg config.Config, database *sql.DB) *Server {
	authService := auth.NewService(database, cfg)
	timelineService := timeline.NewService(database)
	knowledgeService := knowledge.NewService(database, timelineService)
	aiService := ai.NewService(database, cfg, timelineService, knowledgeService)
	webhookService := webhook.NewService(database, aiService, timelineService)
	searchService := search.NewService(database)
	githubService := github.NewService(database, cfg)
	githubService.SetWebhookProcessor(webhookService)
	workspaceService := workspace.NewService(database)
	projectService := project.NewService(database, githubService, aiService, webhookService, knowledgeService, searchService, timelineService)
	router := chi.NewRouter()
	origins := cfg.CORSAllowedOrigins
	if len(origins) == 0 {
		log.Println("[WARN] CORS_ALLOWED_ORIGINS not set — defaulting to localhost origins. Set this for production.")
		origins = []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"http://localhost:8080",
			"http://127.0.0.1:8080",
		}
	}
	router.Use(recoveryMiddleware)
	router.Use(corsMiddleware(origins))
	router.Use(requestLogger)
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	router.Get("/ai/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(aiService.Status())
	})
	router.With(bodySizeLimit(1 << 20), ratelimit.New(5, 10).Middleware).Route("/auth", authService.Routes)
	router.Route("/github", func(r chi.Router) {
		r.Use(bodySizeLimit(10 << 20), ratelimit.New(10, 20).Middleware)
		githubService.Routes(r, authService.Middleware)
	})
	router.With(bodySizeLimit(1 << 20), ratelimit.New(30, 50).Middleware).Route("/workspaces", func(r chi.Router) {
		workspaceService.Routes(r, authService.Middleware)
		r.Route("/{workspaceID}/projects", func(r chi.Router) {
			projectService.Routes(r, authService.Middleware)
		})
	})

	return &Server{
		cfg:    cfg,
		db:     database,
		router: router,
		http: &http.Server{
			Addr:              cfg.Address,
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

func (s *Server) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = s.http.Shutdown(context.Background())
	}()

	fmt.Printf("starting server on %s\n", s.cfg.Address)
	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := make([]byte, 4096)
				n := runtime.Stack(stack, false)
				log.Printf("[PANIC] %s %s: %v\n%s", r.Method, r.URL.Path, rec, stack[:n])
				http.Error(w, `{"error":{"code":"internal_error","message":"Something went wrong"}}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("[%s] %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
		log.Printf("[%s] %s completed in %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Add("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func bodySizeLimit(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}
