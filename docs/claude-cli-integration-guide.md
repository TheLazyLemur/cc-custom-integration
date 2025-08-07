# Claude CLI Integration Guide
## Stream Response Format & Session Management

### Command Structure

```bash
claude [OPTIONS] [PROMPT]

# Full example with stream output
claude \
  --model claude-sonnet-4-20250514 \
  --output-format stream-json \
  --verbose \
  -p \
  --permission-prompt-tool mcp__permission__approval_prompt \
  --mcp-config config.json \
  --resume SESSION_ID \
  "Your prompt here" >> output.json
```

#### Key Options
- `--output-format stream-json`: Returns newline-delimited JSON stream
- `--verbose`: **REQUIRED with stream-json** - Enables full message flow output
- `--resume SESSION_ID`: Continues from previous session (creates NEW session)
- `--model MODEL_NAME`: Specifies Claude model version
- `--mcp-config FILE`: MCP server configuration
- `--permission-prompt-tool TOOL`: Permission handling tool
- `-p`: Prompt mode flag

### Response Stream Format

The CLI outputs newline-delimited JSON objects, each representing a discrete event in the conversation flow.

#### Message Types

##### 1. System Initialization
```json
{
  "type": "system",
  "subtype": "init",
  "cwd": "/private/tmp/CustomClaude",
  "session_id": "f2c0fab9-66f2-49ae-8b20-b2566f10de6a",
  "tools": ["Task", "Bash", "Glob", "Grep", "LS", "Read", "Edit", "Write", ...],
  "mcp_servers": [{"name": "permission", "status": "connected"}],
  "model": "claude-sonnet-4-20250514",
  "permissionMode": "default",
  "slash_commands": ["task_easy", "commit", "code-review", ...],
  "apiKeySource": "none"
}
```

##### 2. Assistant Messages
```json
{
  "type": "assistant",
  "message": {
    "id": "msg_01HUTBuxg8F2NmTFZC2NUTZW",
    "type": "message",
    "role": "assistant",
    "model": "claude-sonnet-4-20250514",
    "content": [
      {
        "type": "text",
        "text": "I'll create a basic API..."
      },
      {
        "type": "tool_use",
        "id": "toolu_013LHPsoMnbQ1KZdhLWDZDJV",
        "name": "TodoWrite",
        "input": {"todos": [...]}
      }
    ],
    "stop_reason": "tool_use" | "end_turn" | null,
    "usage": {
      "input_tokens": 3,
      "cache_creation_input_tokens": 18958,
      "cache_read_input_tokens": 0,
      "output_tokens": 109,
      "service_tier": "standard"
    }
  },
  "parent_tool_use_id": null,
  "session_id": "f2c0fab9-66f2-49ae-8b20-b2566f10de6a"
}
```

##### 3. User Messages (Tool Results)
```json
{
  "type": "user",
  "message": {
    "role": "user",
    "content": [
      {
        "tool_use_id": "toolu_013LHPsoMnbQ1KZdhLWDZDJV",
        "type": "tool_result",
        "content": "Result text or error",
        "is_error": false
      }
    ]
  },
  "parent_tool_use_id": null,
  "session_id": "f2c0fab9-66f2-49ae-8b20-b2566f10de6a"
}
```

##### 4. Error Responses
```json
{
  "type": "tool_result",
  "content": "<tool_use_error>Error message</tool_use_error>",
  "is_error": true,
  "tool_use_id": "toolu_01DDR48QUDmxRWrGSdSifpGA"
}
```

##### 5. Session Result
```json
{
  "type": "result",
  "subtype": "success",
  "is_error": false,
  "duration_ms": 246771,
  "duration_api_ms": 52337,
  "num_turns": 36,
  "result": "Final result text",
  "session_id": "f1c32ff4-7994-418e-9f81-216b4b88bb74",
  "total_cost_usd": 0.10067034999999999,
  "usage": {
    "input_tokens": 49,
    "cache_creation_input_tokens": 2385,
    "cache_read_input_tokens": 212520,
    "output_tokens": 1842,
    "server_tool_use": {"web_search_requests": 0},
    "service_tier": "standard"
  }
}
```

### Critical Session Management Insights

#### ⚠️ Session ID Behavior

**Each CLI invocation creates a NEW session ID**, even when using `--resume`:

```bash
# Initial command
claude "Create a server" 
# → session_id: f2c0fab9-66f2-49ae-8b20-b2566f10de6a

# Resume creates NEW session with loaded context
claude --resume f2c0fab9-66f2-49ae-8b20-b2566f10de6a "Add port flag"
# → NEW session_id: b0a32754-76f1-456a-a5bb-3125a9fa3ed0

# Must use MOST RECENT session for next resume
claude --resume b0a32754-76f1-456a-a5bb-3125a9fa3ed0 "Add logging"
# → NEW session_id: f1c32ff4-7994-418e-9f81-216b4b88bb74
```

**Key Rule**: Always use the session ID from the LAST response's `result` message for continuation.

#### Context Preservation

Context is maintained through a sophisticated caching system:

```json
"usage": {
  "cache_creation_input_tokens": 18958,  // Building context cache
  "cache_read_input_tokens": 212520,     // Loading previous context
  "input_tokens": 49,                    // New input
  "output_tokens": 1842                  // Response
}
```

- Cache grows progressively across sessions
- File system changes persist
- Tool execution history maintained
- Conversation context preserved

### Tool Interaction Pattern

1. **Assistant initiates tool use**:
   - Sends `tool_use` in content array
   - Provides unique `tool_use_id`
   - Includes tool name and input parameters

