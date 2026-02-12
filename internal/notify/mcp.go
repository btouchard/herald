package notify

import (
	"log/slog"
	"sync"
	"time"
)

// MCPSender abstracts the mcp-go server notification methods.
// Defined consumer-side per Go convention.
type MCPSender interface {
	SendNotificationToSpecificClient(sessionID string, method string, params map[string]any) error
	SendNotificationToAllClients(method string, params map[string]any)
}

// MCPNotifier pushes task updates to Claude Chat via MCP notifications.
type MCPNotifier struct {
	sender   MCPSender
	debounce time.Duration

	mu       sync.Mutex
	lastSent map[string]time.Time // taskID → last progress notification time
}

// NewMCPNotifier creates an MCPNotifier with the given debounce interval
// for progress events. Terminal events (completed, failed, cancelled) are
// always sent immediately.
func NewMCPNotifier(sender MCPSender, debounce time.Duration) *MCPNotifier {
	if debounce <= 0 {
		debounce = 3 * time.Second
	}
	return &MCPNotifier{
		sender:   sender,
		debounce: debounce,
		lastSent: make(map[string]time.Time),
	}
}

// Notify sends an MCP notification for the given event.
func (n *MCPNotifier) Notify(event Event) {
	switch event.Type {
	case "task.progress":
		n.sendProgress(event)
	case "task.started":
		n.sendMessage(event, "info")
	case "task.completed":
		n.clearDebounce(event.TaskID)
		n.sendMessage(event, "info")
	case "task.failed":
		n.clearDebounce(event.TaskID)
		n.sendMessage(event, "error")
	case "task.cancelled":
		n.clearDebounce(event.TaskID)
		n.sendMessage(event, "warning")
	default:
		slog.Debug("mcp notifier: unknown event type", "type", event.Type)
	}
}

// sendProgress sends a notifications/progress with debounce.
func (n *MCPNotifier) sendProgress(event Event) {
	n.mu.Lock()
	last, ok := n.lastSent[event.TaskID]
	if ok && time.Since(last) < n.debounce {
		n.mu.Unlock()
		return
	}
	n.lastSent[event.TaskID] = time.Now()
	n.mu.Unlock()

	params := map[string]any{
		"progressToken": event.TaskID,
		"progress":      -1, // indeterminate
		"total":         1,
		"message":       event.Message,
	}

	n.send(event.MCPSessionID, "notifications/progress", params)
}

// sendMessage sends a notifications/message for terminal/start events.
func (n *MCPNotifier) sendMessage(event Event, level string) {
	params := map[string]any{
		"level":  level,
		"logger": "herald",
		"data": map[string]any{
			"type":    event.Type,
			"task_id": event.TaskID,
			"project": event.Project,
			"message": event.Message,
		},
	}

	n.send(event.MCPSessionID, "notifications/message", params)
}

// send dispatches to a specific client or broadcasts.
func (n *MCPNotifier) send(mcpSessionID, method string, params map[string]any) {
	if mcpSessionID != "" {
		if err := n.sender.SendNotificationToSpecificClient(mcpSessionID, method, params); err != nil {
			slog.Debug("mcp notification failed, falling back to broadcast",
				"session_id", mcpSessionID,
				"method", method,
				"error", err)
			n.sender.SendNotificationToAllClients(method, params)
		}
		return
	}
	n.sender.SendNotificationToAllClients(method, params)
}

// clearDebounce removes the debounce entry for a finished task.
func (n *MCPNotifier) clearDebounce(taskID string) {
	n.mu.Lock()
	delete(n.lastSent, taskID)
	n.mu.Unlock()
}
