package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// EventHandler defines the interface for handling session events
type EventHandler interface {
	HandleEvent(event Event)
}

// SessionManager manages Claude CLI sessions with event emission
type SessionManager struct {
	CurrentSessionID   string
	Model              string
	SessionChain       []string
	CumulativeDuration int
	CumulativeTurns    int
	CumulativeCost     float64
	CumulativeUsage    Usage
	ConversationStart  time.Time

	// Event handling
	eventHandlers []EventHandler
	eventMutex    sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		ConversationStart: time.Now(),
		eventHandlers:     make([]EventHandler, 0),
	}
}

// AddEventHandler registers an event handler
func (sm *SessionManager) AddEventHandler(handler EventHandler) {
	sm.eventMutex.Lock()
	defer sm.eventMutex.Unlock()
	sm.eventHandlers = append(sm.eventHandlers, handler)
}

// emitEvent sends an event to all registered handlers
func (sm *SessionManager) emitEvent(eventType EventType, data interface{}) {
	sm.eventMutex.RLock()
	defer sm.eventMutex.RUnlock()

	event := Event{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	}

	for _, handler := range sm.eventHandlers {
		go handler.HandleEvent(event)
	}
}

// ExecuteCommand executes a Claude CLI command with event emission
func (sm *SessionManager) ExecuteCommand(ctx context.Context, prompt string, resume bool) error {
	args := []string{
		"--output-format", "stream-json",
		"--verbose",
		"-p",
		"--permission-prompt-tool", "mcp__permission__approval_prompt",
		"--model", "claude-sonnet-4-20250514",
		"--mcp-config", "config.json",
	}

	if sm.Model != "" {
		args = append(args, "--model", sm.Model)
	}

	if resume && sm.CurrentSessionID != "" {
		args = append(args, "--resume", sm.CurrentSessionID)
	}

	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, "claude", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sm.emitEvent(EventError, fmt.Errorf("failed to create stdout pipe: %w", err))
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		sm.emitEvent(EventError, fmt.Errorf("failed to create stderr pipe: %w", err))
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		sm.emitEvent(EventError, fmt.Errorf("failed to start command: %w", err))
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Handle stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			sm.emitEvent(EventError, fmt.Errorf("stderr: %s", scanner.Text()))
		}
	}()

	if err := sm.ProcessStream(stdout); err != nil {
		sm.emitEvent(EventError, fmt.Errorf("failed to process stream: %w", err))
		return fmt.Errorf("failed to process stream: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		sm.emitEvent(EventError, fmt.Errorf("command failed: %w", err))
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// ProcessStream processes the JSON stream from Claude CLI with event emission
func (sm *SessionManager) ProcessStream(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse the JSON line directly without our Message wrapper
		sm.processJSONLine(line)
	}

	if err := scanner.Err(); err != nil {
		sm.emitEvent(EventError, fmt.Errorf("scanner error: %w", err))
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// processJSONLine processes a raw JSON line from Claude CLI
func (sm *SessionManager) processJSONLine(line string) {
	// First, determine the message type
	var msgType struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype,omitempty"`
	}

	if err := json.Unmarshal([]byte(line), &msgType); err != nil {
		sm.emitEvent(EventError, fmt.Errorf("parse error: %s", line))
		return
	}

	switch msgType.Type {
	case "system":
		if msgType.Subtype == "init" {
			var init SystemInit
			if err := json.Unmarshal([]byte(line), &init); err == nil {
				sm.CurrentSessionID = init.SessionID
				sm.Model = init.Model
				sm.emitEvent(EventSessionInit, init)
			}
		}

	case "assistant":
		// Use the exact same parsing as the original simple CLI
		var assistantData struct {
			Message AssistantMessage `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &assistantData); err == nil {
			sm.processAssistantMessage(assistantData.Message)
		} else {
			sm.emitEvent(EventError, fmt.Errorf("failed to parse assistant message: %w", err))
		}

	case "user":
		// Tool results - emit tool activity event
		sm.emitEvent(EventToolActivity, "tool_execution_progress")

	case "result":
		var result Message
		if err := json.Unmarshal([]byte(line), &result); err == nil {
			if result.Subtype == "success" {
				sm.updateSessionStats(result)
				sm.emitEvent(EventSessionUpdate, sm.getCurrentSessionInfo())
				sm.emitEvent(EventStatsUpdate, sm.getSessionStats())
			} else if result.IsError {
				sm.emitEvent(EventError, fmt.Errorf("result error: %s", result.Result))
			}
		}
	}
}

// processAssistantMessage processes assistant messages and emits conversation events
func (sm *SessionManager) processAssistantMessage(assistantMsg AssistantMessage) {
	var content []map[string]interface{}
	if err := json.Unmarshal(assistantMsg.Content, &content); err == nil {
		for _, item := range content {
			if item["type"] == "text" {
				if text, ok := item["text"].(string); ok {
					convMsg := ConversationMessage{
						ID:        assistantMsg.ID,
						Type:      "assistant",
						Content:   text,
						Timestamp: time.Now(),
						IsError:   false,
					}
					sm.emitEvent(EventMessageReceived, convMsg)
				}
			} else if item["type"] == "tool_use" {
				if toolName, ok := item["name"].(string); ok {
					sm.emitEvent(EventToolActivity, fmt.Sprintf("executing_tool_%s", toolName))
					convMsg := ConversationMessage{
						ID:        assistantMsg.ID,
						Type:      "tool_use",
						Content:   fmt.Sprintf("Using tool: %s", toolName),
						Timestamp: time.Now(),
						IsError:   false,
						ToolName:  toolName,
					}
					sm.emitEvent(EventMessageReceived, convMsg)
				}
			}
		}
	}
}

// updateSessionStats updates session statistics
func (sm *SessionManager) updateSessionStats(msg Message) {
	// Update current session ID - this is critical for session continuity
	sm.CurrentSessionID = msg.SessionID

	// Add to session chain (matching original simple CLI behavior)
	sm.SessionChain = append(sm.SessionChain, msg.SessionID)

	// Update cumulative statistics
	sm.CumulativeDuration += msg.DurationMs
	sm.CumulativeTurns += msg.NumTurns
	sm.CumulativeCost += msg.TotalCostUSD

	if msg.Usage != nil {
		sm.CumulativeUsage.InputTokens += msg.Usage.InputTokens
		sm.CumulativeUsage.CacheCreationInputTokens += msg.Usage.CacheCreationInputTokens
		sm.CumulativeUsage.CacheReadInputTokens += msg.Usage.CacheReadInputTokens
		sm.CumulativeUsage.OutputTokens += msg.Usage.OutputTokens
	}
}

// getCurrentSessionInfo returns current session information
func (sm *SessionManager) getCurrentSessionInfo() SessionInfo {
	return SessionInfo{
		ID:        sm.CurrentSessionID,
		Model:     sm.Model,
		IsActive:  true,
		Duration:  time.Since(sm.ConversationStart),
		TurnCount: sm.CumulativeTurns,
		TotalCost: sm.CumulativeCost,
		Usage:     sm.CumulativeUsage,
		CreatedAt: sm.ConversationStart,
	}
}

// getSessionStats returns current session statistics
func (sm *SessionManager) getSessionStats() SessionStats {
	return SessionStats{
		CumulativeDuration: sm.CumulativeDuration,
		CumulativeTurns:    sm.CumulativeTurns,
		CumulativeCost:     sm.CumulativeCost,
		CumulativeUsage:    sm.CumulativeUsage,
		ConversationStart:  sm.ConversationStart,
	}
}

// StartNewConversation resets the session manager for a new conversation
func (sm *SessionManager) StartNewConversation() {
	if len(sm.SessionChain) > 0 {
		sm.emitEvent(EventSessionUpdate, "conversation_ended")
	}

	sm.CurrentSessionID = ""
	sm.SessionChain = nil
	sm.CumulativeDuration = 0
	sm.CumulativeTurns = 0
	sm.CumulativeCost = 0
	sm.CumulativeUsage = Usage{}
	sm.ConversationStart = time.Now()

	sm.emitEvent(EventSessionInit, "new_conversation_started")
}

// SetModel sets the model for the session manager
func (sm *SessionManager) SetModel(model string) {
	sm.Model = model
	sm.emitEvent(EventSessionUpdate, fmt.Sprintf("model_changed_%s", model))
}

// GetSessionChain returns the current session chain
func (sm *SessionManager) GetSessionChain() []string {
	return append([]string(nil), sm.SessionChain...)
}

// GetCurrentSession returns current session info
func (sm *SessionManager) GetCurrentSession() SessionInfo {
	return sm.getCurrentSessionInfo()
}

// GetStats returns current statistics
func (sm *SessionManager) GetStats() SessionStats {
	return sm.getSessionStats()
}