2. **User returns tool result**:
   - Links via `tool_use_id`
   - Contains execution result or error
   - Errors wrapped in `<tool_use_error>` tags

3. **Error handling flow**:
   - Timeout errors: `"request timed out after 2m0s"`
   - Permission errors: `"no response received"`
   - Network errors: `"fetch failed"`
   - Assistant retries with fallback strategies

### Integration Implementation Guide

#### 1. Stream Parser

```javascript
// Parse newline-delimited JSON stream
const parseStream = (streamData) => {
  return streamData
    .split('\n')
    .filter(line => line.trim())
    .map(line => JSON.parse(line));
};
```

#### 2. Session Manager

```javascript
class SessionManager {
  constructor() {
    this.currentSessionId = null;
    this.sessionChain = [];
  }

  handleMessage(message) {
    if (message.type === 'system' && message.subtype === 'init') {
      this.currentSessionId = message.session_id;
      this.sessionChain.push({
        id: message.session_id,
        timestamp: Date.now(),
        parent: this.currentSessionId
      });
    }
    
    if (message.type === 'result') {
      this.currentSessionId = message.session_id;
    }
  }

  getResumeCommand(prompt) {
    if (!this.currentSessionId) {
      return `claude "${prompt}"`;
    }
    return `claude --resume ${this.currentSessionId} "${prompt}"`;
  }
}
```

#### 3. Message Flow Handler

```javascript
class MessageHandler {
  processMessage(msg) {
    switch(msg.type) {
      case 'system':
        return this.handleSystemInit(msg);
      
      case 'assistant':
        if (msg.message.content.some(c => c.type === 'tool_use')) {
          return this.handleToolRequest(msg);
        }
        return this.handleAssistantText(msg);
      
      case 'user':
        return this.handleToolResult(msg);
      
      case 'result':
        return this.handleSessionResult(msg);
    }
  }
}
```

### UI Component Requirements

#### Essential Components

1. **Session Chain Visualizer**
   - Display session lineage
   - Show context inheritance
   - Highlight current active session

2. **Tool Interaction Viewer**
   - Request → Response pairing
   - Error retry visualization
   - Timing and timeout tracking

3. **Token Usage Monitor**
   ```
   Cache Read:     [████████████████░░░░] 212,520
   Cache Create:   [██░░░░░░░░░░░░░░░░░░]   2,385
   New Input:      [░░░░░░░░░░░░░░░░░░░░]      49
   Output:         [█░░░░░░░░░░░░░░░░░░░]   1,842
   ```

4. **Cost Tracker**
   - Per-session cost
   - Cumulative session chain cost
   - Token efficiency metrics

5. **Error Recovery Display**
   - Show retry attempts
   - Timeout indicators
   - Fallback strategy visualization

### Common Patterns & Pitfalls

#### Pattern: Tool Retry on Timeout
```
1. Edit attempt → Timeout (2m0s)
2. MultiEdit attempt → Timeout (2m0s)
3. Write fallback → Success
```

#### Pattern: File Access Requirements
```
1. Write attempt → Error: "File has not been read yet"
2. Read file → Success
3. Edit file → Success
```

#### Pitfall: Using Old Session IDs
❌ Using original session ID for all resumes loses intermediate context
✅ Always use the most recent session ID from the last result

#### Pitfall: Missing Cache Context
❌ Starting fresh session for related tasks
✅ Use --resume to maintain context and reduce token usage

### Performance Optimization

1. **Cache Utilization**
   - Resume sessions to leverage cache
   - Reduces input token costs significantly
   - Maintains conversation coherence

2. **Batch Tool Operations**
   - Multiple tool calls in single assistant message
   - Reduces round-trip overhead
   - Parallel execution where possible

3. **Error Recovery**
   - Implement exponential backoff for retries
   - Have fallback strategies (Edit → MultiEdit → Write)
   - Track timeout patterns for optimization

### Security Considerations

- Tool permissions managed by MCP servers
- Permission prompts for sensitive operations
- API key source tracking (`apiKeySource` field)
- File system access controls via tool restrictions

### Debugging Tips

1. **Enable verbose mode** for full message flow
2. **Track session chains** to understand context evolution
3. **Monitor cache metrics** for performance analysis
4. **Log tool errors** to identify patterns
5. **Capture full stream** for replay and analysis

---

## Quick Reference

### Essential Commands

```bash
# Start new conversation (--verbose REQUIRED for stream-json)
claude --output-format stream-json --verbose "prompt" >> session.json

# Continue conversation (use LAST session ID)
claude --output-format stream-json --verbose --resume SESSION_ID "follow-up" >> session.json

# With full options
claude \
  --model claude-sonnet-4-20250514 \
  --output-format stream-json \
  --verbose \
  --resume LAST_SESSION_ID \
  "prompt" >> output.json
```

⚠️ **Important**: The `--verbose` flag is REQUIRED when using `--output-format stream-json` to get the full message stream. Without it, the stream output will not function properly.

### Session ID Extraction

```bash
# Get last session ID from stream
tail -1 output.json | jq -r '.session_id'

# Get all session IDs from stream
grep '"type":"result"' output.json | jq -r '.session_id'
```

### Key Metrics to Track

- **Session Chain Length**: Number of resumed sessions
- **Cache Hit Rate**: `cache_read_input_tokens / total_input_tokens`
- **Error Rate**: Tool errors per session
- **Cost Efficiency**: Cost per completed task
- **Response Time**: `duration_ms` vs `duration_api_ms`