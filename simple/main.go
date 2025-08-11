package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type Message struct {
	Type        string          `json:"type"`
	Subtype     string          `json:"subtype,omitempty"`
	Message     json.RawMessage `json:"message,omitempty"`
	SessionID   string          `json:"session_id,omitempty"`
	IsError     bool            `json:"is_error,omitempty"`
	Result      string          `json:"result,omitempty"`
	DurationMs  int             `json:"duration_ms,omitempty"`
	NumTurns    int             `json:"num_turns,omitempty"`
	TotalCostUSD float64        `json:"total_cost_usd,omitempty"`
	Usage       *Usage          `json:"usage,omitempty"`
}

type Usage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

type AssistantMessage struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Role        string          `json:"role"`
	Model       string          `json:"model"`
	Content     json.RawMessage `json:"content"`
	StopReason  string          `json:"stop_reason"`
}

type SystemInit struct {
	CWD       string   `json:"cwd"`
	SessionID string   `json:"session_id"`
	Tools     []string `json:"tools"`
	Model     string   `json:"model"`
}

type SessionManager struct {
	CurrentSessionID     string
	Model               string
	SessionChain        []string
	CumulativeDuration  int
	CumulativeTurns     int
	CumulativeCost      float64
	CumulativeUsage     Usage
	ConversationStart   time.Time
	markdownRenderer    *glamour.TermRenderer
	systemInitShown     bool
}

var (
	// Color scheme
	primaryColor   = lipgloss.Color("#646CFF")
	successColor   = lipgloss.Color("#00D787")
	warningColor   = lipgloss.Color("#FF8700")
	errorColor     = lipgloss.Color("#FF5F87")
	mutedColor     = lipgloss.Color("#6B7280")
	backgroundFade = lipgloss.Color("#F8FAFC")

	// Styles
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		PaddingTop(1).
		PaddingBottom(1)

	subtitleStyle = lipgloss.NewStyle().
		Foreground(mutedColor).
		Italic(true)

	commandStyle = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	helpStyle = lipgloss.NewStyle().
		Foreground(mutedColor).
		PaddingLeft(2)

	systemStyle = lipgloss.NewStyle().
		Foreground(successColor).
		Bold(true)

	errorStyle = lipgloss.NewStyle().
		Foreground(errorColor).
		Bold(true)

	toolStyle = lipgloss.NewStyle().
		Foreground(warningColor).
		Bold(true)

	promptStyle = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	summaryHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Background(backgroundFade).
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1)

	summaryStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2)

	metricStyle = lipgloss.NewStyle().
		Foreground(mutedColor)

	valueStyle = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	// Additional subtle styles
	headerDivider = lipgloss.NewStyle().
		Foreground(mutedColor).
		Faint(true)

	successIndicator = lipgloss.NewStyle().
		Foreground(successColor).
		Bold(true)

	progressDot = lipgloss.NewStyle().
		Foreground(primaryColor)
)

func newMarkdownRenderer() *glamour.TermRenderer {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		// Fallback to basic renderer if auto-style fails
		r, _ = glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(80),
		)
	}
	return r
}

func (sm *SessionManager) renderMarkdown(content string) string {
	if sm.markdownRenderer == nil {
		return content // Fallback to plain text
	}
	
	rendered, err := sm.markdownRenderer.Render(content)
	if err != nil {
		return content // Fallback to plain text on error
	}
	return strings.TrimSuffix(rendered, "\n") // Remove trailing newline
}

