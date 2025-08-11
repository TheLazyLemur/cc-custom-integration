package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"complex/internal/claude"
	"complex/internal/ui/components"
)

// ApplicationState represents the current state of the application
type ApplicationState int

const (
	StateMain ApplicationState = iota
	StateSettings
	StateHelp
)

// InputMode represents the vim-like input mode
type InputMode int

const (
	InputModeNormal InputMode = iota
	InputModeInsert
)

// Application represents the main TUI application
type Application struct {
	ctx            context.Context
	sessionManager *claude.SessionManager
	eventBus       *EventBus
	eventProcessor *EventProcessor
	program        *tea.Program

	// UI State
	state  ApplicationState
	width  int
	height int

	// Current data
	currentSession claude.SessionInfo
	sessionStats   claude.SessionStats
	messages       []claude.ConversationMessage
	errors         []ErrorMsg
	toolActivity   []ToolActivityMsg

	// Input handling
	inputBuffer   string
	inputActive   bool
	inputMode     InputMode
	cursorPos     int
	commandBuffer string // For multi-key commands like "cw"

	// Status
	statusMessage string
	isLoading     bool

	// Styles
	styles *Styles

	// Markdown renderer
	markdownRenderer *components.MarkdownRenderer

	// Scrolling state
	scrollPosition int
}

// Styles contains all the styling for the application
type Styles struct {
	App        lipgloss.Style
	Header     lipgloss.Style
	Footer     lipgloss.Style
	MainPanel  lipgloss.Style
	SidePanel  lipgloss.Style
	InputPanel lipgloss.Style
	Message    lipgloss.Style
	Error      lipgloss.Style
	Tool       lipgloss.Style
	Status     lipgloss.Style
	Highlight  lipgloss.Style
}

// NewStyles creates default styles for the application
func NewStyles() *Styles {
	return &Styles{
		App: lipgloss.NewStyle().
			Padding(1),
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Background(lipgloss.Color("235")).
			Padding(0, 1).
			Bold(true),
		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("235")).
			Padding(0, 1),
		MainPanel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(1).
			Margin(0, 1),
		SidePanel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(1).
			Width(30),
		InputPanel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1).
			Margin(1, 0),
		Message: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			MarginBottom(1),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true),
		Tool: lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Italic(true),
		Status: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		Highlight: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true),
	}
}

// NewApplication creates a new TUI application
func NewApplication(
	ctx context.Context,
	sessionManager *claude.SessionManager,
) (*Application, error) {
	eventBus := NewEventBus(ctx)
	eventProcessor := NewEventProcessor(ctx, eventBus)

	// Create markdown renderer with default width
	markdownRenderer, err := components.NewMarkdownRenderer(80)
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown renderer: %w", err)
	}

	app := &Application{
		ctx:              ctx,
		sessionManager:   sessionManager,
		eventBus:         eventBus,
		eventProcessor:   eventProcessor,
		state:            StateMain,
		messages:         make([]claude.ConversationMessage, 0),
		errors:           make([]ErrorMsg, 0),
		toolActivity:     make([]ToolActivityMsg, 0),
		styles:           NewStyles(),
		markdownRenderer: markdownRenderer,
	}

	// Register event bus as event handler for session manager
	sessionManager.AddEventHandler(eventBus)

	return app, nil
}

// SetProgram sets the bubbletea program reference
func (a *Application) SetProgram(program *tea.Program) {
	a.program = program
	a.eventBus.SetProgram(program)
	a.eventProcessor.ProcessEvents(program)
}

// Init initializes the application (bubbletea interface)
func (a *Application) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		func() tea.Msg {
			return StatusMsg{
				Status:  "init",
				Message: "CustomClaude TUI started",
			}
		},
	)
}

