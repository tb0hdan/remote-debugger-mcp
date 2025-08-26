# Remote debugger MCP
This is a Model Context Protocol (MCP) server that runs on your machine and provides a set remote debugging tools for profiling Go applications.


## Project Overview

Available tools:

- [delve](https://github.com/go-delve/delve) - non-interactive commands only
- kube - port-forwarding to Kubernetes clusters (requires kubectl configured)
- [pprof](https://pkg.go.dev/net/http/pprof)
- sshexec - requires SSH access already configured
- sysinfo - both local and remote system information via SSH

## Adding to coding agents

### Claude Code

```bash
 claude mcp add --scope user --transport http remote-debugger-mcp http://localhost:8899/mcp
```

### Gemini CLI

```bash
gemini mcp add remote-debugger-mcp http://localhost:8899/mcp -t http
```



## Running the Server

Build it once using the following command:

```bash
make
```

then just

```bash
build/remote-debugger-mcp -debug
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

### kube

You can use deployment [pprof-test-deployment.yaml](deployments/pprof-test/pprof-test-deployment.yaml) to test kube tool.

```
kubectl apply -f deployments/pprof-test/pprof-test-deployment.yaml
```

Then use the following command to port forward and gather pprof heap profile.

```
Use kube tool to port forward deployment pprof-test-deployment, then gather pprof heap. Stop port forwarding.
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

### sshexec

- Kill specific PID

```
sshexec Host=192.168.1.100 KillPID=12345
```

 - Kill by process name

```
sshexec Host=192.168.1.100 KillByName=remote-debugger-mcp
```

- Kill with specific signal

```
sshexec Host=192.168.1.100 KillByName=myapp KillSignal=KILL
```

### Sysinfo

```
sysinfo
```
or

```
sysinfo Host=192.168.4.15
```

### Combined usage (tested on Claude)

```
Build this project locally and then transfer it to remote host 192.168.4.15 using sshexec tool. Run it there with -bind 192.168.4.15:8899.
Then fetch profiling information using pprof tool, show it here, terminate remote binary.
```

## Similar projects

- [Pprof Analyzer MCP Server](https://github.com/ZephyrDeng/pprof-analyzer-mcp)
- [pprof-mcp-agent](https://github.com/yudppp/pprof-mcp-agent)
  
