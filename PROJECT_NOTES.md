# Remote Debugger MCP - Project Notes

## Project Overview

The Remote Debugger MCP is a Model Context Protocol (MCP) server that provides debugging and profiling tools for Go applications. It acts as a bridge between AI coding assistants (like Claude Code and Gemini CLI) and remote debugging/profiling tools.

**Current Version:** 1.1.0  
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
   - **Kube Tool** (`pkg/tools/kube/kube.go`) - Kubernetes port-forward operations
   - **System Info Tool** (`pkg/tools/sysinfo/sysinfo.go`) - System information gathering

4. **Connectors:**
   - **SSH Connector** (`pkg/connectors/ssh/ssh.go`) - SSH connection management for remote operations

### Dependencies

- `github.com/modelcontextprotocol/go-sdk v0.2.0` - MCP SDK
- `github.com/rs/zerolog v1.34.0` - Structured logging
- `github.com/go-playground/validator/v10 v10.23.0` - Input validation
- `github.com/stretchr/testify v1.11.0` - Testing framework

## Current Tools

### 1. Delve Debugger Tool

**Purpose:** Connects to remote Delve debugger instances for interactive debugging with session support

**Features:**
- Connects to remote dlv servers via `dlv connect`
- **NEW:** Session-based persistent connections for interactive debugging
- **NEW:** Session management with automatic cleanup (30-minute timeout)
- **NEW:** Three operation modes: connect, disconnect, command
- **NEW:** Background session monitoring and cleanup
- Configurable host/port (default: localhost:2345)  
- Command execution with automatic exit (legacy mode)
- Pagination support (max_lines, offset)
- Error handling for connection issues
- Backward compatibility with single-command mode

**Input Parameters:**
- `host` (optional): Target host (default: localhost)
- `port` (optional): Target port (default: 2345)
- `command` (optional): Delve command to execute (default: help)
- **NEW:** `session_id` (optional): Session identifier for persistent connections
- **NEW:** `action` (optional): Operation type - connect, disconnect, or command (default: command)
- `max_lines` (optional): Pagination limit (default: 1000)
- `offset` (optional): Line offset for pagination

**Session Usage:**
1. Connect: `delve SessionID=debug1 Action=connect Host=localhost Port=2345`
2. Execute commands: `delve SessionID=debug1 Action=command Command=continue`
3. Disconnect: `delve SessionID=debug1 Action=disconnect`

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

### 4. Kube Tool

**Purpose:** Kubernetes operations for debugging containerized applications

**Features:**
- kubectl port-forward for pods and services
- Automatic port availability checking and allocation
- Support for multiple Kubernetes contexts and namespaces
- Background port-forwarding management
- Kubeconfig file support

**Input Parameters:**
- `action` (required): Action to perform (currently: "port-forward")
- `namespace` (optional): Kubernetes namespace (default: "default")
- `resource` (required for port-forward): Resource to act on (e.g., "pod/my-pod", "service/my-service")
- `local_port` (optional): Local port for port-forward (default: 6060)
- `remote_port` (optional): Remote port for port-forward (defaults to local_port)
- `kubeconfig` (optional): Path to kubeconfig file
- `context` (optional): Kubernetes context to use
- `extra_args` (optional): Additional kubectl arguments

### 5. System Info Tool

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
- `make test` - Run test suites
- `make tools` - Install development tools
- `make tag` - Tag and push current version

## Current Git Status

**Branch:** master  
**Recent Changes:**
- **NEW:** Added session-based persistent connections to Delve debugger tool
- **NEW:** Enhanced Delve tool with connect/disconnect/command actions  
- **NEW:** Added automatic session cleanup and timeout management (30 minutes)
- **NEW:** Added pprof-test-std test application for standard HTTP pprof integration
- **NEW:** Updated pprof-test with HidePort option and startup logging
- **NEW:** Enhanced Delve test suite with session management testing
- Added comprehensive input validation with go-playground/validator
- Added complete test suites for all tools with stretchr/testify
- Input validation protects against injection vulnerabilities
- Added BSD 3-Clause License
- Added SSH Exec tool for remote binary execution
- Added System Info tool for system resource monitoring
- Added SSH connector for shared SSH functionality
- Renamed profiler tool to pprof for clarity
- Added golangci-lint configuration
- Updated README with similar projects section

