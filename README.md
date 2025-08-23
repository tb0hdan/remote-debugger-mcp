# Remote debugger MCP
This is a Model Context Protocol (MCP) server that provides a remote debugger tool for profiling Go applications using pprof.


## Project Overview

Available tools:

- [delve](https://github.com/go-delve/delve)
- [pprof](https://pkg.go.dev/net/http/pprof)

## Adding to coding agents

### Claude Code

```bash
 claude mcp add --transport http remote-debugger-mcp http://localhost:8899/mcp
```

### Gemini CLI

```bash
gemini mcp add remote-debugger-mcp http://localhost:8899/mcp -t http
```



## Running the Server

You need to build it once using the following command:
```bash
make
```

then just

```bash
build/remote-debugger-mcp
```


## Tools usage 

### delve

Running application with delve debugger

```bash
dlv debug --accept-multiclient --headless --listen=:2345
```

or even with PID (for example, 862262)

```bash
dlv attach 862262 --accept-multiclient --headless --listen=:2345
```

Sample agent usage

```
delve Command=help
```


### pprof

Running application with profiling

See [pprof documentation](https://pkg.go.dev/net/http/pprof) for details on how to run your application with profiling enabled.


Sample agent usage

```
List available pprof profiles for port 8899
```

or

```
pprof Host=192.168.4.15 Profile=heap 
```

or even

```
Run available pprof profiles for host 192.168.4.15 and aggregate data
```

## Similar projects

- [Pprof Analyzer MCP Server](https://github.com/ZephyrDeng/pprof-analyzer-mcp)
- [pprof-mcp-agent](https://github.com/yudppp/pprof-mcp-agent)
  
