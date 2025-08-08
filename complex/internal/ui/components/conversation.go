package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"complex/internal/claude"
)

// ConversationComponent handles the display of conversation messages
type ConversationComponent struct {
	messages  []claude.ConversationMessage
	width     int
	height    int
	scrollPos int
	styles    *ConversationStyles
}

// ConversationStyles contains styling for conversation display
type ConversationStyles struct {
	Container        lipgloss.Style
	UserMessage      lipgloss.Style
	AssistantMessage lipgloss.Style
	SystemMessage    lipgloss.Style
	ToolMessage      lipgloss.Style
	ErrorMessage     lipgloss.Style
	Timestamp        lipgloss.Style
	Divider          lipgloss.Style
}

// NewConversationStyles creates default conversation styles
func NewConversationStyles() *ConversationStyles {
	return &ConversationStyles{
		Container: lipgloss.NewStyle().
			Padding(1),
		UserMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color("75")). // Light blue
			MarginBottom(1).
			Padding(0, 1),
		AssistantMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")). // Light gray
			MarginBottom(1).
			Padding(0, 1),
		SystemMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")). // Gray
			MarginBottom(1).
			Padding(0, 1).
			Italic(true),
		ToolMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")). // Purple
			MarginBottom(1).
			Padding(0, 1).
			Italic(true),
		ErrorMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")). // Red
			MarginBottom(1).
			Padding(0, 1).
			Bold(true),
		Timestamp: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")). // Dark gray
			Faint(true),
		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("238")). // Very dark gray
			MarginTop(1).
			MarginBottom(1),
	}
}

// NewConversationComponent creates a new conversation component
func NewConversationComponent() *ConversationComponent {
	return &ConversationComponent{
		messages: make([]claude.ConversationMessage, 0),
		styles:   NewConversationStyles(),
	}
}

// SetDimensions sets the width and height for the component
func (cc *ConversationComponent) SetDimensions(width, height int) {
	cc.width = width
	cc.height = height
}

// AddMessage adds a new message to the conversation
func (cc *ConversationComponent) AddMessage(message claude.ConversationMessage) {
	cc.messages = append(cc.messages, message)

	// Auto-scroll to bottom when new message is added
	cc.ScrollToBottom()

	// Limit message history to prevent memory issues
	if len(cc.messages) > 1000 {
		cc.messages = cc.messages[len(cc.messages)-1000:]
	}
}

// SetMessages sets the entire message list
func (cc *ConversationComponent) SetMessages(messages []claude.ConversationMessage) {
	cc.messages = append([]claude.ConversationMessage(nil), messages...)
	cc.ScrollToBottom()
}

// GetMessages returns the current messages
func (cc *ConversationComponent) GetMessages() []claude.ConversationMessage {
	return append([]claude.ConversationMessage(nil), cc.messages...)
}

// ScrollUp scrolls up by one line
func (cc *ConversationComponent) ScrollUp() {
	if cc.scrollPos > 0 {
		cc.scrollPos--
	}
}

// ScrollDown scrolls down by one line
func (cc *ConversationComponent) ScrollDown() {
	maxScroll := cc.getMaxScrollPosition()
	if cc.scrollPos < maxScroll {
		cc.scrollPos++
	}
}

// ScrollPageUp scrolls up by one page
func (cc *ConversationComponent) ScrollPageUp() {
	pageSize := cc.height - 2
	cc.scrollPos = max(0, cc.scrollPos-pageSize)
}

// ScrollPageDown scrolls down by one page
func (cc *ConversationComponent) ScrollPageDown() {
	pageSize := cc.height - 2
	maxScroll := cc.getMaxScrollPosition()
	cc.scrollPos = min(maxScroll, cc.scrollPos+pageSize)
}

// ScrollToTop scrolls to the top of the conversation
func (cc *ConversationComponent) ScrollToTop() {
	cc.scrollPos = 0
}

// ScrollToBottom scrolls to the bottom of the conversation
func (cc *ConversationComponent) ScrollToBottom() {
	cc.scrollPos = cc.getMaxScrollPosition()
}

// getMaxScrollPosition calculates the maximum scroll position
func (cc *ConversationComponent) getMaxScrollPosition() int {
	if cc.height == 0 {
		return 0
	}

	totalLines := cc.getTotalLines()
	visibleLines := cc.height - 2 // Account for padding

	maxScroll := totalLines - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	return maxScroll
}

// getTotalLines calculates total lines needed for all messages
func (cc *ConversationComponent) getTotalLines() int {
	if cc.width == 0 {
		return 0
	}

	totalLines := 0
	contentWidth := cc.width - 4 // Account for padding and borders

	for _, msg := range cc.messages {
		lines := cc.calculateMessageLines(msg, contentWidth)
		totalLines += lines + 1 // +1 for spacing between messages
	}

	return totalLines
}

// calculateMessageLines calculates how many lines a message will take
func (cc *ConversationComponent) calculateMessageLines(
	msg claude.ConversationMessage,
	width int,
) int {
	if width <= 0 {
		return 1
	}

	// Header line (user/assistant indicator + timestamp)
	lines := 1

	// Content lines
	content := msg.Content
	contentLines := strings.Split(wordWrap(content, width-2), "\n") // -2 for message prefix
	lines += len(contentLines)

	return lines
}

