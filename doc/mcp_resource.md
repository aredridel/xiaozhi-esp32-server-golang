# MCP Tool Call Return Content Type Documentation

## Overview

This document describes in detail the tool call return content types supported by the program. The program adopts a **structured response system**, supporting processing and rendering of multiple content types.

## 🔧 Core Processing Flow

### Tool Call Response Processing

The core processor of tool call responses is responsible for:

1. **Tool Call Execution**: Iterate through all tool call requests
2. **Result Parsing**: Parse results returned by tools
3. **Content Type Identification**: Process differently based on content type
4. **Resource Rendering**: Process different types of content such as audio, text, resource links, etc.

## 📋 Supported Content Types

### 1. Audio Content (AudioContent)

**Type**: `mcp_go.AudioContent`

**Features**:
- Contains Base64 encoded audio data
- Supports multiple audio formats (MIME Type)
- Direct playback, terminates subsequent LLM processing

**Processing Flow**:
```go
if audioContent, ok := content.(mcp_go.AudioContent); ok {
    // Decode Base64 audio data
    rawAudioData, err := base64.StdEncoding.DecodeString(audioContent.Data)
    // Use music_player to play audio
    audioChan, err := play_music.PlayMusicFromAudioData(ctx, rawAudioData, ...)
    // Send playback status message
    l.serverTransport.SendSentenceStart(playText)
    // Play audio through TTS manager
    l.ttsManager.SendTTSAudio(ctx, audioChan, true)
}
```

**Use Cases**:
- Music playback tools
- Speech synthesis tools
- Audio file playback

### 2. Resource Link (ResourceLink)

**Type**: `mcp_go.ResourceLink`

**Features**:
- Contains resource URI and metadata
- Supports paginated reading of large resources
- Streaming processing, suitable for large files
- Uses Pipe mechanism for real-time audio stream playback

**Processing Flow**:
```go
if resourceLink, ok := content.(mcp_go.ResourceLink); ok {
    // Create Pipe for streaming transmission
    pipeReader, pipeWriter = io.Pipe()
    
    // Start paginated reading goroutine
    go func() {
        // Paginated reading of resource
        resourceResult, err := client.ReadResource(readCtx, mcp_go.ReadResourceRequest{
            Params: mcp_go.ReadResourceParams{
                URI: resourceLink.URI,
                Arguments: map[string]any{
                    "url": resourceLink.Description, 
                    "start": start, 
                    "end": start + page
                },
            },
        })
        
        // Process BlobResourceContents
        for _, content := range resourceResult.Contents {
            if audioContent, ok := content.(mcp_go.BlobResourceContents); ok {
                // Decode and send to audio stream channel
                rawAudioData, err := base64.StdEncoding.DecodeString(audioContent.Blob)
                streamChan <- rawAudioData
            }
        }
    }()
    
    // Use music_player to play audio stream
    audioChan, err := play_music.PlayMusicFromPipe(ctx, pipeReader, ...)
}
```

**Paginated Reading Parameter Details**:

#### Request Parameter Format
```go
Arguments: map[string]any{
    "url": resourceLink.Description,  // Actual resource URL
    "start": start,                   // Starting byte position
    "end": start + page,              // Ending byte position
}
```

#### Parameter Description
- **url**: Actual resource URL address, from `resourceLink.Description`
- **start**: Starting byte position, counted from 0
- **end**: Ending byte position (exclusive), i.e., reading range [start, end)
- **Page Size**: Defined by `McpReadResourcePageSize` constant, default 100KB

#### Paginated Reading Flow
```go
start := 0
page := McpReadResourcePageSize  // 100 * 1024
totalRead := 0
pageCount := 0

for {
    // Create context with timeout
    readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    
    // Send paginated reading request
    resourceResult, err := client.ReadResource(readCtx, mcp_go.ReadResourceRequest{
        Params: mcp_go.ReadResourceParams{
            URI: resourceLink.URI,
            Arguments: map[string]any{
                "url": resourceLink.Description, 
                "start": start, 
                "end": start + page
            },
        },
    })
    cancel()
    
    // Process returned BlobResourceContents
    for _, content := range resourceResult.Contents {
        if audioContent, ok := content.(mcp_go.BlobResourceContents); ok {
            // Decode Base64 data
            rawAudioData, err := base64.StdEncoding.DecodeString(audioContent.Blob)
            
            // Check if it's end flag
            if string(rawAudioData) == McpReadResourceStreamDoneFlag {
                return nil // Reading complete
            }
            
            // Send to audio stream channel
            streamChan <- rawAudioData
            totalRead += len(rawAudioData)
        }
    }
    
    // Check reading completion condition
    if len(rawAudioData) < page || !hasData {
        return nil // Reading complete
    }
    
    // Update starting position
    start += page
    pageCount++
}
```

