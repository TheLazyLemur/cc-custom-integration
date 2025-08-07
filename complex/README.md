# CustomClaude Complex TUI

A sophisticated Terminal User Interface (TUI) for Claude CLI interaction built with bubbletea and lipgloss.

## Phase 1 Implementation Status ✅

**Phase 1 (Foundation Architecture) - COMPLETED**

- ✅ **SessionManager Extraction**: Extracted from simple CLI with event emission capabilities
- ✅ **Shared Data Structures**: Complete type definitions for all components
- ✅ **Go Module Setup**: Configured with bubbletea v0.24.2 and lipgloss v0.9.1
- ✅ **Basic TUI Application**: Full bubbletea application structure with multi-view support
- ✅ **Core Event System**: Event-driven architecture with async event processing
- ✅ **Conversation Display**: Scrollable conversation component with message formatting

**Phase 1.5 (Enhancements) - COMPLETED**

- ✅ **Markdown Rendering**: Full markdown support for assistant messages using glamour
  - Code blocks with syntax highlighting
  - Lists, headers, and formatting
  - Responsive width adjustment
  - Emoji support
- ✅ **Full Scrollback**: Complete conversation history navigation
  - 500 message history retention
  - Line-by-line and page scrolling
  - Keyboard navigation (arrows, page up/down, home/end)
  - Visual scroll position indicators
  - Auto-scroll on new messages

## Features Implemented

### Core Architecture
- **Event-driven design**: Async event processing with proper goroutine management
- **Clean separation**: Hexagonal architecture with clear boundaries
- **Multi-view support**: Main, Help, and Settings views with keyboard navigation
- **Session management**: Real-time session tracking with statistics

### User Interface
- **Real-time display**: Streaming message display with proper formatting
- **Multi-panel layout**: Conversation, session info, and input panels
- **Responsive design**: Terminal resize handling and adaptive layouts
- **Keyboard navigation**: Full keyboard control with intuitive shortcuts

### Message Display
- **Message types**: Support for user, assistant, tool, system, and error messages
- **Formatting**: Proper word wrapping, timestamps, and visual indicators
- **Markdown rendering**: Rich text formatting for assistant responses
  - Syntax-highlighted code blocks
  - Formatted lists and headers
  - Emoji support
- **Scrolling**: Smooth scrolling with page up/down support
- **History management**: Automatic message pruning to prevent memory issues

### Session Management
- **Real-time stats**: Token usage, cost tracking, and turn counts
- **Session chain**: Visual display of session relationships
- **Error handling**: Comprehensive error display and logging
- **Tool activity**: Real-time tool execution monitoring

## Building and Running

```bash
# Build the application
go build -o complex-tui cmd/main.go

# Run the application
./complex-tui
```

## Keyboard Shortcuts

### Basic Navigation
- **Enter** - Start typing a message
- **Ctrl+C/Q** - Quit application  
- **Ctrl+N** - Start new conversation
- **Ctrl+H** - Show help screen
- **Ctrl+S** - Settings screen (placeholder)
- **Ctrl+M/Esc** - Return to main view or cancel input

### Scrolling
- **↑/↓** or **j/k** - Scroll up/down one line
- **PgUp/PgDn** - Scroll by page
- **Home/End** - Jump to top/bottom of conversation
- Auto-scrolls to bottom when new messages arrive
- Scroll position indicator shows current view and total lines

## Architecture

```
complex/
├── cmd/main.go                    # Application entry point
├── internal/
│   ├── claude/                    # Core Claude integration
│   │   ├── session.go            # Enhanced SessionManager with events
│   │   ├── types.go              # Shared data structures
│   ├── app/                      # Application orchestration  
│   │   ├── app.go               # Main TUI application
│   │   ├── events.go            # Event system and routing
│   └── ui/components/            # UI components
│       ├── conversation.go      # Message display component
│       └── markdown.go          # Markdown rendering wrapper
├── go.mod                        # Dependencies
└── README.md                     # This file
```

## Dependencies

- **bubbletea v0.24.2**: TUI framework
- **lipgloss v0.9.1+**: Terminal styling library
- **glamour v0.10.0**: Markdown rendering with terminal styling
- **Go 1.21+**: Required Go version

## Next Steps

Phase 1 provides the foundation for a fully functional TUI. Next phases will add:

- **Phase 2**: Multi-panel layouts, session tree visualization, async Claude execution
- **Phase 3**: Token usage dashboard, interactive session management, enhanced error handling
- **Phase 4**: Performance optimization, advanced styling, comprehensive testing

## Status

✅ **Phase 1 Complete**: Foundation architecture implemented and tested
🚧 **Phase 2**: Ready to begin multi-panel interface development
⏳ **Phase 3**: Advanced features pending
⏳ **Phase 4**: Polish and optimization pending