func (sm *SessionManager) ExecuteCommand(prompt string, resume bool) error {
	args := []string{
		"--output-format", "stream-json",
		"--verbose",
		"-p",
		"--permission-prompt-tool", "mcp__permission__approval_prompt",
		"--mcp-config", "config.json",
	}

	if sm.Model != "" {
		args = append(args, "--model", sm.Model)
	}

	if resume && sm.CurrentSessionID != "" {
		args = append(args, "--resume", sm.CurrentSessionID)
	}

	args = append(args, prompt)

	cmd := exec.Command("claude", args...)
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Fprintf(os.Stderr, "[stderr] %s\n", scanner.Text())
		}
	}()

	if err := sm.ProcessStream(stdout); err != nil {
		return fmt.Errorf("failed to process stream: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

func (sm *SessionManager) ProcessStream(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var msg Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			fmt.Printf("[parse error] %s\n", line)
			continue
		}

		switch msg.Type {
		case "system":
			if msg.Subtype == "init" {
				var init SystemInit
				if err := json.Unmarshal([]byte(line), &init); err == nil {
					sm.CurrentSessionID = init.SessionID
					sm.Model = init.Model
					if !sm.systemInitShown {
						fmt.Printf("\n%s Session initialized: %s\n", 
							systemStyle.Render("⚡ [System]"), 
							valueStyle.Render(init.SessionID))
						fmt.Printf("%s Model: %s\n", 
							systemStyle.Render("🤖 [System]"), 
							valueStyle.Render(init.Model))
						fmt.Printf("%s Working directory: %s\n", 
							systemStyle.Render("📁 [System]"), 
							valueStyle.Render(init.CWD))
						fmt.Printf("%s Available tools: %s\n\n", 
							systemStyle.Render("🛠️ [System]"), 
							valueStyle.Render(fmt.Sprintf("%d", len(init.Tools))))
						sm.systemInitShown = true
					}
				}
			}

		case "assistant":
			var assistantData struct {
				Message AssistantMessage `json:"message"`
			}
			if err := json.Unmarshal([]byte(line), &assistantData); err == nil {
				var content []map[string]interface{}
				if err := json.Unmarshal(assistantData.Message.Content, &content); err == nil {
					for _, item := range content {
						if item["type"] == "text" {
							if text, ok := item["text"].(string); ok {
								rendered := sm.renderMarkdown(text)
								fmt.Print(rendered)
							}
						} else if item["type"] == "tool_use" {
							fmt.Printf("\n%s\n", toolStyle.Render(fmt.Sprintf("🔧 [Tool: %s]", item["name"])))
						}
					}
				}
				
				if assistantData.Message.StopReason == "end_turn" {
					fmt.Println()
				}
			}

		case "user":
			// Tool results - just indicate completion
			fmt.Print(progressDot.Render("•"))

		case "result":
			if msg.Subtype == "success" {
				sm.CurrentSessionID = msg.SessionID
				sm.SessionChain = append(sm.SessionChain, msg.SessionID)
				
				// Accumulate session data
				sm.CumulativeDuration += msg.DurationMs
				sm.CumulativeTurns += msg.NumTurns
				sm.CumulativeCost += msg.TotalCostUSD
				
				if msg.Usage != nil {
					sm.CumulativeUsage.InputTokens += msg.Usage.InputTokens
					sm.CumulativeUsage.CacheCreationInputTokens += msg.Usage.CacheCreationInputTokens
					sm.CumulativeUsage.CacheReadInputTokens += msg.Usage.CacheReadInputTokens
					sm.CumulativeUsage.OutputTokens += msg.Usage.OutputTokens
				}
				
				// Just show a completion indicator, not full session info
				fmt.Print(" ")
				fmt.Print(successIndicator.Render(""))
				fmt.Print("\n")
			} else if msg.IsError {
				fmt.Printf("\n%s %s\n", errorStyle.Render("❌ [Error]"), msg.Result)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

func (sm *SessionManager) ShowConversationSummary() {
	if len(sm.SessionChain) == 0 {
		return
	}

	duration := time.Since(sm.ConversationStart)
	
	// Header
	fmt.Print("\n")
	fmt.Print(summaryHeaderStyle.Render("CONVERSATION SUMMARY"))
	fmt.Print("\n")
	
	// Main stats
	var summaryContent strings.Builder
	summaryContent.WriteString(fmt.Sprintf("%s %s\n", 
		metricStyle.Render("Duration:"), 
		valueStyle.Render(duration.Round(time.Second).String())))
	summaryContent.WriteString(fmt.Sprintf("%s %s\n", 
		metricStyle.Render("Sessions:"), 
		valueStyle.Render(fmt.Sprintf("%d", len(sm.SessionChain)))))
	summaryContent.WriteString(fmt.Sprintf("%s %s\n", 
		metricStyle.Render("Total Turns:"), 
		valueStyle.Render(fmt.Sprintf("%d", sm.CumulativeTurns))))
	summaryContent.WriteString(fmt.Sprintf("%s %s\n\n", 
		metricStyle.Render("Total Cost:"), 
		valueStyle.Render(fmt.Sprintf("$%.6f", sm.CumulativeCost))))
	
	// Token usage
	summaryContent.WriteString(fmt.Sprintf("%s\n", 
		commandStyle.Render("Token Usage:")))
	summaryContent.WriteString(fmt.Sprintf("  %s %s\n", 
		metricStyle.Render("Input Tokens:"), 
		valueStyle.Render(fmt.Sprintf("%d", sm.CumulativeUsage.InputTokens))))
	summaryContent.WriteString(fmt.Sprintf("  %s %s\n", 
		metricStyle.Render("Cache Creation:"), 
		valueStyle.Render(fmt.Sprintf("%d", sm.CumulativeUsage.CacheCreationInputTokens))))
	summaryContent.WriteString(fmt.Sprintf("  %s %s\n", 
		metricStyle.Render("Cache Read:"), 
		valueStyle.Render(fmt.Sprintf("%d", sm.CumulativeUsage.CacheReadInputTokens))))
	summaryContent.WriteString(fmt.Sprintf("  %s %s\n", 
		metricStyle.Render("Output Tokens:"), 
		valueStyle.Render(fmt.Sprintf("%d", sm.CumulativeUsage.OutputTokens))))
	
	totalTokens := sm.CumulativeUsage.InputTokens +
		sm.CumulativeUsage.CacheCreationInputTokens +
		sm.CumulativeUsage.CacheReadInputTokens +
		sm.CumulativeUsage.OutputTokens
	summaryContent.WriteString(fmt.Sprintf("  %s %s", 
		metricStyle.Render("Total Tokens:"), 
		valueStyle.Render(fmt.Sprintf("%d", totalTokens))))
	
	if len(sm.SessionChain) > 1 {
		summaryContent.WriteString(fmt.Sprintf("\n\n%s\n", 
			commandStyle.Render("Session Chain:")))
		for i, sessionID := range sm.SessionChain {
			summaryContent.WriteString(fmt.Sprintf("  %s %s\n", 
				metricStyle.Render(fmt.Sprintf("%d.", i+1)), 
				valueStyle.Render(sessionID)))
		}
	}
	
	fmt.Print(summaryStyle.Render(summaryContent.String()))
	fmt.Print("\n")
}

func (sm *SessionManager) StartNewConversation() {
	if len(sm.SessionChain) > 0 {
		sm.ShowConversationSummary()
	}
	
	// Reset for new conversation
	sm.CurrentSessionID = ""
	sm.SessionChain = nil
	sm.CumulativeDuration = 0
	sm.CumulativeTurns = 0
	sm.CumulativeCost = 0
	sm.CumulativeUsage = Usage{}
	sm.ConversationStart = time.Now()
	sm.systemInitShown = false
	
	fmt.Print("\n")
	fmt.Print(systemStyle.Render("🆕 [System]"))
	fmt.Print(" ")
	fmt.Print(subtitleStyle.Render("Starting new conversation..."))
	fmt.Print("\n")
}

func main() {
	sm := &SessionManager{
		ConversationStart:   time.Now(),
		markdownRenderer:    newMarkdownRenderer(),
	}
	reader := bufio.NewReader(os.Stdin)

	fmt.Print(titleStyle.Render("Claude CLI Integration"))
	fmt.Print("\n")
	fmt.Print(subtitleStyle.Render("Interactive Claude CLI with session management"))
	fmt.Print("\n")
	fmt.Print(headerDivider.Render("────────────────────────────────────────"))
	fmt.Print("\n\n")
	
	fmt.Print(commandStyle.Render("Commands:"))
	fmt.Print("\n")
	fmt.Print(helpStyle.Render("  /new     - Start a new conversation"))
	fmt.Print("\n")
	fmt.Print(helpStyle.Render("  /model   - Set model (e.g., claude-sonnet-4-20250514)"))
	fmt.Print("\n")
	fmt.Print(helpStyle.Render("  /session - Show current session ID"))
	fmt.Print("\n")
	fmt.Print(helpStyle.Render("  /exit    - Exit the program"))
	fmt.Print("\n\n")
	fmt.Print(headerDivider.Render("────────────────────────────────────────"))
	fmt.Print("\n")
	fmt.Print(subtitleStyle.Render("Type your prompt and press Enter to send to Claude."))
	fmt.Print("\n\n")

	for {
		fmt.Print(promptStyle.Render("> "))
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("%s Error reading input: %v\n", errorStyle.Render("❌ [Error]"), err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		switch {
		case input == "/exit":
			sm.ShowConversationSummary()
			fmt.Print(subtitleStyle.Render("Goodbye!"))
			fmt.Print("\n")
			return

		case input == "/new":
			sm.StartNewConversation()
			continue

		case input == "/session":
			if sm.CurrentSessionID == "" {
				fmt.Print(subtitleStyle.Render("No active session"))
				fmt.Print("\n")
			} else {
				fmt.Printf("%s %s\n", 
					metricStyle.Render("Current session:"), 
					valueStyle.Render(sm.CurrentSessionID))
			}
			continue

		case strings.HasPrefix(input, "/model "):
			model := strings.TrimPrefix(input, "/model ")
			sm.Model = model
			fmt.Printf("%s %s\n", 
				metricStyle.Render("Model set to:"), 
				valueStyle.Render(model))
			continue

		case strings.HasPrefix(input, "/"):
			fmt.Printf("%s Unknown command: %s\n", 
				errorStyle.Render("❌ [Error]"), 
				input)
			continue

		default:
			resume := sm.CurrentSessionID != ""
			if err := sm.ExecuteCommand(input, resume); err != nil {
				fmt.Printf("%s %v\n", errorStyle.Render("❌ [Error]"), err)
			}
		}
	}
}