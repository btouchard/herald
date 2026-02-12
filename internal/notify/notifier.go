package notify

// Event represents a task lifecycle notification.
type Event struct {
	Type    string // "task.started", "task.progress", "task.completed", "task.failed", "task.cancelled"
	TaskID  string
	Project string
	Message string

	// MCPSessionID targets a specific MCP client session.
	// Empty means broadcast to all.
	MCPSessionID string
}

// Notifier sends task lifecycle notifications.
type Notifier interface {
	Notify(event Event)
}

// Hub dispatches events to multiple notifiers.
type Hub struct {
	notifiers []Notifier
}

// NewHub creates a Hub with the given notifiers.
func NewHub(notifiers ...Notifier) *Hub {
	return &Hub{notifiers: notifiers}
}

// Notify sends an event to all registered notifiers.
func (h *Hub) Notify(event Event) {
	for _, n := range h.notifiers {
		go n.Notify(event)
	}
}