#### Streaming Processing Mechanism

**Pipe Transmission Architecture**:
```go
// Create Pipe for audio stream transmission
pipeReader, pipeWriter = io.Pipe()

// Start data writing goroutine
go func() {
    for {
        select {
        case audioData, ok := <-streamChan:
            if !ok {
                pipeWriter.Close()
                return
            }
            pipeWriter.Write(audioData)
        case <-ctx.Done():
            return
        }
    }
}()

// Use music_player to play audio from Pipe
audioChan, err := play_music.PlayMusicFromPipe(ctx, pipeReader, ...)
```

#### Error Handling Mechanism

**Timeout Retry**:
```go
if err != nil {
    // If it's a timeout error, try retry
    if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
        log.Warnf("Resource read timeout, trying retry...")
        time.Sleep(1 * time.Second)
        continue
    }
    return fmt.Errorf("Failed to read resource: %v", err)
}
```

**Context Cancellation**:
```go
select {
case <-ctx.Done():
    log.Debugf("Resource read cancelled")
    return nil
case streamChan <- rawAudioData:
    // Normal data send
}
```

#### Pagination Mechanism Features
- **Memory Optimization**: Paginated reading avoids loading large files into memory at once
- **Streaming Processing**: Read and play simultaneously, supports real-time audio streams
- **Auto End**: Detect `McpReadResourceStreamDoneFlag` flag to determine reading completion
- **Error Recovery**: Supports timeout retry and context cancellation
- **Real-time Playback**: Use Pipe mechanism to read and play simultaneously
- **Timeout Control**: Each paginated read has a 30-second timeout limit
- **Retry Mechanism**: Automatic retry on timeout errors, 1-second interval

**Use Cases**:
- Large audio file playback
- Streaming media resource processing
- Network resource access
- Real-time audio stream playback

### 3. Text Content (TextContent)

**Type**: `mcp_go.TextContent`

**Features**:
- Plain text content
- Accumulated into response message
- Does not terminate subsequent processing

**Processing Flow**:
```go
if textContent, ok := content.(mcp_go.TextContent); ok {
    mcpContent += textContent.Text
}
```

**Use Cases**:
- Query result return
- Status information display
- Error message display

### 4. Blob Resource Content (BlobResourceContents)

**Type**: `mcp_go.BlobResourceContents`

**Features**:
- Binary data content
- Base64 encoded
- Supports streaming processing

**Processing Flow**:
```go
if audioContent, ok := content.(mcp_go.BlobResourceContents); ok {
    rawAudioData, err := base64.StdEncoding.DecodeString(audioContent.Blob)
    // Check if it's end flag
    if string(rawAudioData) == McpReadResourceStreamDoneFlag {
        return nil
    }
    // Send to audio stream channel
    streamChan <- rawAudioData
}
```

## 🏗️ Structured Response System

### Response Type Classification

The program supports four main response types:

#### 1. Action Response (MCPActionResponse)
- **Purpose**: Execute specific actions, such as playing music, exiting dialogue
- **Termination**: Configurable, usually terminates subsequent LLM processing
- **Control Flags**: `FinalAction`, `NoFurtherResponse`, `SilenceLLM`

#### 2. Audio Response (MCPAudioResponse)
- **Purpose**: Audio resource playback
- **Termination**: Usually terminates subsequent processing
- **Features**: Contains audio data and playback information

#### 3. Content Response (MCPContentResponse)
- **Purpose**: Return query data, status information
- **Termination**: Does not terminate subsequent processing
- **Features**: Contains data and display prompts

#### 4. Error Response (MCPErrorResponse)
- **Purpose**: Unified error handling
- **Termination**: Does not terminate subsequent processing
- **Features**: Contains error code and suggestions

### Response Processing Interface

```go
type MCPResponse interface {
    GetType() MCPResponseType
    GetSuccess() bool
    IsTerminal() bool // Key: Determine whether to terminate subsequent LLM processing
    ToJSON() (string, error)
    GetContent() []mcp_go.Content
}
```

## 🔄 Processing Flow Details

### 1. Tool Call Execution
```go
fcResult, err := tool.InvokableRun(toolCtx, toolCall.Function.Arguments)
```

### 2. Result Parsing
```go
// Try to parse local tool result
if mcpResp, ok := l.handleLocalToolResult(fcResult); ok {
    contentList = mcpResp.GetContent()
} else if toolCallResult, ok := l.handleToolResult(fcResult); ok {
    contentList = toolCallResult.Content
}
```

