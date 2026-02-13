package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mark3labs/mcp-go/server"

	"github.com/btouchard/herald/internal/auth"
	"github.com/btouchard/herald/internal/config"
	"github.com/btouchard/herald/internal/executor"
	heraldmcp "github.com/btouchard/herald/internal/mcp"
	authmw "github.com/btouchard/herald/internal/mcp/middleware"
	"github.com/btouchard/herald/internal/notify"
	"github.com/btouchard/herald/internal/project"
	"github.com/btouchard/herald/internal/store"
	"github.com/btouchard/herald/internal/task"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cmdServe(os.Args[2:])
	case "version":
		fmt.Printf("herald %s\n", version)
	case "check":
		cmdCheck(os.Args[2:])
	case "health":
		cmdHealth(os.Args[2:])
	case "rotate-secret":
		cmdRotateSecret(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: herald <command> [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  serve           Start the Herald server\n")
	fmt.Fprintf(os.Stderr, "  check           Validate configuration\n")
	fmt.Fprintf(os.Stderr, "  health          Check if the server is running\n")
	fmt.Fprintf(os.Stderr, "  rotate-secret   Generate a new client secret (invalidates sessions)\n")
	fmt.Fprintf(os.Stderr, "  version         Print version\n")
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config file")
	_ = fs.Parse(args) // ExitOnError handles errors

	cfg, err := loadConfig(*configPath)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Load or auto-generate client secret (env var takes precedence)
	if err := ensureClientSecret(cfg, *configPath); err != nil {
		slog.Error("failed to load client secret", "error", err)
		os.Exit(1)
	}

	setupLogging(cfg)

	slog.Info("starting herald",
		"version", version,
		"host", cfg.Server.Host,
		"port", cfg.Server.Port)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func cmdCheck(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config file")
	_ = fs.Parse(args) // ExitOnError handles errors

	_, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("configuration is valid")
}

func cmdHealth(args []string) {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	port := fs.Int("port", 8420, "server port")
	_ = fs.Parse(args) // ExitOnError handles errors

	url := fmt.Sprintf("http://127.0.0.1:%d/health", *port)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unhealthy: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "unhealthy: status %d\n", resp.StatusCode)
		os.Exit(1)
	}
	fmt.Println("healthy")
}

func cmdRotateSecret(args []string) {
	fs := flag.NewFlagSet("rotate-secret", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config file")
	_ = fs.Parse(args) // ExitOnError handles errors

	dir := configDirFrom(*configPath)
	secret, err := auth.RotateSecret(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	_ = secret // don't print the secret itself
	fmt.Println("Secret rotated. Restart Herald to apply. All existing sessions will be invalidated.")
}

// ensureClientSecret loads the client secret into cfg.Auth.ClientSecret.
// Priority: env var HERALD_CLIENT_SECRET > config file value > auto-generated file.
func ensureClientSecret(cfg *config.Config, configPath string) error {
	// If the config already has a non-empty secret (from YAML with env var
	// substitution), keep it.
	if cfg.Auth.ClientSecret != "" {
		return nil
	}

	dir := configDirFrom(configPath)
	secret, err := auth.LoadOrCreateSecret(dir)
	if err != nil {
		return err
	}

	cfg.Auth.ClientSecret = secret
	slog.Info("client secret loaded from file", "path", filepath.Join(dir, "secret"))
	return nil
}

// configDirFrom returns the config directory. If a config file path is given,
// its parent directory is used; otherwise falls back to ~/.config/herald.
func configDirFrom(configPath string) string {
	if configPath != "" {
		return filepath.Dir(configPath)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "herald")
	}
	return "."
}

func loadConfig(path string) (*config.Config, error) {
	if path != "" {
		return config.LoadFromFile(path)
	}
	return config.Load()
}

func setupLogging(cfg *config.Config) {
	var level slog.Level
	switch cfg.Server.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handlers := []slog.Handler{
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	}

	if cfg.Server.LogFile != "" {
		f, err := os.OpenFile(cfg.Server.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
		if err != nil {
			slog.Warn("failed to open log file, using stdout only", "path", cfg.Server.LogFile, "error", err)
		} else {
			handlers = append(handlers, slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level}))
		}
	}

	logger := slog.New(slog.NewMultiHandler(handlers...))
	slog.SetDefault(logger)
}

func run(ctx context.Context, cfg *config.Config) error {
	// --- SQLite Store ---
	dbPath := config.ExpandHome(cfg.Database.Path)
	db, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = db.Close() }()

	slog.Info("database opened", "path", dbPath)

	// --- Project Manager ---
	pm := project.NewManager(cfg.Projects)
	if err := pm.Validate(); err != nil {
		return fmt.Errorf("project validation: %w", err)
	}

	// --- Executor ---
	exec := &executor.ClaudeExecutor{
		ClaudePath: cfg.Execution.ClaudePath,
		WorkDir:    cfg.Execution.WorkDir,
		Env:        cfg.Execution.Env,
	}

	// --- Task Manager ---
	tm := task.NewManager(exec, cfg.Execution.MaxConcurrent, cfg.Execution.MaxTimeout)

	// --- MCP Server ---
	mcpServer := heraldmcp.NewServer(&heraldmcp.Deps{
		Projects:  pm,
		Tasks:     tm,
		Store:     db,
		Execution: cfg.Execution,
		Version:   version,
	})

	// --- Push Notifications ---
	mcpNotifier := notify.NewMCPNotifier(mcpServer, 3*time.Second)
	hub := notify.NewHub(mcpNotifier)
	tm.SetNotifyFunc(func(e task.TaskEvent) {
		hub.Notify(notify.Event{
			Type:         e.Type,
			TaskID:       e.TaskID,
			Project:      e.Project,
			Message:      e.Message,
			MCPSessionID: e.MCPSessionID,
		})
	})

	mcpHTTP := server.NewStreamableHTTPServer(mcpServer)

	// --- OAuth Server (backed by SQLite) ---
	authStore := auth.NewSQLiteAuthStore(db)
	oauth := auth.NewOAuthServerWithStore(cfg.Auth, cfg.Server.PublicURL, authStore)
	go oauth.StartCleanupLoop(ctx.Done())

	// --- HTTP Router ---
	r := chi.NewRouter()
	r.Use(authmw.SecurityHeaders)

	// Protected Resource Metadata (RFC 9728) — required for MCP OAuth discovery
	r.Get("/.well-known/oauth-protected-resource", oauth.HandleProtectedResourceMetadata)

	// OAuth endpoints (no auth required, IP rate limited against brute force)
	oauthLimiter := authmw.IPRateLimit(10, 5)
	r.Group(func(r chi.Router) {
		r.Use(oauthLimiter)
		r.Get("/.well-known/oauth-authorization-server", oauth.HandleMetadata)
		r.Get("/oauth/authorize", oauth.HandleAuthorize)
		r.Post("/oauth/token", oauth.HandleToken)
	})

	// MCP endpoint (rate limited + Bearer token required)
	resourceMetadataURL := cfg.Server.PublicURL + "/.well-known/oauth-protected-resource"
	r.Group(func(r chi.Router) {
		r.Use(authmw.RateLimit(cfg.RateLimit))
		r.Use(authmw.BearerAuth(oauth, resourceMetadataURL))
		r.Handle("/mcp", mcpHTTP)
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// --- HTTP Server ---
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("herald is ready", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
