package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/btouchard/herald/internal/config"
	"github.com/btouchard/herald/internal/executor"
	"github.com/btouchard/herald/internal/mcp/handlers"
	"github.com/btouchard/herald/internal/project"
	"github.com/btouchard/herald/internal/task"
)

// Deps holds shared dependencies injected into MCP handlers.
type Deps struct {
	Projects     *project.Manager
	Tasks        *task.Manager
	Store        handlers.DurationEstimator
	Execution    config.ExecutionConfig
	Capabilities executor.Capabilities
	Version      string
}

// serverInstructions are returned to the client during MCP initialize.
// They guide Claude Chat to delegate coding to Claude Code efficiently.
const serverInstructions = `Herald bridges you (Claude Chat) to Claude Code running on the user's machine.

IMPORTANT — prompt efficiency:
- Claude Code has FULL access to the codebase, files, and git history. You do NOT need to write code, file contents, or documentation in your prompts.
- Send concise, functional prompts that describe WHAT to do, not HOW. Claude Code will figure out the implementation.
- BAD: pasting a full file with modifications, writing code blocks for Claude Code to apply, or drafting documentation to be written.
- GOOD: "Add rate limiting middleware to the API routes (200 req/min per token)" or "Fix the null pointer in task.go when priority is empty".
- Think of yourself as a product manager giving clear requirements, not a developer writing code.
- If the user provides code or file content, summarize the intent instead of forwarding it verbatim.`

// NewServer creates and configures the MCP server with all tools registered.
func NewServer(deps *Deps) *server.MCPServer {
	s := server.NewMCPServer(
		"Herald",
		deps.Version,
		server.WithToolCapabilities(true),
		server.WithLogging(),
		server.WithInstructions(serverInstructions),
	)

	registerTools(s, deps)

	return s
}