**Current Staged Files:**
- `README.md` - Documentation updates (modified)
- **NEW:** `cmd/pprof-test-std/main.go` - Standard HTTP pprof test server
- `cmd/pprof-test/main.go` - Echo-based pprof test server with improvements
- `pkg/tools/delve/delve.go` - Enhanced Delve tool with session support
- `pkg/tools/delve/delve_test.go` - Updated test suite for session features

**Previously Added Files:**
- `pkg/tools/sshexec/sshexec_test.go` - Complete test suite for SSH exec tool
- `pkg/tools/pprof/pprof_test.go` - Complete test suite for pprof tool
- `pkg/tools/kube/kube_test.go` - Complete test suite for Kube tool
- `pkg/tools/sysinfo/sysinfo_test.go` - Complete test suite for System info tool
- `.golangci.yml` - Linter configuration
- `cmd/debugger/main.go` - Main server implementation
- `go.mod` / `go.sum` - Dependencies with validator and testify
- `pkg/connectors/ssh/ssh.go` - SSH connector
- `pkg/tools/pprof/pprof.go` - pprof tool with validation
- `pkg/tools/sshexec/sshexec.go` - SSH exec tool with validation
- `pkg/tools/sysinfo/sysinfo.go` - System info tool with validation
- `pkg/tools/kube/kube.go` - Kube tool with validation

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

# Legacy single-command mode
delve Command=help

# Session-based debugging
delve SessionID=debug1 Action=connect Host=localhost Port=2345
delve SessionID=debug1 Action=command Command=continue
delve SessionID=debug1 Action=command Command="break main.main"
delve SessionID=debug1 Action=command Command=locals  
delve SessionID=debug1 Action=disconnect
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

### Kube Integration
```bash
# Port-forward a pod to local port 8080
kube Action=port-forward Resource=pod/my-app LocalPort=8080 RemotePort=8080

# Port-forward a service with specific context and namespace
kube Action=port-forward Resource=service/web-service Namespace=production Context=prod-cluster LocalPort=3000 RemotePort=80

# Natural language examples
"Set up port forward from pod/redis in namespace cache to local port 6379"
"Forward port 8080 from service/api in production namespace using prod context"
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
- ✅ **NEW:** Delve debugger tool with session-based persistent connections
- ✅ **NEW:** Session management with automatic cleanup and timeout (30 minutes)
- ✅ **NEW:** Three-mode Delve operations: connect, disconnect, command
- ✅ **NEW:** Background session monitoring and cleanup goroutines
- ✅ **NEW:** Enhanced pprof test applications (standard HTTP and Echo-based)
- ✅ Delve debugger tool with full command support (legacy mode)
- ✅ pprof profiler tool with multiple profile types
- ✅ SSH exec tool for remote binary execution
- ✅ SSH exec tool process killing (PID and name-based)
- ✅ Configurable kill signals (TERM, KILL, etc.)
- ✅ Kubernetes port-forward tool for pod/service debugging
- ✅ System info tool for resource monitoring (local and remote)
- ✅ SSH connector for shared SSH functionality
- ✅ Background process execution and management
- ✅ Pagination support for large outputs
- ✅ Structured logging with debug mode
- ✅ Build system with linting and golangci-lint configuration
- ✅ Comprehensive input validation using go-playground/validator
- ✅ Complete test coverage with stretchr/testify suite framework
- ✅ Injection vulnerability protection for all user inputs
- ✅ Test suites for all tools with validation testing
- ✅ Version management
- ✅ Error handling and connection management
- ✅ BSD 3-Clause licensing

### Technical Debt & Improvements Needed
- ✅ ~~Add comprehensive test coverage~~ (COMPLETED)
- ✅ ~~Add tool parameter validation~~ (COMPLETED)
- [ ] Add configuration file support
- [ ] Implement authentication/authorization
- [ ] Add metrics and monitoring
- [ ] Add Docker support
- [ ] Consider WebSocket transport for real-time debugging
- [ ] Add support for additional debugger backends
- [ ] Add integration tests for end-to-end scenarios

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

**Input Validation:** All user-supplied inputs are validated using go-playground/validator to protect against injection vulnerabilities. This includes hostname validation, port range validation, path length limits, and content sanitization.

## Next Steps Recommendations

1. ~~Add comprehensive test suite~~ ✅ COMPLETED
2. ~~Implement input validation and sanitization~~ ✅ COMPLETED  
3. Add configuration management
4. Consider rate limiting and authentication
5. Add Docker containerization
6. Expand documentation with more examples
7. Add CI/CD pipeline
8. Add integration tests for complete workflows