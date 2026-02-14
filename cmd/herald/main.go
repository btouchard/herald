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
	"strings"
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
	"github.com/btouchard/herald/internal/tunnel"
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	err = run(ctx, cfg)
	stop()
	if err != nil {
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
	status := resp.StatusCode
	_ = resp.Body.Close()

	if status != http.StatusOK {
		fmt.Fprintf(os.Stderr, "unhealthy: status %d\n", status)
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

// printBanner displays a formatted startup summary with server info and OAuth credentials.
func printBanner(cfg *config.Config, tunnelURL string) {
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Herald %s - powered by Benjamin Touchard - https://kolapsis.com\n", version)
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Server:          %s\n", addr)
	if cfg.Server.PublicURL != "" {
		fmt.Fprintf(os.Stderr, "  Public URL:      %s\n", cfg.Server.PublicURL)
	}
	if tunnelURL != "" {
		fmt.Fprintf(os.Stderr, "  Tunnel:          %s (%s)\n", tunnelURL, cfg.Tunnel.Provider)
	}
	fmt.Fprintf(os.Stderr, "  Database:        %s\n", cfg.Database.Path)
	fmt.Fprintf(os.Stderr, "  Max concurrent:  %d\n", cfg.Execution.MaxConcurrent)

	// Projects
	if len(cfg.Projects) > 0 {
		names := make([]string, 0, len(cfg.Projects))
		for name := range cfg.Projects {
			names = append(names, name)
		}
		fmt.Fprintf(os.Stderr, "  Projects:        %s\n", strings.Join(names, ", "))
	} else {
		fmt.Fprintf(os.Stderr, "  Projects:        (none)\n")
	}

	// OAuth
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Custom Connector (OAuth 2.1):\n")
	fmt.Fprintf(os.Stderr, "    Client ID:     %s\n", cfg.Auth.ClientID)
	fmt.Fprintf(os.Stderr, "    Client Secret: %s\n", cfg.Auth.ClientSecret)
	if len(cfg.Auth.RedirectURIs) > 0 {
		fmt.Fprintf(os.Stderr, "    Redirect URIs: %s\n", cfg.Auth.RedirectURIs[0])
		for _, uri := range cfg.Auth.RedirectURIs[1:] {
			fmt.Fprintf(os.Stderr, "                   %s\n", uri)
		}
	} else {
		fmt.Fprintf(os.Stderr, "    Redirect URIs: (none — auth will fail! see configs/herald.example.yaml)\n")
	}
	fmt.Fprintf(os.Stderr, "\n")
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
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	}

	if cfg.Server.LogFile != "" {
		f, err := os.OpenFile(cfg.Server.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
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

	// --- Optional Tunnel (ngrok) ---
	// Start tunnel BEFORE OAuth to ensure cfg.Server.PublicURL is set correctly
	var tun tunnel.Tunnel
	tunnelURL := ""
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	if cfg.Tunnel.Enabled {
		slog.Info("tunnel enabled", "provider", cfg.Tunnel.Provider)

		switch cfg.Tunnel.Provider {
		case "ngrok":
			tun = tunnel.NewNgrok(cfg.Tunnel.AuthToken, cfg.Tunnel.Domain)
			publicURL, err := tun.Start(ctx, addr)
			if err != nil {
				slog.Warn("failed to start tunnel, continuing with local server only", "error", err)
			} else {
				tunnelURL = publicURL
				// Override PublicURL with tunnel URL for OAuth metadata
				cfg.Server.PublicURL = tunnelURL
				slog.Info("tunnel established", "public_url", tunnelURL)
			}
		default:
			slog.Warn("unknown tunnel provider, ignoring", "provider", cfg.Tunnel.Provider)
		}
	}

	// --- OAuth Server (backed by SQLite) ---
	// Now uses the correct PublicURL (either from config or from tunnel)
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

	// Favicon (embedded SVG — overrides parent domain favicon for Custom Connector icon)
	r.Get("/favicon.ico", serveFavicon)
	r.Get("/favicon.svg", serveFavicon)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// --- HTTP Server (local) ---
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	errCh := make(chan error, 2)

	// Start local server
	go func() {
		slog.Info("starting local server", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("local server: %w", err)
		}
	}()

	// Start tunnel server (if tunnel was established)
	if tun != nil && tunnelURL != "" {
		go func() {
			slog.Info("starting tunnel server", "public_url", tunnelURL)
			if err := http.Serve(tun.Listener(), r); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("tunnel server: %w", err)
			}
		}()
	}

	// Print banner after all servers are started
	printBanner(cfg, tunnelURL)
	slog.Info("herald is ready", "local", addr, "tunnel", tunnelURL)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
	}

	slog.Info("shutting down")

	// Close tunnel first
	if tun != nil {
		if err := tun.Close(); err != nil {
			slog.Warn("failed to close tunnel", "error", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("graceful shutdown timed out, forcing close", "error", err)
		srv.Close()
	}

	return nil
}

// Herald favicon — yellow-green tilted rounded square with dark "H".
const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
<g transform="rotate(-3 256 256)">
<rect x="18" y="18" width="476" height="476" rx="95" fill="#c8ff00"/>
<g fill="#0a0a0f">
  <rect x="142" y="120" width="56" height="272" rx="8"/>
  <rect x="314" y="120" width="56" height="272" rx="8"/>
  <path d="M190 232 L322 220 L322 272 L190 284 Z"/>
</g>
</g>
</svg>`

func serveFavicon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(faviconSVG))
}
