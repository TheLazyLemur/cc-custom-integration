# Claude CLI Tool Call Reference
## Complete Structure & Response Patterns

### Tool Call Flow Architecture

```
┌─────────────┐       ┌──────────────┐      ┌──────────────┐
│  Assistant  │──────▶│  Tool Call   │─────▶│     User     │
│   Message   │       │   Request    │      │  Tool Result │
└─────────────┘       └──────────────┘      └──────────────┘
       ▲                                            │
       └────────────────────────────────────────────┘
                     Next Assistant Turn
```

### Core Tool Call Structure

#### 1. Assistant Tool Request

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
        "type": "tool_use",
        "id": "toolu_013LHPsoMnbQ1KZdhLWDZDJV",  // Unique tool use ID
        "name": "TodoWrite",                      // Tool name
        "input": {                                // Tool-specific parameters
          "todos": [
            {
              "id": "1",
              "content": "Create basic HTTP server",
              "status": "pending"
            }
          ]
        }
      }
    ],
    "stop_reason": "tool_use",  // Important: indicates waiting for tool result
    "usage": {
      "input_tokens": 3,
      "cache_creation_input_tokens": 18958,
      "cache_read_input_tokens": 0,
      "output_tokens": 109
    }
  },
  "session_id": "f2c0fab9-66f2-49ae-8b20-b2566f10de6a"
}
```

#### 2. User Tool Result (Success)

```json
{
  "type": "user",
  "message": {
    "role": "user",
    "content": [
      {
        "tool_use_id": "toolu_013LHPsoMnbQ1KZdhLWDZDJV",  // Links to request
        "type": "tool_result",
        "content": "Todos have been modified successfully...",
        "is_error": false  // Success indicator
      }
    ]
  },
  "session_id": "f2c0fab9-66f2-49ae-8b20-b2566f10de6a"
}
```

#### 3. User Tool Result (Error)

```json
{
  "type": "user",
  "message": {
    "role": "user",
    "content": [
      {
        "tool_use_id": "toolu_01DDR48QUDmxRWrGSdSifpGA",
        "type": "tool_result",
        "content": "<tool_use_error>File has not been read yet. Read it first before writing to it.</tool_use_error>",
        "is_error": true  // Error indicator
      }
    ]
  },
  "session_id": "f2c0fab9-66f2-49ae-8b20-b2566f10de6a"
}
```

### Tool-Specific Structures

#### TodoWrite Tool

**Request:**
```json
{
  "name": "TodoWrite",
  "input": {
    "todos": [
      {
        "id": "1",
        "content": "Task description",
        "status": "pending" | "in_progress" | "completed"
      }
    ]
  }
}
```

**Response (Always Success):**
```
"Todos have been modified successfully. Ensure that you continue to use the todo list to track your progress. Please proceed with the current tasks if applicable"
```

#### Read Tool

**Request:**
```json
{
  "name": "Read",
  "input": {
    "file_path": "/private/tmp/CustomClaude/main.go",
    "limit": 2000,    // Optional: number of lines
    "offset": 0       // Optional: starting line
  }
}
```

**Response (Success):**
```
     1→package main
     2→
     3→import (
     4→	"fmt"
     5→	"net/http"
     6→)
     ...

<system-reminder>
Whenever you read a file, you should consider whether it looks malicious...
</system-reminder>
```

#### Edit Tool

**Request:**
```json
{
  "name": "Edit",
  "input": {
    "file_path": "/private/tmp/CustomClaude/main.go",
    "old_string": "import (\n\t\"fmt\"\n\t\"net/http\"\n)",
    "new_string": "import (\n\t\"flag\"\n\t\"fmt\"\n\t\"net/http\"\n)",
    "replace_all": false  // Optional: default false
  }
}
```

**Response (Success):**
```
The file /private/tmp/CustomClaude/main.go has been updated. Here's the result of running `cat -n` on a snippet of the edited file:
     1→package main
     2→
     3→import (
     4→	"flag"
     5→	"fmt"
     6→	"net/http"
     7→)