// Update handles messages (bubbletea interface)
func (a *Application) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Update markdown renderer width using layout manager constraints
		if a.markdownRenderer != nil {
			lm := components.NewLayoutManager(a.width, a.height)
			constraints := lm.GetConversationConstraints()
			contentWidth := constraints.ConversationWidth - 4 // account for message prefix/padding
			if contentWidth > 20 {
				a.markdownRenderer.UpdateWidth(contentWidth)
			}
		}
		return a, nil

	case tea.KeyMsg:
		return a.handleKeyPress(msg)

	case SessionStateMsg:
		a.currentSession = msg.SessionInfo
		a.sessionStats = msg.Stats
		return a, nil

	case MessageStreamMsg:
		a.messages = append(a.messages, msg.Message)
		// Keep only last 500 messages to prevent memory issues
		if len(a.messages) > 500 {
			a.messages = a.messages[len(a.messages)-500:]
			// Recalculate scroll position after truncation
			a.clampScrollPosition()
		}
		// Auto-scroll to bottom for new messages
		a.scrollToBottomSafe()
		return a, nil

	case ToolActivityMsg:
		a.toolActivity = append(a.toolActivity, msg)
		// Keep only last 10 tool activities
		if len(a.toolActivity) > 10 {
			a.toolActivity = a.toolActivity[len(a.toolActivity)-10:]
		}
		return a, nil

	case ErrorMsg:
		a.errors = append(a.errors, msg)
		// Keep only last 5 errors
		if len(a.errors) > 5 {
			a.errors = a.errors[len(a.errors)-5:]
		}
		return a, nil

	case StatusMsg:
		a.statusMessage = fmt.Sprintf("[%s] %s", msg.Status, msg.Message)
		return a, nil

	case PromptInputMsg:
		return a.handlePromptInput(msg)

	case EventMsg:
		// Handle raw events if needed
		return a, nil

	default:
		return a, nil
	}
}

