# Remote Debugger MCP - Project Notes

## Project Overview

The Remote Debugger MCP is a Model Context Protocol (MCP) server that provides debugging and profiling tools for Go applications. It acts as a bridge between AI coding assistants (like Claude Code and Gemini CLI) and remote debugging/profiling tools.

**Current Version:** 1.0.0  
**Language:** Go 1.24  
**License:** BSD 3-Clause

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
   - **pprof Tool** (`pkg/tools/pprof/pprof.go`) - Go profiling integration
   - **SSH Exec Tool** (`pkg/tools/sshexec/sshexec.go`) - Remote binary execution via SSH
   - **System Info Tool** (`pkg/tools/sysinfo/sysinfo.go`) - System information gathering

4. **Connectors:**
   - **SSH Connector** (`pkg/connectors/ssh/ssh.go`) - SSH connection management for remote operations

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

### 3. SSH Exec Tool

**Purpose:** Transfer and execute binaries on remote hosts via SSH, or kill remote processes

**Features:**
- Secure binary transfer to remote hosts
- Remote execution with argument passing
- Automatic cleanup of transferred binaries (optional)
- Support for custom remote paths
- Output capture with pagination
- Exit code reporting
- **NEW:** Kill remote processes by PID or name pattern
- **NEW:** Configurable kill signals (TERM, KILL, etc.)
- Background process execution and management

**Input Parameters:**

*For binary execution:*
- `host` (required): SSH target host
- `port` (optional): SSH port (default: 22)
- `user` (optional): SSH user (default: current user)
- `binary_path` (required for exec): Local binary path to transfer
- `remote_path` (optional): Remote destination path (default: /tmp/<filename>)
- `args` (optional): Arguments to pass to the binary
- `keep_binary` (optional): Keep binary after execution (default: false)
- `run_in_background` (optional): Run process in background (default: false)
- `max_lines` (optional): Maximum lines to return (default: 1000)
- `offset` (optional): Line offset for pagination

*For process killing (mutually exclusive with exec mode):*
- `host` (required): SSH target host
- `port` (optional): SSH port (default: 22)
- `user` (optional): SSH user (default: current user)
- `kill_pid` (optional): PID to kill on remote host (mutually exclusive with kill_by_name)
- `kill_by_name` (optional): Kill processes by name pattern (mutually exclusive with kill_pid)
- `kill_signal` (optional): Signal to send when killing (default: TERM)
- `max_lines` (optional): Maximum lines to return (default: 1000)
- `offset` (optional): Line offset for pagination

### 4. System Info Tool

**Purpose:** Gather comprehensive system information from local or remote hosts

**Features:**
- CPU information (model, cores, threads, load averages, usage)
- Memory information (total, used, free, available, cached)
- System identification (hostname, kernel, OS, uptime)
- Remote execution via SSH
- Local execution when no SSH parameters provided
- Formatted output with structured data

**Input Parameters:**
- `ssh_host` (optional): SSH host for remote execution
- `ssh_port` (optional): SSH port (default: 22)
- `ssh_user` (optional): SSH user
- `max_lines` (optional): Maximum lines to return (default: 1000)
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
- Added BSD 3-Clause License
- Added SSH Exec tool for remote binary execution
- Added System Info tool for system resource monitoring
- Added SSH connector for shared SSH functionality
- Renamed profiler tool to pprof for clarity
- Added golangci-lint configuration
- Updated README with similar projects section

**Current Modified/Added Files:**
- `.golangci.yml` - Linter configuration (modified)
- `README.md` - Documentation updates (modified)
- `cmd/debugger/main.go` - Main server implementation (modified)
- `pkg/connectors/ssh/ssh.go` - SSH connector (new)
- `pkg/tools/delve/delve.go` - Delve tool (modified)
- `pkg/tools/pprof/pprof.go` - Renamed from profiler (renamed/modified)
- `pkg/tools/sshexec/sshexec.go` - SSH exec tool (new)
- `pkg/tools/sysinfo/sysinfo.go` - System info tool (new)

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

### SSH Exec Integration
```bash
# Transfer and execute a binary on remote host
sshexec Host=192.168.1.100 BinaryPath=/local/path/to/binary Args=["--verbose", "--config=/etc/app.conf"]

# Execute with custom remote path and keep binary
sshexec Host=server.example.com User=deploy BinaryPath=./myapp RemotePath=/opt/apps/myapp KeepBinary=true

# Execute binary in background
sshexec Host=192.168.1.100 BinaryPath=./server RunInBackground=true

# Kill a specific process by PID
sshexec Host=192.168.1.100 KillPID=12345

# Kill processes by name pattern
sshexec Host=192.168.1.100 KillByName=remote-debugger-mcp

# Kill processes with specific signal
sshexec Host=192.168.1.100 KillByName=myapp KillSignal=KILL

# Natural language examples
"Execute ./myserver on 192.168.1.100 in background"
"Kill all processes matching 'remote-debugger' on 192.168.1.100"
"Kill PID 12345 on remote server with SIGTERM"
```

### System Info Integration  
```bash
# Get local system information
sysinfo

# Get remote system information via SSH
sysinfo SSHHost=192.168.1.100 SSHUser=admin

# Natural language examples
"Check system resources on server 192.168.1.100"
"Get CPU and memory usage from production server"
```

## Development Status

### Completed Features
- ✅ MCP server framework with HTTP transport
- ✅ Delve debugger tool with full command support
- ✅ pprof profiler tool with multiple profile types
- ✅ SSH exec tool for remote binary execution
- ✅ **NEW:** SSH exec tool process killing (PID and name-based)
- ✅ **NEW:** Configurable kill signals (TERM, KILL, etc.)
- ✅ System info tool for resource monitoring (local and remote)
- ✅ SSH connector for shared SSH functionality
- ✅ Background process execution and management
- ✅ Pagination support for large outputs
- ✅ Structured logging with debug mode
- ✅ Build system with linting and golangci-lint configuration
- ✅ Version management
- ✅ Error handling and connection management
- ✅ BSD 3-Clause licensing

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
- Executes external commands (`dlv`, `go tool pprof`, `ssh`, `scp`)
- Makes network connections to remote services  
- Exposes debugging capabilities via HTTP
- Transfers and executes binaries on remote hosts via SSH
- Collects system information from local and remote hosts

**Defensive Use Only:** This tool is designed for legitimate debugging and profiling purposes. It should only be used in controlled development/testing environments.

**SSH Security:** The SSH-based tools rely on the system's SSH configuration and authentication mechanisms (SSH keys, agent forwarding, etc.). Ensure proper SSH security practices are followed.

## Next Steps Recommendations

1. Add comprehensive test suite
2. Implement input validation and sanitization
3. Add configuration management
4. Consider rate limiting and authentication
5. Add Docker containerization
6. Expand documentation with more examples
7. Add CI/CD pipeline