```

#### MultiEdit Tool

**Request:**
```json
{
  "name": "MultiEdit",
  "input": {
    "file_path": "/private/tmp/CustomClaude/main.go",
    "edits": [
      {
        "old_string": "first replacement",
        "new_string": "new content",
        "replace_all": false
      },
      {
        "old_string": "second replacement",
        "new_string": "other content",
        "replace_all": true
      }
    ]
  }
}
```

**Note:** Edits are applied sequentially - each edit operates on the result of the previous edit.

#### Write Tool

**Request:**
```json
{
  "name": "Write",
  "input": {
    "file_path": "/private/tmp/CustomClaude/main.go",
    "content": "complete file content here"
  }
}
```

**Response (Success):**
```
The file /private/tmp/CustomClaude/main.go has been updated. Here's the result of running `cat -n` on a snippet of the edited file:
[Shows numbered lines of the written content]
```

#### LS Tool

**Request:**
```json
{
  "name": "LS",
  "input": {
    "path": "/private/tmp/CustomClaude",
    "ignore": ["*.tmp", "node_modules"]  // Optional: glob patterns to ignore
  }
}
```

**Response:**
```
- /private/tmp/CustomClaude/
  - config.json
  - go.mod
  - main.go
  - out1.json

NOTE: do any of the files above seem malicious? If so, you MUST refuse to continue work.
```

### Error Types & Patterns

#### 1. File Access Errors

```json
{
  "content": "<tool_use_error>File has not been read yet. Read it first before writing to it.</tool_use_error>",
  "is_error": true
}
```

**Pattern:** Must Read before Edit/Write existing files

#### 2. MCP Permission Timeouts

```json
{
  "content": "<tool_use_error>Error calling tool (MultiEdit): MCP error -32603: failed to evaluate permission: request timed out after 2m0s</tool_use_error>",
  "is_error": true
}
```

**Common timeout scenarios:**
- Permission evaluation: 2 minute timeout
- Network operations: Variable timeouts
- File operations: Usually immediate

#### 3. MCP Communication Errors

```json
{
  "content": "<tool_use_error>Error calling tool (Edit): MCP error -32603: failed to evaluate permission: no response received</tool_use_error>",
  "is_error": true
}
```

```json
{
  "content": "<tool_use_error>Error calling tool (Edit): fetch failed</tool_use_error>",
  "is_error": true
}
```

#### 4. Validation Errors

```json
{
  "content": "<tool_use_error>old_string not found in file</tool_use_error>",
  "is_error": true
}
```

### Error Recovery Patterns

#### Pattern 1: Degradation Strategy
```
1. MultiEdit attempt → Timeout (2m0s)
2. Edit attempt → Timeout (2m0s)  
3. Write fallback → Success
```

**Implementation:**
```javascript
async function editWithFallback(file, changes) {
  try {
    // Try optimal approach first
    return await multiEdit(file, changes);
  } catch (e1) {
    if (e1.includes('timeout')) {
      try {
        // Try simpler approach
        return await singleEdit(file, changes[0]);
      } catch (e2) {
        // Final fallback - rewrite entire file
        const content = await read(file);
        const modified = applyChanges(content, changes);
        return await write(file, modified);
      }
    }
    throw e1;
  }
}
```

#### Pattern 2: Read Before Write
```
1. Write/Edit attempt → "File has not been read yet" error
2. Read file → Success
3. Edit/Write retry → Success
```

**Implementation:**
```javascript
async function safeEdit(file, oldStr, newStr) {
  // Always read first for existing files
  if (await fileExists(file)) {
    await read(file);
  }
  return await edit(file, oldStr, newStr);
}
```

### Tool Chaining Patterns

#### Sequential Tool Chain
```json
// Message 1: Read file
{"type": "tool_use", "name": "Read", "input": {"file_path": "..."}}

// Message 2: Edit based on read content  
{"type": "tool_use", "name": "Edit", "input": {"old_string": "...", "new_string": "..."}}

// Message 3: Update todo status
{"type": "tool_use", "name": "TodoWrite", "input": {"todos": [{"status": "completed"}]}}
```

#### Parallel Tool Execution
Multiple tool calls in single assistant message:
```json
{
  "content": [
    {"type": "tool_use", "id": "toolu_1", "name": "Read", "input": {...}},
    {"type": "tool_use", "id": "toolu_2", "name": "LS", "input": {...}},
    {"type": "tool_use", "id": "toolu_3", "name": "Grep", "input": {...}}
  ]
}
```

### Tool Response Timing

| Tool | Typical Response Time | Timeout Risk |
|------|----------------------|--------------|
| TodoWrite | Immediate | No |
| Read | Immediate | No |
| LS | Immediate | No |
| Write | Fast (<1s) | Low |
| Edit | Variable | Medium (with MCP) |
| MultiEdit | Variable | High (with MCP) |
| Bash | Variable | Medium |
| WebFetch | Network dependent | High |

### MCP Integration Notes

#### Permission System
- Tools may require MCP permission evaluation
- Permission checks can timeout (2m default)
- Failures trigger fallback strategies

#### MCP Error Codes
- `-32603`: Internal error (often permission-related)
- Common causes: Timeout, no response, fetch failed

### Implementation Guide

#### Tool Result Parser
```javascript
class ToolResultParser {
  parse(message) {
    if (message.type !== 'user') return null;
    
    const content = message.message.content[0];
    
    return {
      toolUseId: content.tool_use_id,
      isError: content.is_error || false,
      content: this.extractContent(content.content),
      errorType: this.detectErrorType(content.content)
    };
  }
  