// handleKeyPress handles keyboard input
func (a *Application) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle insert mode character input first (highest priority)
	if a.inputActive && a.inputMode == InputModeInsert {
		switch msg.String() {
		case "esc":
			a.inputMode = InputModeNormal
			if a.cursorPos > 0 && a.cursorPos >= len(a.inputBuffer) {
				a.cursorPos = len(a.inputBuffer) - 1
			}
			a.commandBuffer = ""
			return a, nil
		case "backspace":
			if a.cursorPos > 0 {
				a.inputBuffer = a.inputBuffer[:a.cursorPos-1] + a.inputBuffer[a.cursorPos:]
				a.cursorPos--
			}
			return a, nil
		case "enter":
			if strings.TrimSpace(a.inputBuffer) != "" {
				prompt := strings.TrimSpace(a.inputBuffer)
				a.inputBuffer = ""
				a.inputActive = false
				a.inputMode = InputModeNormal
				a.cursorPos = 0
				a.isLoading = true

				return a, func() tea.Msg {
					return PromptInputMsg{
						Prompt: prompt,
						Resume: a.sessionManager.CurrentSessionID != "",
					}
				}
			}
			return a, nil
		case "left":
			if a.cursorPos > 0 {
				a.cursorPos--
			}
			return a, nil
		case "right":
			if a.cursorPos < len(a.inputBuffer) {
				a.cursorPos++
			}
			return a, nil
		default:
			// Insert any single character
			if len(msg.String()) == 1 {
				a.insertChar(msg.String())
			}
			return a, nil
		}
	}

	// Handle normal mode and non-input mode keys
	switch msg.String() {
	case "ctrl+c":
		return a, tea.Quit

	case "q":
		if !a.inputActive {
			return a, tea.Quit
		}
		return a, nil

	case "ctrl+n":
		return a, func() tea.Msg {
			a.sessionManager.StartNewConversation()
			return StatusMsg{
				Status:  "session",
				Message: "Started new conversation",
			}
		}

	case "ctrl+h":
		a.state = StateHelp
		return a, nil

	case "ctrl+s":
		a.state = StateSettings
		return a, nil

	case "ctrl+m":
		a.state = StateMain
		return a, nil

	case "enter":
		if !a.inputActive {
			a.inputActive = true
			a.inputMode = InputModeNormal
			a.cursorPos = 0
		}
		return a, nil

	case "esc":
		if a.inputActive {
			a.inputActive = false
			a.inputMode = InputModeNormal
			a.cursorPos = 0
		} else {
			a.state = StateMain
		}
		return a, nil

	// Vim-like input handling
	case "i":
		if a.inputActive && a.inputMode == InputModeNormal {
			a.inputMode = InputModeInsert
			a.commandBuffer = ""
		}
		return a, nil

	case "a":
		if a.inputActive && a.inputMode == InputModeNormal {
			a.inputMode = InputModeInsert
			if a.cursorPos < len(a.inputBuffer) {
				a.cursorPos++
			}
			a.commandBuffer = ""
		}
		return a, nil

	case "A":
		if a.inputActive && a.inputMode == InputModeNormal {
			a.inputMode = InputModeInsert
			a.cursorPos = len(a.inputBuffer)
			a.commandBuffer = ""
		}
		return a, nil

	case "x":
		if a.inputActive && a.inputMode == InputModeNormal && a.cursorPos < len(a.inputBuffer) {
			a.inputBuffer = a.inputBuffer[:a.cursorPos] + a.inputBuffer[a.cursorPos+1:]
			if a.cursorPos >= len(a.inputBuffer) && len(a.inputBuffer) > 0 {
				a.cursorPos = len(a.inputBuffer) - 1
			}
		}
		return a, nil

	case "d":
		if a.inputActive && a.inputMode == InputModeNormal {
			if a.commandBuffer == "d" {
				// dd - delete entire line
				a.inputBuffer = ""
				a.cursorPos = 0
				a.commandBuffer = ""
			} else {
				a.commandBuffer = "d"
			}
		}
		return a, nil

	case "c":
		if a.inputActive && a.inputMode == InputModeNormal {
			if a.commandBuffer == "c" {
				// cc - change entire line
				a.inputBuffer = ""
				a.cursorPos = 0
				a.inputMode = InputModeInsert
				a.commandBuffer = ""
			} else {
				a.commandBuffer = "c"
			}
		}
		return a, nil

	case "w":
		if a.inputActive && a.inputMode == InputModeNormal {
			if a.commandBuffer == "d" {
				// dw - delete word
				a.deleteWord()
				a.commandBuffer = ""
			} else if a.commandBuffer == "c" {
				// cw - change word
				a.deleteWord()
				a.inputMode = InputModeInsert
				a.commandBuffer = ""
			} else {
				// w - move forward by word
				a.moveWordForward()
			}
		}
		return a, nil

	case "b":
		if a.inputActive && a.inputMode == InputModeNormal {
			a.moveWordBackward()
		}
		return a, nil

	case "0":
		if a.inputActive && a.inputMode == InputModeNormal {
			a.cursorPos = 0
		}
		return a, nil

	case "$":
		if a.inputActive && a.inputMode == InputModeNormal {
			if len(a.inputBuffer) > 0 {
				a.cursorPos = len(a.inputBuffer) - 1
			} else {
				a.cursorPos = 0
			}
		}
		return a, nil

	case "left":
		if a.inputActive && a.inputMode == InputModeNormal && a.cursorPos > 0 {
			a.cursorPos--
		}
		return a, nil

	case "right":
		if a.inputActive && a.inputMode == InputModeNormal && a.cursorPos < len(a.inputBuffer)-1 {
			a.cursorPos++
		}
		return a, nil

	case "up":
		if !a.inputActive {
			a.scrollUp()
		}
		return a, nil

	case "k":
		if !a.inputActive {
			a.scrollUp()
		}
		// In normal mode, 'k' doesn't do anything for input (could add up navigation later)
		return a, nil

	case "down":
		if !a.inputActive {
			a.scrollDown()
		}
		return a, nil

	case "j":
		if !a.inputActive {
			a.scrollDown()
		}
		// In normal mode, 'j' doesn't do anything for input (could add down navigation later)
		return a, nil

	case "pgup":
		if !a.inputActive {
			a.scrollPageUp()
		}
		return a, nil

	case "pgdown":
		if !a.inputActive {
			a.scrollPageDown()
		}
		return a, nil

	case "home":
		if !a.inputActive {
			a.scrollToTop()
		}
		return a, nil

	case "end":
		if !a.inputActive {
			a.scrollToBottom()
		}
		return a, nil

	default:
		return a, nil
	}
}

