# Remote debugger MCP
This is a Model Context Protocol (MCP) server that provides a remote debugger tool for profiling Go applications using pprof.


## Project Overview

Available tools: pprof

## Adding to Claude

```bash
 claude mcp add --transport http remote-debugger-mcp http://localhost:8899/mcp
```

## Running the Server

```bash
go run .
```

## Running application with profiling

See [pprof documentation](https://pkg.go.dev/net/http/pprof) for details on how to run your application with profiling enabled.