> `handleToolResult` **no longer requires tool return values to be JSON**.  
> - If returning standard MCP `CallToolResult` JSON, it will be parsed as structured content.  
> - If returning plain string, it will be automatically wrapped as `TextContent` to continue subsequent flow.  
> This allows both plain text tools and structured MCP tools to be processed uniformly.

### 3. Content Type Processing
```go
for _, content := range contentList {
    switch content.(type) {
    case mcp_go.AudioContent:
        // Process audio content
    case mcp_go.ResourceLink:
        // Process resource link
    case mcp_go.TextContent:
        // Process text content
    }
}
```

### 4. Subsequent Processing Control
```go
if invokeToolSuccess && !shouldStopLLMProcessing {
    l.DoLLmRequest(ctx, nil, l.einoTools, true)
}
```

## 📊 Content Type Comparison Table

| Content Type | Termination | Processing Method | Use Case | Example Tool |
|----------|--------|----------|----------|----------|
| **AudioContent** | Terminate | Direct playback | Small audio files | play_music |
| **ResourceLink** | Terminate | Paginated reading + Streaming playback | Large files/Streaming | music_player |
| **TextContent** | Not terminate | Accumulate text | Information query | get_datetime |
| **BlobResourceContents** | Terminate | Streaming processing | Audio stream data | audio_stream |

## 🎯 Best Practices

### 1. Tool Implementation Suggestions
- **Audio tools**: Return `AudioContent` or `ResourceLink`
- **Query tools**: Return `TextContent`
- **Action tools**: Use structured response system

### 2. Performance Optimization
- Use `ResourceLink` for paginated processing of large files, supports streaming playback
- Use `AudioContent` directly for small audio files, reduces network overhead
- Avoid overly long text content, affects response speed
- Use Pipe mechanism to read and play simultaneously, improves user experience

### 3. Error Handling
- Use `MCPErrorResponse` for unified error format
- Provide meaningful error codes and suggestions
- Maintain backward compatibility

## 🔧 Configuration Parameters

### Pagination Configuration
- `McpReadResourcePageSize`: Resource read page size, default 100KB (100 * 1024)
- `McpReadResourceStreamDoneFlag`: Stream end flag, is `"[DONE]"`
- **Read Timeout**: Timeout for each paginated read, default 30 seconds
- **Retry Mechanism**: Automatic retry on timeout errors, 1-second interval

### Audio Configuration
- `OutputAudioFormat.SampleRate`: Output audio sample rate
- `OutputAudioFormat.FrameDuration`: Output audio frame duration
- **Audio Format**: Automatically recognized based on `resourceLink.MIMEType`

## 📝 Extension Guide

### Adding New Content Types
1. Define new content type in `mcp_go` package
2. Add type processing logic in `handleToolCallResponse`
3. Implement corresponding processing function
4. Update documentation and tests

### Custom Response Types
1. Inherit `MCPResponseBase`
2. Implement `MCPResponse` interface
3. Add parsing logic in `ParseMCPResponse`
4. Provide convenient constructor

## 🎵 MCP Audio Server Independent Repository

### Overview

MCP Audio Server has been split into an independent repository. It is recommended to run and debug audio MCP Servers through an independent project. The current section of the document mainly explains its protocol compatibility with the main service.

### Core Functions

#### 1. Music Playback Tool
- **Tool Name**: `musicPlayer`
- **Function**: Search and play music
- **Return**: `ResourceLink` type audio resource link

#### 2. Audio Resource Template
- **URI Format**: `resource://read_from_http`
- **Function**: Supports paginated reading of audio data, parameters passed through Arguments
- **Parameters**: url (actual music URL), start (starting position), end (ending position)
- **Return**: `BlobResourceContents` type audio data

### Key Features

- **Paginated Reading**: Supports streaming processing of large files
- **HTTP Range Request**: Implements segmented acquisition of audio data
- **Error Handling**: Handles 416 status code and other exceptions
- **Timeout Retry**: Automatic retry on timeout errors, 1-second interval
- **Context Cancellation**: Supports graceful resource read cancellation
- **Base64 Encoding**: Securely passes music URL parameters
- **Multi-transport Support**: stdio and HTTP two transport methods
- **Real-time Playback**: Use Pipe mechanism to read and play simultaneously

### Usage

```bash
# Get and enter independent repository
git clone https://github.com/hackers365/mcp_audio_server.git
cd mcp_audio_server

# Start server
go run .

# Tool call
{
  "name": "musicPlayer",
  "arguments": {"query": "Jay Chou"}
}
```

This independent project demonstrates how to build MCP tools supporting audio resource processing, and can be used as a reference template for developing other audio-related tools. For more complete usage instructions, please refer to `doc/mcp_audio_example.md`.

---

*This document reflects all tool call return content types currently supported by the program.* 
