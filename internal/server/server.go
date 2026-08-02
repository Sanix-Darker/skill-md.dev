// Package server provides the HTTP server.
package server

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sanixdarker/skillf/internal/app"
	"github.com/sanixdarker/skillf/internal/server/handlers"
	servermw "github.com/sanixdarker/skillf/internal/server/middleware"
	"github.com/sanixdarker/skillf/web"
)

// Server represents the HTTP server.
type Server struct {
	app         *app.App
	server      *http.Server
	router      *chi.Mux
	rateLimiter *servermw.RateLimiter
}

// New creates a new Server.
func New(application *app.App) *Server {
	s := &Server{
		app:         application,
		router:      chi.NewRouter(),
		rateLimiter: servermw.NewRateLimiter(5, 20), // 5 req/sec, burst of 20
	}

	s.setupMiddleware()
	s.setupRoutes()

	s.server = &http.Server{
		Addr:         net.JoinHostPort(resolveListenHost(application.Config.ListenHost), strconv.Itoa(application.Config.Port)),
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(s.rateLimiter.Limit) // Rate limiting
	s.router.Use(servermw.SecurityHeaders)
	s.router.Use(servermw.Logger(s.app.Logger))
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Compress(5))
	s.router.Use(servermw.HTMX)
}

func (s *Server) setupRoutes() {
	// Static files
	s.router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(web.StaticFS))))

	// Create handlers
	homeHandler := handlers.NewHomeHandler(s.app)
	convertHandler := handlers.NewConvertHandler(s.app)
	mergeHandler := handlers.NewMergeHandler(s.app)
	skillsHandler := handlers.NewSkillsHandler(s.app)
	systemHandler := handlers.NewSystemHandler(s.app)

	// Pages
	s.router.Get("/", homeHandler.Index)
	s.router.Get("/convert", convertHandler.Index)
	s.router.Get("/merge", mergeHandler.Index)
	s.router.Get("/ssh", homeHandler.SSH)
	s.router.Get("/skill/{slug}", skillsHandler.View)

	// External skill routes

	// API endpoints (HTMX)
	s.router.Get("/health", systemHandler.Health)
	s.router.Get("/api/system", systemHandler.System)
	s.router.Post("/api/convert", convertHandler.Convert)
	s.router.Post("/api/convert/url", convertHandler.ConvertURL)
	s.router.Post("/api/convert/detect", convertHandler.DetectFormat)
	s.router.Post("/api/merge", mergeHandler.Merge)
	s.router.Get("/api/merge/strategies", mergeHandler.Strategies)
	s.router.Post("/api/skills", skillsHandler.Create)
	s.router.Get("/api/skills", skillsHandler.List)
	s.router.Get("/api/skill/{slug}", skillsHandler.Get)
	s.router.Delete("/api/skill/{id}", skillsHandler.Delete)
	s.router.Get("/api/skill/{slug}/download", skillsHandler.Download)
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func resolveListenHost(host string) string {
	trimmedHost := strings.TrimSpace(strings.Trim(host, "[]"))
	if trimmedHost == "" {
		return "0.0.0.0"
	}
	return trimmedHost
}
