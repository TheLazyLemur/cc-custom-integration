package app

import (
	"context"
	"sync"
	"time"

	"complex/internal/claude"

	tea "github.com/charmbracelet/bubbletea"
)

// EventBus manages event distribution throughout the application
type EventBus struct {
	subscribers map[claude.EventType][]chan claude.Event
	mutex       sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	program     *tea.Program
}

// NewEventBus creates a new event bus
func NewEventBus(ctx context.Context) *EventBus {
	busCtx, cancel := context.WithCancel(ctx)
	return &EventBus{
		subscribers: make(map[claude.EventType][]chan claude.Event),
		ctx:         busCtx,
		cancel:      cancel,
	}
}

// SetProgram sets the tea program for sending messages
func (eb *EventBus) SetProgram(program *tea.Program) {
	eb.program = program
}

// Subscribe subscribes to specific event types
func (eb *EventBus) Subscribe(eventType claude.EventType, bufferSize int) <-chan claude.Event {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	eventCh := make(chan claude.Event, bufferSize)
	eb.subscribers[eventType] = append(eb.subscribers[eventType], eventCh)

	return eventCh
}

// HandleEvent implements claude.EventHandler interface
func (eb *EventBus) HandleEvent(event claude.Event) {
	eb.mutex.RLock()
	subscribers, exists := eb.subscribers[event.Type]
	eb.mutex.RUnlock()

	if !exists {
		return
	}

	// Send event to all subscribers of this type
	for _, subscriber := range subscribers {
		select {
		case subscriber <- event:
		case <-eb.ctx.Done():
			return
		default:
			// Non-blocking send - drop event if channel is full
		}
	}

	// Send event to bubbletea program if available
	if eb.program != nil {
		eb.program.Send(EventMsg{Event: event})
	}
}

// Shutdown gracefully shuts down the event bus
func (eb *EventBus) Shutdown() {
	eb.cancel()

	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	// Close all subscriber channels
	for _, subscribers := range eb.subscribers {
		for _, ch := range subscribers {
			close(ch)
		}
	}

	eb.subscribers = make(map[claude.EventType][]chan claude.Event)
}

// EventMsg wraps claude.Event for bubbletea
type EventMsg struct {
	Event claude.Event
}

// SessionStateMsg represents session state changes
type SessionStateMsg struct {
	SessionInfo claude.SessionInfo
	Stats       claude.SessionStats
}

// MessageStreamMsg represents streaming message content
type MessageStreamMsg struct {
	Message   claude.ConversationMessage
	IsPartial bool
}

// ToolActivityMsg represents tool execution activity
type ToolActivityMsg struct {
	Activity string
	Status   string
}

// ErrorMsg represents error events
type ErrorMsg struct {
	Error     error
	Context   string
	Timestamp time.Time
}

// ConversationHistoryMsg represents conversation updates
type ConversationHistoryMsg struct {
	Messages []claude.ConversationMessage
}

// StatusMsg represents general status updates
type StatusMsg struct {
	Status  string
	Message string
}

// PromptInputMsg represents user prompt input
type PromptInputMsg struct {
	Prompt string
	Resume bool
}

// ResizeMsg represents terminal resize events
type ResizeMsg struct {
	Width  int
	Height int
}

// NavigationMsg represents UI navigation events
type NavigationMsg struct {
	Action string
	Target string
}

// CommandMsg represents application commands
type CommandMsg struct {
	Command string
	Args    []string
}

// QuitMsg represents quit application request
type QuitMsg struct{}

// EventProcessor processes events and converts them to bubbletea messages
type EventProcessor struct {
	eventBus *EventBus
	ctx      context.Context
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(ctx context.Context, eventBus *EventBus) *EventProcessor {
	return &EventProcessor{
		eventBus: eventBus,
		ctx:      ctx,
	}
}

// ProcessEvents starts processing events and sending them as tea messages
func (ep *EventProcessor) ProcessEvents(program *tea.Program) {
	// Subscribe to all event types
	sessionEvents := ep.eventBus.Subscribe(claude.EventSessionInit, 10)
	sessionUpdates := ep.eventBus.Subscribe(claude.EventSessionUpdate, 10)
	messageEvents := ep.eventBus.Subscribe(claude.EventMessageReceived, 50)
	toolEvents := ep.eventBus.Subscribe(claude.EventToolActivity, 20)
	errorEvents := ep.eventBus.Subscribe(claude.EventError, 20)
	statsEvents := ep.eventBus.Subscribe(claude.EventStatsUpdate, 10)

	go ep.processEventStream(sessionEvents, program, ep.handleSessionEvent)
	go ep.processEventStream(sessionUpdates, program, ep.handleSessionUpdate)
	go ep.processEventStream(messageEvents, program, ep.handleMessageEvent)
	go ep.processEventStream(toolEvents, program, ep.handleToolEvent)
	go ep.processEventStream(errorEvents, program, ep.handleErrorEvent)
	go ep.processEventStream(statsEvents, program, ep.handleStatsEvent)
}

// processEventStream processes a stream of events
func (ep *EventProcessor) processEventStream(eventCh <-chan claude.Event, program *tea.Program, handler func(claude.Event) tea.Msg) {
	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			if msg := handler(event); msg != nil {
				program.Send(msg)
			}
		case <-ep.ctx.Done():
			return
		}
	}
}

// Event handlers convert claude events to tea messages
func (ep *EventProcessor) handleSessionEvent(event claude.Event) tea.Msg {
	switch data := event.Data.(type) {
	case claude.SystemInit:
		return SessionStateMsg{
			SessionInfo: claude.SessionInfo{
				ID:        data.SessionID,
				Model:     data.Model,
				IsActive:  true,
				CreatedAt: event.Timestamp,
			},
		}
	case string:
		return StatusMsg{
			Status:  "session",
			Message: data,
		}
	}
	return nil
}

func (ep *EventProcessor) handleSessionUpdate(event claude.Event) tea.Msg {
	switch data := event.Data.(type) {
	case claude.SessionInfo:
		return SessionStateMsg{
			SessionInfo: data,
		}
	case string:
		return StatusMsg{
			Status:  "session_update",
			Message: data,
		}
	}
	return nil
}

func (ep *EventProcessor) handleMessageEvent(event claude.Event) tea.Msg {
	switch data := event.Data.(type) {
	case claude.ConversationMessage:
		return MessageStreamMsg{
			Message:   data,
			IsPartial: false,
		}
	case claude.Message:
		return StatusMsg{
			Status:  "raw_message",
			Message: string(data.Type),
		}
	}
	return nil
}

func (ep *EventProcessor) handleToolEvent(event claude.Event) tea.Msg {
	if activity, ok := event.Data.(string); ok {
		return ToolActivityMsg{
			Activity: activity,
			Status:   "active",
		}
	}
	return nil
}

func (ep *EventProcessor) handleErrorEvent(event claude.Event) tea.Msg {
	if err, ok := event.Data.(error); ok {
		return ErrorMsg{
			Error:     err,
			Context:   "session",
			Timestamp: event.Timestamp,
		}
	}
	return nil
}

func (ep *EventProcessor) handleStatsEvent(event claude.Event) tea.Msg {
	if stats, ok := event.Data.(claude.SessionStats); ok {
		return SessionStateMsg{
			Stats: stats,
		}
	}
	return nil
}