// handlePromptInput processes user prompt input
func (a *Application) handlePromptInput(msg PromptInputMsg) (tea.Model, tea.Cmd) {
	// Add user message to conversation immediately
	userMsg := claude.ConversationMessage{
		ID:        fmt.Sprintf("user_%d", time.Now().UnixNano()),
		Type:      "user",
		Content:   msg.Prompt,
		Timestamp: time.Now(),
		IsError:   false,
	}
	a.messages = append(a.messages, userMsg)

	// Auto-scroll to bottom to show new user message
	a.scrollToBottomSafe()

	return a, tea.Cmd(func() tea.Msg {
		go func() {
			if err := a.sessionManager.ExecuteCommand(a.ctx, msg.Prompt, msg.Resume); err != nil {
				a.program.Send(ErrorMsg{
					Error:   err,
					Context: "command_execution",
				})
			}
		}()

		a.isLoading = false
		return StatusMsg{
			Status:  "command",
			Message: fmt.Sprintf("Executing: %s", msg.Prompt),
		}
	})
}

// View renders the application (bubbletea interface)
func (a *Application) View() string {
	switch a.state {
	case StateHelp:
		return a.renderHelpView()
	case StateSettings:
		return a.renderSettingsView()
	default:
		return a.renderMainView()
	}
}

// renderMainView renders the main conversation view
func (a *Application) renderMainView() string {
	if a.width == 0 || a.height == 0 {
		return "Initializing..."
	}

	// Header
	header := a.styles.Header.
		Width(a.width - 2).
		Render("CustomClaude TUI - Claude CLI Interface")

	// Footer with shortcuts
	footer := a.styles.Footer.
		Width(a.width - 2).
		Render("Ctrl+C/Q: Quit | Ctrl+N: New | Ctrl+H: Help | Enter: Input | Esc: Cancel")

	// Layout calculations via LayoutManager
	lm := components.NewLayoutManager(a.width, a.height)
	dims := lm.CalculatePanelDimensions()

	// Conversation panel: pass inner content height (panel height minus padding/border)
	conversationContent := a.renderConversationPanel(
		dims.ConversationWidth-4,
		max(1, dims.ConversationHeight-4),
	)
	conversationPanel := a.styles.MainPanel.
		Width(dims.ConversationWidth).
		Height(dims.ConversationHeight).
		Render(conversationContent)

	// Side panel with session info (pass inner height like conversation)
	sideContent := a.renderSidePanel(max(1, dims.SidebarHeight-4))
	sidePanel := a.styles.SidePanel.
		Height(dims.SidebarHeight).
		Render(sideContent)

		// Input panel
	inputContent := a.renderInputPanel(a.width - 4)
	inputPanel := a.styles.InputPanel.
		Width(a.width - 2).
		Render(inputContent)

	// Combine panels
	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		conversationPanel,
		sidePanel,
	)

	// Combine all sections
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		mainContent,
		inputPanel,
		footer,
	)
}

// Optional future: hook for layout validation. Currently a no-op to avoid changing behavior.
// func (a *Application) validateLayout() {
//     lm := components.NewLayoutManager(a.width, a.height)
//     _ = lm // Placeholder for validation via lm.ValidatePanelHeights
// }

