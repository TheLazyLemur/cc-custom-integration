package claude

import (
	"encoding/json"
	"time"
)

// Message represents a JSON stream message from Claude CLI
type Message struct {
	Type         string          `json:"type"`
	Subtype      string          `json:"subtype,omitempty"`
	Message      json.RawMessage `json:"message,omitempty"`
	SessionID    string          `json:"session_id,omitempty"`
	IsError      bool            `json:"is_error,omitempty"`
	Result       string          `json:"result,omitempty"`
	DurationMs   int             `json:"duration_ms,omitempty"`
	NumTurns     int             `json:"num_turns,omitempty"`
	TotalCostUSD float64         `json:"total_cost_usd,omitempty"`
	Usage        *Usage          `json:"usage,omitempty"`
}

// Usage represents token usage statistics
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

// AssistantMessage represents an assistant response message
type AssistantMessage struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Role       string          `json:"role"`
	Model      string          `json:"model"`
	Content    json.RawMessage `json:"content"`
	StopReason string          `json:"stop_reason"`
}

// SystemInit represents system initialization message
type SystemInit struct {
	CWD       string   `json:"cwd"`
	SessionID string   `json:"session_id"`
	Tools     []string `json:"tools"`
	Model     string   `json:"model"`
}

// SessionStats represents accumulated session statistics
type SessionStats struct {
	CumulativeDuration int       `json:"cumulative_duration"`
	CumulativeTurns    int       `json:"cumulative_turns"`
	CumulativeCost     float64   `json:"cumulative_cost"`
	CumulativeUsage    Usage     `json:"cumulative_usage"`
	ConversationStart  time.Time `json:"conversation_start"`
}

// Event represents events that can be emitted by the session manager
type Event struct {
	Type      EventType   `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// EventType represents the different types of events
type EventType string

const (
	EventSessionInit     EventType = "session_init"
	EventSessionUpdate   EventType = "session_update"
	EventMessageReceived EventType = "message_received"
	EventToolActivity    EventType = "tool_activity"
	EventError           EventType = "error"
	EventStatsUpdate     EventType = "stats_update"
)

// ConversationMessage represents a processed message for UI display
type ConversationMessage struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	IsError   bool      `json:"is_error"`
	ToolName  string    `json:"tool_name,omitempty"`
}

// SessionInfo represents session information for UI display
type SessionInfo struct {
	ID        string        `json:"id"`
	Model     string        `json:"model"`
	IsActive  bool          `json:"is_active"`
	Duration  time.Duration `json:"duration"`
	TurnCount int           `json:"turn_count"`
	TotalCost float64       `json:"total_cost"`
	Usage     Usage         `json:"usage"`
	CreatedAt time.Time     `json:"created_at"`
}
