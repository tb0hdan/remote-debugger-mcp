# Remote Debugger MCP - Project Notes

## Project Overview

The Remote Debugger MCP is a Model Context Protocol (MCP) server that provides debugging and profiling tools for Go applications. It acts as a bridge between AI coding assistants (like Claude Code and Gemini CLI) and remote debugging/profiling tools.

**Current Version:** 1.0.0  
**Language:** Go 1.24  
**License:** Not specified

## Architecture

### Core Components

1. **MCP Server** (`cmd/debugger/main.go`)
   - HTTP server running on port 8899 (default)
   - Exposes MCP endpoint at `/mcp`
   - Embeds version from `VERSION` file
   - Supports debug mode with structured logging (zerolog)

2. **Tool Interface** (`pkg/tools/tools.go`)
   - Simple interface for registering tools with the MCP server
   - Allows modular tool architecture

3. **Available Tools:**
   - **Delve Tool** (`pkg/tools/delve/delve.go`) - Remote Go debugger integration
   - **Profiler Tool** (`pkg/tools/profiler/profiler.go`) - pprof integration

### Dependencies

- `github.com/modelcontextprotocol/go-sdk v0.2.0` - MCP SDK
- `github.com/rs/zerolog v1.34.0` - Structured logging

## Current Tools

### 1. Delve Debugger Tool

**Purpose:** Connects to remote Delve debugger instances for interactive debugging

**Features:**
- Connects to remote dlv servers via `dlv connect`
- Configurable host/port (default: localhost:2345)  
- Command execution with automatic exit
- Pagination support (max_lines, offset)
- Error handling for connection issues

**Input Parameters:**
- `host` (optional): Target host (default: localhost)
- `port` (optional): Target port (default: 2345)
- `command` (optional): Delve command to execute (default: help)
- `max_lines` (optional): Pagination limit (default: 1000)
- `offset` (optional): Line offset for pagination

### 2. pprof Profiler Tool

**Purpose:** Retrieves and analyzes Go application profiling data

**Features:**
- Connects to pprof endpoints (`/debug/pprof/`)
- Supports all pprof profile types (heap, cpu, goroutine, etc.)
- Text-based output via `go tool pprof -text`
- Configurable profiling duration for CPU profiles
- Pagination support for large outputs

**Input Parameters:**
- `host` (optional): Target host (default: localhost)
- `port` (optional): Target port (default: 6060)
- `profile` (optional): Profile type (default: heap)
- `seconds` (optional): CPU profiling duration (default: 30)
- `max_lines` (optional): Pagination limit (default: 100)
- `offset` (optional): Line offset for pagination

## Build System

**Makefile targets:**
- `make` or `make all` - Run linter and build
- `make lint` - Run golangci-lint
- `make build` - Build binary to `build/remote-debugger-mcp`
- `make tools` - Install development tools
- `make tag` - Tag and push current version

## Current Git Status

**Branch:** master  
**Recent Changes:**
- Several README.md updates
- Project restructure (moved main.go to cmd/debugger/)
- Added build system and linting
- Removed old profiler implementation, added new tools structure

**Modified Files:**
- `.gitignore` - Updated ignore patterns
- `README.md` - Documentation updates  
- `go.mod/go.sum` - Dependency updates
- Various new files added in restructure

## Integration

### Claude Code
```bash
claude mcp add --transport http remote-debugger-mcp http://localhost:8899/mcp
```

### Gemini CLI
```bash
gemini mcp add remote-debugger-mcp http://localhost:8899/mcp -t http
```

## Usage Examples

### Delve Integration
```bash
# Start application with delve
dlv debug --accept-multiclient --headless --listen=:2345

# Or attach to running process
dlv attach <PID> --accept-multiclient --headless --listen=:2345

# Agent usage
delve Command=help
```

### pprof Integration
```bash
# Agent usage examples
pprof Host=192.168.4.15 Profile=heap
# Or natural language
"Run available pprof profiles for host 192.168.4.15 and aggregate data"
```

## Development Status

### Completed Features
- ✅ MCP server framework with HTTP transport
- ✅ Delve debugger tool with full command support
- ✅ pprof profiler tool with multiple profile types
- ✅ Pagination support for large outputs
- ✅ Structured logging with debug mode
- ✅ Build system with linting
- ✅ Version management
- ✅ Error handling and connection management

### Technical Debt & Improvements Needed
- [ ] Add comprehensive test coverage
- [ ] Add configuration file support
- [ ] Implement authentication/authorization
- [ ] Add metrics and monitoring
- [ ] Add Docker support
- [ ] Add tool parameter validation
- [ ] Consider WebSocket transport for real-time debugging
- [ ] Add support for additional debugger backends

### Known Limitations
- Currently Go-specific (Delve, pprof)
- No authentication mechanism
- Limited error recovery for network issues
- No support for persistent debugging sessions
- Text-only output (no binary profile analysis)

## Security Considerations

This is a debugging/profiling tool that:
- Executes external commands (`dlv`, `go tool pprof`)
- Makes network connections to remote services  
- Exposes debugging capabilities via HTTP

**Defensive Use Only:** This tool is designed for legitimate debugging and profiling purposes. It should only be used in controlled development/testing environments.

## Next Steps Recommendations

1. Add comprehensive test suite
2. Implement input validation and sanitization
3. Add configuration management
4. Consider rate limiting and authentication
5. Add Docker containerization
6. Expand documentation with more examples
7. Add CI/CD pipeline