// renderConversationPanel renders the main conversation area with scrolling
func (a *Application) renderConversationPanel(width, height int) string {
	if len(a.messages) == 0 {
		return a.styles.Status.Render("No messages yet. Press Enter to start a conversation.")
	}

	// First, render ALL messages into lines
	var allLines []string

	for i, msg := range a.messages {
		var formattedMsg string
		switch msg.Type {
		case "assistant":
			// Use markdown renderer for assistant messages
			if a.markdownRenderer != nil {
				if rendered, err := a.markdownRenderer.Render(msg.Content); err == nil {
					// Clean up the rendered output
					rendered = strings.TrimSpace(rendered)
					lines := strings.Split(rendered, "\n")

					// Add emoji prefix to first line only
					if len(lines) > 0 {
						lines[0] = "🤖 " + lines[0]
						for j := 1; j < len(lines); j++ {
							lines[j] = "   " + lines[j] // Indent continuation
						}
					}
					formattedMsg = strings.Join(lines, "\n")
				} else {
					wrappedContent := wordWrap(msg.Content, width-4)
					formattedMsg = a.styles.Message.Render("🤖 " + wrappedContent)
				}
			} else {
				wrappedContent := wordWrap(msg.Content, width-4)
				formattedMsg = a.styles.Message.Render("🤖 " + wrappedContent)
			}
		case "tool_use":
			wrappedContent := wordWrap(msg.Content, width-4)
			formattedMsg = a.styles.Tool.Render("🔧 " + wrappedContent)
		case "user":
			wrappedContent := wordWrap(msg.Content, width-4)
			formattedMsg = a.styles.Highlight.Render("👤 " + wrappedContent)
		default:
			wrappedContent := wordWrap(msg.Content, width-4)
			formattedMsg = a.styles.Message.Render("ℹ️  " + wrappedContent)
		}

		// Split formatted message into individual lines
		msgLines := strings.Split(formattedMsg, "\n")
		allLines = append(allLines, msgLines...)

		// Add spacing between messages (except after last message)
		if i < len(a.messages)-1 {
			allLines = append(allLines, "")
		}
	}

	// Calculate total lines
	totalLines := len(allLines)

	// Ensure minimum height
	if height < 3 {
		return a.styles.Status.Render("Window too small")
	}

	// Always reserve space for scroll indicator to maintain consistent viewport
	scrollIndicatorLines := 2
	contentViewportHeight := height - scrollIndicatorLines

	// Show scroll indicator when needed, but viewport height stays consistent
	needsScrollIndicator := totalLines > contentViewportHeight

	// Ensure scroll position is valid
	if a.scrollPosition < 0 {
		a.scrollPosition = 0
	}

	// Calculate max scroll position
	maxScroll := 0
	if totalLines > contentViewportHeight {
		maxScroll = totalLines - contentViewportHeight
	}

	if a.scrollPosition > maxScroll {
		a.scrollPosition = maxScroll
	}

	// Get the lines to display based on scroll position
	var displayLines []string
	if totalLines <= contentViewportHeight {
		// All content fits, show everything
		displayLines = allLines
	} else {
		// Apply scrolling - take exactly contentViewportHeight lines
		startLine := a.scrollPosition
		endLine := startLine + contentViewportHeight
		if endLine > totalLines {
			endLine = totalLines
		}
		displayLines = allLines[startLine:endLine]
	}

	// Build final content
	var finalContent []string

	// Add the content lines
	finalContent = append(finalContent, displayLines...)

	// Add scroll indicator if needed
	if needsScrollIndicator {
		// Calculate actual displayed range
		// displayStart := a.scrollPosition + 1
		// displayEnd := a.scrollPosition + len(displayLines)

		// scrollInfo := fmt.Sprintf("[Lines %d-%d of %d] ", displayStart, displayEnd, totalLines)

		// if a.scrollPosition == 0 {
		// 	scrollInfo += "↓ scroll down"
		// } else if a.scrollPosition >= maxScroll {
		// 	scrollInfo += "↑ scroll up"
		// } else {
		// 	scrollInfo += "↑↓ scroll"
		// }

		// Pad content to exact height before adding scroll indicator
		for len(finalContent) < contentViewportHeight {
			finalContent = append(finalContent, "")
		}

		// Add separator and scroll indicator
		finalContent = append(finalContent, "")
		// if len(finalContent) < height {
		// 	finalContent = append(finalContent, a.styles.Status.Render(scrollInfo))
		// }
	}

	for len(finalContent) < height {
		finalContent = append(finalContent, "")
	}
	// Safety cap: never exceed allotted height
	if len(finalContent) > height {
		finalContent = finalContent[:height]
	}
	content := strings.Join(finalContent, "\n")

	return content
}