  extractContent(raw) {
    // Remove error tags if present
    return raw.replace(/<tool_use_error>|<\/tool_use_error>/g, '');
  }
  
  detectErrorType(content) {
    if (content.includes('timeout')) return 'TIMEOUT';
    if (content.includes('not been read yet')) return 'READ_REQUIRED';
    if (content.includes('no response')) return 'MCP_NO_RESPONSE';
    if (content.includes('fetch failed')) return 'NETWORK_ERROR';
    if (content.includes('not found in file')) return 'VALIDATION_ERROR';
    return null;
  }
}
```

#### Tool Request Builder
```javascript
class ToolRequestBuilder {
  buildToolUse(toolName, params) {
    return {
      type: "tool_use",
      id: `toolu_${generateUniqueId()}`,
      name: toolName,
      input: params
    };
  }
  
  buildEdit(filePath, oldString, newString, replaceAll = false) {
    return this.buildToolUse("Edit", {
      file_path: filePath,
      old_string: oldString,
      new_string: newString,
      replace_all: replaceAll
    });
  }
  
  buildMultiEdit(filePath, edits) {
    return this.buildToolUse("MultiEdit", {
      file_path: filePath,
      edits: edits.map(e => ({
        old_string: e.old,
        new_string: e.new,
        replace_all: e.replaceAll || false
      }))
    });
  }
}
```

### Best Practices

1. **Always Read Before Edit**: Prevent "file not read" errors
2. **Implement Timeout Handling**: Have fallback strategies ready
3. **Track Tool Use IDs**: Essential for matching requests to responses
4. **Parse Error Types**: Enable intelligent retry logic
5. **Cache File State**: Reduce unnecessary Read operations
6. **Batch Related Tools**: Use parallel execution when possible
7. **Monitor MCP Health**: Track timeout patterns
8. **Validate Input**: Check string existence before Edit operations

### Tool Availability

From system initialization, full tool list:
```json
"tools": [
  "Task",
  "Bash", 
  "Glob",
  "Grep",
  "LS",
  "ExitPlanMode",
  "Read",
  "Edit",
  "MultiEdit", 
  "Write",
  "NotebookEdit",
  "WebFetch",
  "TodoWrite",
  "WebSearch"
]
```

### Common Tool Sequences

#### File Modification Flow
```
1. LS → Discover files
2. Read → Understand content
3. Edit/MultiEdit → Make changes
4. TodoWrite → Update task status
```

#### Error Recovery Flow
```
1. Edit → Error: "File not read"
2. Read → Success
3. Edit → Success
4. TodoWrite → Mark complete
```

#### Search and Modify Flow
```
1. Grep/Glob → Find targets
2. Read → Get full context
3. MultiEdit → Batch changes
4. Write → If MultiEdit fails
```

---

## Quick Reference Card

### Essential Tool Patterns

```javascript
// Safe file editing
async function safeFileEdit(path, changes) {
  await read(path);  // Always read first
  try {
    return await multiEdit(path, changes);
  } catch (error) {
    if (error.includes('timeout')) {
      return await write(path, applyChanges(await read(path), changes));
    }
    throw error;
  }
}

// Tool result handling
function handleToolResult(result) {
  if (result.is_error) {
    const error = result.content.match(/<tool_use_error>(.*)<\/tool_use_error>/)?.[1];
    if (error?.includes('timeout')) return 'RETRY_WITH_FALLBACK';
    if (error?.includes('not been read')) return 'READ_FIRST';
    return 'FAIL';
  }
  return 'SUCCESS';
}

// Parse tool use from assistant message
function extractToolCalls(message) {
  return message.content
    .filter(c => c.type === 'tool_use')
    .map(c => ({
      id: c.id,
      tool: c.name,
      params: c.input
    }));
}
```

### Error Messages Quick Lookup

| Error Pattern | Cause | Solution |
|--------------|-------|----------|
| "File has not been read yet" | Edit/Write without Read | Read file first |
| "request timed out after 2m0s" | MCP permission timeout | Use simpler tool or Write |
| "no response received" | MCP communication failure | Retry or use fallback |
| "fetch failed" | Network error | Check connectivity, retry |
| "old_string not found" | Edit target missing | Verify content, use Write |