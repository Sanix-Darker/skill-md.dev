// Package app provides the application container and dependency injection.
package app

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"github.com/sanixdarker/skillforge/internal/converter"
	"github.com/sanixdarker/skillforge/internal/merger"
	"github.com/sanixdarker/skillforge/internal/registry"
	"github.com/sanixdarker/skillforge/internal/sources"
	"github.com/sanixdarker/skillforge/internal/storage"
)

// Config holds application configuration.
type Config struct {
	Port              int
	DBPath            string
	StaticPath        string
	Debug             bool
	GitHubToken       string
	GitLabToken       string
	BitbucketUsername string
	BitbucketPassword string
	CodebergToken     string
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Port:   8080,
		DBPath: "./skillforge.db",
		Debug:  false,
	}
}

// App is the main application container.
type App struct {
	Config           *Config
	DB               *sql.DB
	Logger           *slog.Logger
	ConverterManager *converter.Manager
	Merger           *merger.Merger
	RegistryService  *registry.Service
	FederatedSource  *sources.FederatedSource
}

// New creates a new application instance.
func New(cfg *Config) (*App, error) {
	// Set up logger
	logLevel := slog.LevelInfo
	if cfg.Debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Initialize database
	db, err := storage.NewDB(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run migrations
	if err := storage.Migrate(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize converter manager
	converterManager := converter.NewManager()

	// Initialize merger
	mergerInstance := merger.New()

	// Initialize registry service
	repo := registry.NewRepository(db)
	registryService := registry.NewService(repo)

	// Initialize federated source
	federatedSource := sources.NewFederatedSource(logger)

	// Register local source
	localSource := sources.NewLocalSource(registryService)
	federatedSource.RegisterSource(localSource)

	// Register external sources
	skillsshSource := sources.NewSkillsSHSource()
	federatedSource.RegisterSource(skillsshSource)

	githubSource := sources.NewGitHubSource(cfg.GitHubToken)
	federatedSource.RegisterSource(githubSource)

	gitlabSource := sources.NewGitLabSource(cfg.GitLabToken)
	federatedSource.RegisterSource(gitlabSource)

	bitbucketSource := sources.NewBitbucketSource(cfg.BitbucketUsername, cfg.BitbucketPassword)
	federatedSource.RegisterSource(bitbucketSource)

	codebergSource := sources.NewCodebergSource(cfg.CodebergToken)
	federatedSource.RegisterSource(codebergSource)

	return &App{
		Config:           cfg,
		DB:               db,
		Logger:           logger,
		ConverterManager: converterManager,
		Merger:           mergerInstance,
		RegistryService:  registryService,
		FederatedSource:  federatedSource,
	}, nil
}

// Close cleans up application resources.
func (a *App) Close() error {
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}
