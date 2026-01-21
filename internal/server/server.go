// Package server provides the HTTP server.
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sanixdarker/skill-md/internal/app"
	"github.com/sanixdarker/skill-md/internal/server/handlers"
	servermw "github.com/sanixdarker/skill-md/internal/server/middleware"
	"github.com/sanixdarker/skill-md/web"
)

// Server represents the HTTP server.
type Server struct {
	app    *app.App
	server *http.Server
	router *chi.Mux
}

// New creates a new Server.
func New(application *app.App) *Server {
	s := &Server{
		app:    application,
		router: chi.NewRouter(),
	}

	s.setupMiddleware()
	s.setupRoutes()

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", application.Config.Port),
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
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

	// Pages
	s.router.Get("/", homeHandler.Index)
	s.router.Get("/convert", convertHandler.Index)
	s.router.Get("/merge", mergeHandler.Index)
	s.router.Get("/skill/{slug}", skillsHandler.View)

	// External skill routes
	s.router.Get("/external/{source}/*", skillsHandler.ViewExternal)

	// API endpoints (HTMX)
	s.router.Post("/api/convert", convertHandler.Convert)
	s.router.Post("/api/convert/url", convertHandler.ConvertURL)
	s.router.Post("/api/convert/detect", convertHandler.DetectFormat)
	s.router.Post("/api/merge", mergeHandler.Merge)
	s.router.Post("/api/skills", skillsHandler.Create)
	s.router.Get("/api/skills", skillsHandler.List)
	s.router.Get("/api/skills/search", skillsHandler.Search)
	s.router.Get("/api/skill/{slug}", skillsHandler.Get)
	s.router.Delete("/api/skill/{id}", skillsHandler.Delete)
	s.router.Get("/api/skill/{slug}/download", skillsHandler.Download)
	s.router.Post("/api/skills/import-external", skillsHandler.ImportExternal)
	s.router.Get("/api/external/{source}/content/*", skillsHandler.GetExternalContent)
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
