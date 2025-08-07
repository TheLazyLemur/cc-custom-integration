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
					fmt.Printf("\n[System] Session initialized: %s\n", init.SessionID)
					fmt.Printf("[System] Model: %s\n", init.Model)
					fmt.Printf("[System] Working directory: %s\n", init.CWD)
					fmt.Printf("[System] Available tools: %d\n\n", len(init.Tools))
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
								fmt.Print(text)
							}
						} else if item["type"] == "tool_use" {
							fmt.Printf("\n[Tool: %s]\n", item["name"])
						}
					}
				}
				
				if assistantData.Message.StopReason == "end_turn" {
					fmt.Println()
				}
			}

		case "user":
			// Tool results - just indicate completion
			fmt.Print(".")

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
				fmt.Println()
			} else if msg.IsError {
				fmt.Printf("\n[Error] %s\n", msg.Result)
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
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CONVERSATION SUMMARY")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Duration: %s\n", duration.Round(time.Second))
	fmt.Printf("Sessions: %d\n", len(sm.SessionChain))
	fmt.Printf("Total Turns: %d\n", sm.CumulativeTurns)
	fmt.Printf("Total Cost: $%.6f\n", sm.CumulativeCost)
	fmt.Println()
	fmt.Println("Token Usage:")
	fmt.Printf("  Input Tokens: %d\n", sm.CumulativeUsage.InputTokens)
	fmt.Printf("  Cache Creation: %d\n", sm.CumulativeUsage.CacheCreationInputTokens)
	fmt.Printf("  Cache Read: %d\n", sm.CumulativeUsage.CacheReadInputTokens)
	fmt.Printf("  Output Tokens: %d\n", sm.CumulativeUsage.OutputTokens)
	fmt.Printf("  Total Tokens: %d\n", 
		sm.CumulativeUsage.InputTokens+
		sm.CumulativeUsage.CacheCreationInputTokens+
		sm.CumulativeUsage.CacheReadInputTokens+
		sm.CumulativeUsage.OutputTokens)
	
	if len(sm.SessionChain) > 1 {
		fmt.Println("\nSession Chain:")
		for i, sessionID := range sm.SessionChain {
			fmt.Printf("  %d. %s\n", i+1, sessionID)
		}
	}
	fmt.Println(strings.Repeat("=", 60))
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
	
	fmt.Println("\nStarting new conversation...")
}

func main() {
	sm := &SessionManager{
		ConversationStart: time.Now(),
	}
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Claude CLI Integration")
	fmt.Println("======================")
	fmt.Println("Commands:")
	fmt.Println("  /new     - Start a new conversation")
	fmt.Println("  /model   - Set model (e.g., claude-sonnet-4-20250514)")
	fmt.Println("  /session - Show current session ID")
	fmt.Println("  /exit    - Exit the program")
	fmt.Println("\nType your prompt and press Enter to send to Claude.")
	fmt.Println()

	for {
		fmt.Print("> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		switch {
		case input == "/exit":
			sm.ShowConversationSummary()
			fmt.Println("Goodbye!")
			return

		case input == "/new":
			sm.StartNewConversation()
			continue

		case input == "/session":
			if sm.CurrentSessionID == "" {
				fmt.Println("No active session")
			} else {
				fmt.Printf("Current session: %s\n", sm.CurrentSessionID)
			}
			continue

		case strings.HasPrefix(input, "/model "):
			model := strings.TrimPrefix(input, "/model ")
			sm.Model = model
			fmt.Printf("Model set to: %s\n", model)
			continue

		case strings.HasPrefix(input, "/"):
			fmt.Printf("Unknown command: %s\n", input)
			continue

		default:
			resume := sm.CurrentSessionID != ""
			if err := sm.ExecuteCommand(input, resume); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		}
	}
}