// Render renders the conversation component
func (cc *ConversationComponent) Render() string {
	if cc.width == 0 || cc.height == 0 {
		return "Conversation loading..."
	}

	if len(cc.messages) == 0 {
		emptyMsg := cc.styles.SystemMessage.Render(
			"No messages yet. Start a conversation to see messages here.",
		)
		return cc.styles.Container.
			Width(cc.width).
			Height(cc.height).
			Render(emptyMsg)
	}

	content := cc.renderVisibleMessages()

	return cc.styles.Container.
		Width(cc.width).
		Height(cc.height).
		Render(content)
}

func (cc *ConversationComponent) renderVisibleMessages() string {
	if len(cc.messages) == 0 {
		return ""
	}

	contentWidth := cc.width - 4  // Account for padding
	visibleLines := cc.height - 2 // Number of lines to show

	var allLines []string

	for _, msg := range cc.messages {
		msgLines := cc.renderMessage(msg, contentWidth)
		lines := strings.Split(msgLines, "\n")
		allLines = append(allLines, lines...)
		allLines = append(allLines, "") // spacing between messages
	}

	// Clamp scroll position
	if cc.scrollPos > len(allLines)-visibleLines {
		cc.scrollPos = max(0, len(allLines)-visibleLines)
	}

	end := cc.scrollPos + visibleLines
	if end > len(allLines) {
		end = len(allLines)
	}

	visible := allLines[cc.scrollPos:end]
	return strings.Join(visible, "\n")
}

// renderVisibleMessages renders only the messages that should be visible
// func (cc *ConversationComponent) renderVisibleMessages() string {
// 	if len(cc.messages) == 0 {
// 		return ""
// 	}
//
// 	contentWidth := cc.width - 4 // Account for padding
// 	visibleLines := cc.height - 2 // Account for container padding
//
// 	var renderedLines []string
// 	currentLine := 0
//
// 	// Calculate which messages to show based on scroll position
// 	for _, msg := range cc.messages {
// 		msgLines := cc.renderMessage(msg, contentWidth)
// 		msgLinesList := strings.Split(msgLines, "\n")
//
// 		for _, line := range msgLinesList {
// 			if currentLine >= cc.scrollPos && len(renderedLines) < visibleLines {
// 				renderedLines = append(renderedLines, line)
// 			}
// 			currentLine++
// 		}
//
// 		// Add spacing between messages
// 		if currentLine >= cc.scrollPos && len(renderedLines) < visibleLines {
// 			renderedLines = append(renderedLines, "")
// 		}
// 		currentLine++
//
// 		// Break if we have enough lines
// 		if len(renderedLines) >= visibleLines {
// 			break
// 		}
// 	}
//
// 	return strings.Join(renderedLines, "\n")
// }

// renderMessage renders a single message
func (cc *ConversationComponent) renderMessage(msg claude.ConversationMessage, width int) string {
	var style lipgloss.Style
	var prefix string
	var icon string

	// Determine style and prefix based on message type
	switch msg.Type {
	case "user":
		style = cc.styles.UserMessage
		prefix = "You"
		icon = "👤"
	case "assistant":
		style = cc.styles.AssistantMessage
		prefix = "Claude"
		icon = "🤖"
	case "tool_use":
		style = cc.styles.ToolMessage
		prefix = fmt.Sprintf("Tool: %s", msg.ToolName)
		icon = "🔧"
	case "system":
		style = cc.styles.SystemMessage
		prefix = "System"
		icon = "ℹ️"
	default:
		style = cc.styles.SystemMessage
		prefix = "Unknown"
		icon = "❓"
	}

	// Handle error messages
	if msg.IsError {
		style = cc.styles.ErrorMessage
		icon = "❌"
		prefix = "Error"
	}

	// Format timestamp
	timestamp := cc.styles.Timestamp.Render(
		msg.Timestamp.Format("15:04:05"),
	)

	// Create header line
	header := fmt.Sprintf("%s %s %s", icon, prefix, timestamp)

	// Wrap content
	wrappedContent := wordWrap(msg.Content, width-2) // -2 for indentation
	contentLines := strings.Split(wrappedContent, "\n")

	// Indent content lines
	var indentedContent []string
	for _, line := range contentLines {
		indentedContent = append(indentedContent, "  "+line)
	}

	// Combine header and content
	messageContent := header + "\n" + strings.Join(indentedContent, "\n")

	return style.Render(messageContent)
}

// GetScrollInfo returns current scroll information for display
func (cc *ConversationComponent) GetScrollInfo() (current, max int) {
	return cc.scrollPos, cc.getMaxScrollPosition()
}

// IsAtBottom returns true if scrolled to bottom
func (cc *ConversationComponent) IsAtBottom() bool {
	return cc.scrollPos >= cc.getMaxScrollPosition()
}

// IsAtTop returns true if scrolled to top
func (cc *ConversationComponent) IsAtTop() bool {
	return cc.scrollPos <= 0
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// wordWrap wraps text to fit within the specified width
func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}

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
		// If adding this word would exceed width, start new line
		if currentLine.Len()+len(word)+1 > width {
			if currentLine.Len() > 0 {
				result = append(result, currentLine.String())
				currentLine.Reset()
			}
		}

		// Add word to current line
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}

	// Add final line if not empty
	if currentLine.Len() > 0 {
		result = append(result, currentLine.String())
	}

	return strings.Join(result, "\n")
}
