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


## Sample Claude usage

```
pprof Host=192.168.4.15 Profile=heap 
```

or even

```
Run available pprof profiles for host 192.168.4.15 and aggregate data
```