// renderSidePanel renders the side panel with session info
func (a *Application) renderSidePanel(height int) string {
	var content []string

	// Session info
	content = append(content, a.styles.Highlight.Render("Session Info"))

	// Show both session manager and current session info for debugging
	managerSessionID := a.sessionManager.CurrentSessionID
	currentSessionID := a.currentSession.ID

	if managerSessionID != "" {
		content = append(content,
			fmt.Sprintf("Manager ID: %s", truncateString(managerSessionID, 18)),
		)
	}

	if currentSessionID != "" {
		content = append(content,
			fmt.Sprintf("Current ID: %s", truncateString(currentSessionID, 18)),
			fmt.Sprintf("Model: %s", a.currentSession.Model),
			fmt.Sprintf("Turns: %d", a.currentSession.TurnCount),
			fmt.Sprintf("Cost: $%.4f", a.currentSession.TotalCost),
		)
	} else {
		if managerSessionID != "" {
			content = append(content, "Manager has session, UI doesn't")
		} else {
			content = append(content, "No active session")
		}
	}

	content = append(content, "")

	// Token usage
	if a.sessionStats.CumulativeUsage.InputTokens > 0 {
		content = append(content, a.styles.Highlight.Render("Token Usage"))
		content = append(content,
			fmt.Sprintf("Input: %d", a.sessionStats.CumulativeUsage.InputTokens),
			fmt.Sprintf("Output: %d", a.sessionStats.CumulativeUsage.OutputTokens),
			fmt.Sprintf("Cache: %d", a.sessionStats.CumulativeUsage.CacheReadInputTokens),
		)
		content = append(content, "")
	}

	// Recent errors
	if len(a.errors) > 0 {
		content = append(content, a.styles.Error.Render("Recent Errors"))
		for _, err := range a.errors[max(0, len(a.errors)-3):] {
			content = append(
				content,
				a.styles.Error.Render("• "+truncateString(err.Error.Error(), 25)),
			)
		}
		content = append(content, "")
	}

	// Tool activity
	if len(a.toolActivity) > 0 {
		content = append(content, a.styles.Tool.Render("Tool Activity"))
		for _, activity := range a.toolActivity[max(0, len(a.toolActivity)-3):] {
			content = append(
				content,
				a.styles.Tool.Render("• "+truncateString(activity.Activity, 25)),
			)
		}
	}

	// Ensure the side panel content fits exactly the inner height
	if height < 1 {
		height = 1
	}
	if len(content) < height {
		for len(content) < height {
			content = append(content, "")
		}
	} else if len(content) > height {
		content = content[:height]
	}
	return strings.Join(content, "\n")
}

// renderInputPanel renders the input area
func (a *Application) renderInputPanel(width int) string {
	if a.isLoading {
		return a.styles.Status.Render("⏳ Processing...")
	}

	if a.inputActive {
		var modeIndicator string
		var cursor string

		switch a.inputMode {
		case InputModeNormal:
			modeIndicator = "[NORMAL]"
			cursor = "█" // Block cursor for normal mode
		case InputModeInsert:
			modeIndicator = "[INSERT]"
			cursor = "│" // Line cursor for insert mode
		}

		// Show command buffer if in multi-key command
		if a.commandBuffer != "" {
			modeIndicator = fmt.Sprintf("[NORMAL:%s]", a.commandBuffer)
		}

		// Build input line with cursor at correct position
		var inputLine string
		if len(a.inputBuffer) == 0 {
			inputLine = cursor
		} else if a.cursorPos >= len(a.inputBuffer) {
			inputLine = a.inputBuffer + cursor
		} else {
			inputLine = a.inputBuffer[:a.cursorPos] + cursor + a.inputBuffer[a.cursorPos:]
		}

		prompt := fmt.Sprintf("%s > %s", modeIndicator, inputLine)
		return a.styles.Highlight.Render(prompt)
	}

	instruction := "Press Enter to start typing your message..."
	if a.statusMessage != "" {
		instruction = a.statusMessage
	}

	return a.styles.Status.Render(instruction)
}

