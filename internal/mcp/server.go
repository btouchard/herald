package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/kolapsis/herald/internal/config"
	"github.com/kolapsis/herald/internal/mcp/handlers"
	"github.com/kolapsis/herald/internal/project"
	"github.com/kolapsis/herald/internal/task"
)

// Deps holds shared dependencies injected into MCP handlers.
type Deps struct {
	Projects  *project.Manager
	Tasks     *task.Manager
	Store     handlers.DurationEstimator
	Execution config.ExecutionConfig
	Version   string
}

// NewServer creates and configures the MCP server with all tools registered.
func NewServer(deps *Deps) *server.MCPServer {
	s := server.NewMCPServer(
		"Herald",
		deps.Version,
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	registerTools(s, deps)

	return s
}