// renderHelpView renders the help screen
func (a *Application) renderHelpView() string {
	content := []string{
		a.styles.Header.Render("CustomClaude TUI - Help"),
		"",
		a.styles.Highlight.Render("Keyboard Shortcuts:"),
		"  Enter     - Start typing a message",
		"  Ctrl+C/Q  - Quit application",
		"  Ctrl+N    - Start new conversation",
		"  Ctrl+H    - Show this help",
		"  Ctrl+S    - Settings (future)",
		"  Ctrl+M    - Return to main view",
		"  Esc       - Cancel input or return to main",
		"",
		a.styles.Highlight.Render("Vim-like Input Mode:"),
		"  Normal Mode:",
		"    i       - Insert mode at cursor",
		"    a       - Insert mode after cursor",
		"    A       - Insert mode at end of line",
		"    x       - Delete character under cursor",
		"    dd      - Delete entire line",
		"    cw      - Change word (delete and insert)",
		"    cc      - Change entire line",
		"    w       - Move forward by word",
		"    b       - Move backward by word",
		"    0       - Move to beginning of line",
		"    $       - Move to end of line",
		"    ←/→     - Move cursor left/right",
		"  Insert Mode:",
		"    Esc     - Return to normal mode",
		"    Enter   - Send message (if not empty)",
		"    Backspace - Delete previous character",
		"",
		a.styles.Highlight.Render("Scrolling:"),
		"  ↑/↓ or j/k  - Scroll up/down one line (when not in input)",
		"  PgUp/PgDn   - Scroll page up/down",
		"  Home/End    - Jump to top/bottom",
		"",
		a.styles.Highlight.Render("Features:"),
		"  • Real-time streaming from Claude",
		"  • Session management and statistics",
		"  • Tool execution monitoring",
		"  • Token usage tracking",
		"  • Error handling and display",
		"  • Markdown rendering for responses",
		"  • Full scrollback with 500 message history",
		"",
		"Press Ctrl+M or Esc to return to main view",
	}

	return a.styles.App.Render(strings.Join(content, "\n"))
}

// renderSettingsView renders the settings screen (placeholder)
func (a *Application) renderSettingsView() string {
	content := []string{
		a.styles.Header.Render("CustomClaude TUI - Settings"),
		"",
		"Settings panel coming soon...",
		"",
		"Press Ctrl+M or Esc to return to main view",
	}

	return a.styles.App.Render(strings.Join(content, "\n"))
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func wordWrap(text string, width int) string {
	if len(text) <= width {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var result []string
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len()+len(word)+1 > width {
			if currentLine.Len() > 0 {
				result = append(result, currentLine.String())
				currentLine.Reset()
			}
		}

		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}

	if currentLine.Len() > 0 {
		result = append(result, currentLine.String())
	}

	return strings.Join(result, "\n")
}

// Helper methods for safe scrolling
func (a *Application) calculateMaxScrollPosition() int {
	// Use LayoutManager to match rendered widths/heights
	lm := components.NewLayoutManager(a.width, a.height)
	dims := lm.CalculatePanelDimensions()
	constraints := lm.GetConversationConstraints()

	// Match wrapping used in renderConversationPanel for non-markdown content
	wrapBaseWidth := dims.ConversationWidth - 4
	if wrapBaseWidth < 1 {
		wrapBaseWidth = 1
	}
	wrapWidth := wrapBaseWidth - 4
	if wrapWidth < 1 {
		wrapWidth = 1
	}

	// Calculate total lines from all messages using same logic as renderConversationPanel
	var allLines []string
	for i, msg := range a.messages {
		var formattedMsg string
		switch msg.Type {
		case "assistant":
			if a.markdownRenderer != nil {
				if rendered, err := a.markdownRenderer.Render(msg.Content); err == nil {
					rendered = strings.TrimSpace(rendered)
					lines := strings.Split(rendered, "\n")
					if len(lines) > 0 {
						lines[0] = "🤖 " + lines[0]
						for j := 1; j < len(lines); j++ {
							lines[j] = "   " + lines[j]
						}
					}
					formattedMsg = strings.Join(lines, "\n")
				} else {
					wrapped := wordWrap(msg.Content, wrapWidth)
					formattedMsg = "🤖 " + wrapped
				}
			} else {
				wrapped := wordWrap(msg.Content, wrapWidth)
				formattedMsg = "🤖 " + wrapped
			}
		case "tool_use":
			wrapped := wordWrap(msg.Content, wrapWidth)
			formattedMsg = "🔧 " + wrapped
		case "user":
			wrapped := wordWrap(msg.Content, wrapWidth)
			formattedMsg = "👤 " + wrapped
		default:
			wrapped := wordWrap(msg.Content, wrapWidth)
			formattedMsg = "ℹ️  " + wrapped
		}
		msgLines := strings.Split(formattedMsg, "\n")
		allLines = append(allLines, msgLines...)
		if i < len(a.messages)-1 {
			allLines = append(allLines, "")
		}
	}

	totalLines := len(allLines)

	viewportHeight := constraints.ViewportHeight
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	maxScroll := totalLines - viewportHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

func (a *Application) clampScrollPosition() {
	if a.scrollPosition < 0 {
		a.scrollPosition = 0
	}
	maxScroll := a.calculateMaxScrollPosition()
	if a.scrollPosition > maxScroll {
		a.scrollPosition = maxScroll
	}
}

func (a *Application) scrollToBottomSafe() {
	a.scrollPosition = a.calculateMaxScrollPosition()
}

// Scrolling methods
func (a *Application) scrollUp() {
	if a.scrollPosition > 0 {
		a.scrollPosition--
	}
}

func (a *Application) scrollDown() {
	maxScroll := a.calculateMaxScrollPosition()
	if a.scrollPosition < maxScroll {
		a.scrollPosition++
	}
}

func (a *Application) scrollPageUp() {
	lm := components.NewLayoutManager(a.width, a.height)
	dims := lm.GetConversationConstraints()

	// Calculate viewport height the same way as renderConversationPanel
	height := max(1, dims.ConversationHeight-4)
	scrollIndicatorLines := 2
	viewport := height - scrollIndicatorLines

	if viewport < 1 {
		viewport = 1
	}
	a.scrollPosition -= viewport
	a.clampScrollPosition()
}

func (a *Application) scrollPageDown() {
	lm := components.NewLayoutManager(a.width, a.height)
	dims := lm.GetConversationConstraints()

	// Calculate viewport height the same way as renderConversationPanel
	height := max(1, dims.ConversationHeight-4)
	scrollIndicatorLines := 2
	viewport := height - scrollIndicatorLines

	if viewport < 1 {
		viewport = 1
	}
	a.scrollPosition += viewport
	a.clampScrollPosition()
}

func (a *Application) scrollToTop() {
	a.scrollPosition = 0
}

func (a *Application) scrollToBottom() {
	a.scrollToBottomSafe()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Vim-like input helper methods

// insertChar inserts a character at the current cursor position
func (a *Application) insertChar(char string) {
	if a.cursorPos >= len(a.inputBuffer) {
		a.inputBuffer += char
		a.cursorPos = len(a.inputBuffer)
	} else {
		a.inputBuffer = a.inputBuffer[:a.cursorPos] + char + a.inputBuffer[a.cursorPos:]
		a.cursorPos++
	}
}

// moveWordForward moves cursor to start of next word
func (a *Application) moveWordForward() {
	if a.cursorPos >= len(a.inputBuffer) {
		return
	}

	// Skip current word
	for a.cursorPos < len(a.inputBuffer) && a.inputBuffer[a.cursorPos] != ' ' {
		a.cursorPos++
	}

	// Skip spaces
	for a.cursorPos < len(a.inputBuffer) && a.inputBuffer[a.cursorPos] == ' ' {
		a.cursorPos++
	}

	if a.cursorPos >= len(a.inputBuffer) && len(a.inputBuffer) > 0 {
		a.cursorPos = len(a.inputBuffer) - 1
	}
}

// moveWordBackward moves cursor to start of previous word
func (a *Application) moveWordBackward() {
	if a.cursorPos <= 0 {
		return
	}

	// Move back one position
	a.cursorPos--

	// Skip spaces
	for a.cursorPos > 0 && a.inputBuffer[a.cursorPos] == ' ' {
		a.cursorPos--
	}

	// Skip to start of word
	for a.cursorPos > 0 && a.inputBuffer[a.cursorPos-1] != ' ' {
		a.cursorPos--
	}
}

// deleteWord deletes the word at cursor position
func (a *Application) deleteWord() {
	if a.cursorPos >= len(a.inputBuffer) {
		return
	}

	startPos := a.cursorPos

	// Find end of word
	for a.cursorPos < len(a.inputBuffer) && a.inputBuffer[a.cursorPos] != ' ' {
		a.cursorPos++
	}

	// Include trailing space if it exists
	if a.cursorPos < len(a.inputBuffer) && a.inputBuffer[a.cursorPos] == ' ' {
		a.cursorPos++
	}

	// Delete the word
	a.inputBuffer = a.inputBuffer[:startPos] + a.inputBuffer[a.cursorPos:]
	a.cursorPos = startPos

	// Adjust cursor if at end
	if a.cursorPos >= len(a.inputBuffer) && len(a.inputBuffer) > 0 {
		a.cursorPos = len(a.inputBuffer) - 1
	}
}
