package sysinfo

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/connectors/ssh"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/types"
)

type Input struct {
	SSHHost  string `json:"ssh_host,omitempty"`  // SSH host for remote execution
	SSHPort  int    `json:"ssh_port,omitempty"`  // SSH port for remote execution (default: 22)
	SSHUser  string `json:"ssh_user,omitempty"`  // SSH user for remote execution
	MaxLines int    `json:"max_lines,omitempty"` // Maximum lines to return (default: 1000)
	Offset   int    `json:"offset,omitempty"`    // Line offset for pagination
}

type SystemInfo struct {
	CPUInfo    CPUInfo    `json:"cpu_info"`
	MemoryInfo MemoryInfo `json:"memory_info"`
	Hostname   string     `json:"hostname"`
	Kernel     string     `json:"kernel"`
	OS         string     `json:"os"`
	Uptime     string     `json:"uptime"`
}

type CPUInfo struct {
	Model      string `json:"model"`
	Cores      int    `json:"cores"`
	Threads    int    `json:"threads"`
	LoadAvg1   string `json:"load_avg_1min"`
	LoadAvg5   string `json:"load_avg_5min"`
	LoadAvg15  string `json:"load_avg_15min"`
	Usage      string `json:"usage_percent"`
}

type MemoryInfo struct {
	TotalMB     int    `json:"total_mb"`
	UsedMB      int    `json:"used_mb"`
	FreeMB      int    `json:"free_mb"`
	AvailableMB int    `json:"available_mb"`
	CachedMB    int    `json:"cached_mb"`
	SwapTotalMB int    `json:"swap_total_mb"`
	SwapUsedMB  int    `json:"swap_used_mb"`
	SwapFreeMB  int    `json:"swap_free_mb"`
	UsagePercent string `json:"usage_percent"`
}

type Tool struct {
	logger zerolog.Logger
}

func (s *Tool) executeCommand(command string, conn *ssh.Connector) (string, error) {
	if conn != nil {
		return conn.ExecuteCommand(command)
	}
	
	// Local execution
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.Output()
	return string(output), err
}

func (s *Tool) gatherSystemInfo(conn *ssh.Connector) *SystemInfo {
	info := &SystemInfo{}

	// Get hostname
	if output, err := s.executeCommand("hostname", conn); err == nil {
		info.Hostname = strings.TrimSpace(output)
	}

	// Get kernel version
	if output, err := s.executeCommand("uname -r", conn); err == nil {
		info.Kernel = strings.TrimSpace(output)
	}

	// Get OS information
	if output, err := s.executeCommand("cat /etc/os-release | grep PRETTY_NAME | cut -d'=' -f2 | tr -d '\"'", conn); err == nil {
		info.OS = strings.TrimSpace(output)
	} else if output, err := s.executeCommand("uname -s", conn); err == nil {
		info.OS = strings.TrimSpace(output)
	}

	// Get uptime
	if output, err := s.executeCommand("uptime -p 2>/dev/null || uptime", conn); err == nil {
		info.Uptime = strings.TrimSpace(output)
	}

	// Get CPU information
	cpuInfo := CPUInfo{}

	// CPU model
	if output, err := s.executeCommand("grep 'model name' /proc/cpuinfo | head -1 | cut -d':' -f2", conn); err == nil {
		cpuInfo.Model = strings.TrimSpace(output)
	}

	// CPU cores
	if output, err := s.executeCommand("grep -c ^processor /proc/cpuinfo", conn); err == nil {
		if cores, err := strconv.Atoi(strings.TrimSpace(output)); err == nil {
			cpuInfo.Threads = cores
		}
	}

	// Physical cores
	if output, err := s.executeCommand("grep 'cpu cores' /proc/cpuinfo | head -1 | cut -d':' -f2", conn); err == nil {
		if cores, err := strconv.Atoi(strings.TrimSpace(output)); err == nil {
			cpuInfo.Cores = cores
		}
	}

	// Load average
	if output, err := s.executeCommand("cat /proc/loadavg", conn); err == nil {
		fields := strings.Fields(output)
		if len(fields) >= 3 {
			cpuInfo.LoadAvg1 = fields[0]
			cpuInfo.LoadAvg5 = fields[1]
			cpuInfo.LoadAvg15 = fields[2]
		}
	}

	// CPU usage (calculating from /proc/stat)
	if output, err := s.executeCommand("awk '/cpu / {u=$2-u; s=$4-s; n=$5-n; i=$6-i} END {print 100 * (u+s+n) / (u+s+n+i)}' <(grep 'cpu ' /proc/stat) <(sleep 1; grep 'cpu ' /proc/stat)", conn); err == nil {
		usage := strings.TrimSpace(output)
		if usage != "" {
			cpuInfo.Usage = fmt.Sprintf("%.1f%%", parseFloat(usage))
		}
	}

	info.CPUInfo = cpuInfo

	// Get Memory information
	memInfo := MemoryInfo{}

	// Parse /proc/meminfo
	if output, err := s.executeCommand("cat /proc/meminfo", conn); err == nil {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}

			valueKB, _ := strconv.Atoi(fields[1])
			valueMB := valueKB / 1024

			switch fields[0] {
			case "MemTotal:":
				memInfo.TotalMB = valueMB
			case "MemFree:":
				memInfo.FreeMB = valueMB
			case "MemAvailable:":
				memInfo.AvailableMB = valueMB
			case "Cached:":
				memInfo.CachedMB = valueMB
			case "SwapTotal:":
				memInfo.SwapTotalMB = valueMB
			case "SwapFree:":
				memInfo.SwapFreeMB = valueMB
			}
		}

		// Calculate used memory
		memInfo.UsedMB = memInfo.TotalMB - memInfo.AvailableMB
		if memInfo.AvailableMB == 0 {
			// Fallback if MemAvailable is not present
			memInfo.UsedMB = memInfo.TotalMB - memInfo.FreeMB - memInfo.CachedMB
			memInfo.AvailableMB = memInfo.FreeMB + memInfo.CachedMB
		}

		// Calculate swap used
		memInfo.SwapUsedMB = memInfo.SwapTotalMB - memInfo.SwapFreeMB

		// Calculate usage percentage
		if memInfo.TotalMB > 0 {
			usagePercent := float64(memInfo.UsedMB) / float64(memInfo.TotalMB) * 100
			memInfo.UsagePercent = fmt.Sprintf("%.1f%%", usagePercent)
		}
	}

	info.MemoryInfo = memInfo

	return info
}

func parseFloat(s string) float64 {
	// Clean the string and parse
	re := regexp.MustCompile(`[0-9.]+`)
	match := re.FindString(s)
	if match == "" {
		return 0
	}
	val, _ := strconv.ParseFloat(match, 64)
	return val
}

func (s *Tool) formatSystemInfo(info *SystemInfo, target string) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("System Information for %s:\n", target))
	output.WriteString(strings.Repeat("=", 50) + "\n\n")

	// General info
	output.WriteString("General Information:\n")
	output.WriteString(fmt.Sprintf("  Hostname: %s\n", info.Hostname))
	output.WriteString(fmt.Sprintf("  OS: %s\n", info.OS))
	output.WriteString(fmt.Sprintf("  Kernel: %s\n", info.Kernel))
	output.WriteString(fmt.Sprintf("  Uptime: %s\n", info.Uptime))
	output.WriteString("\n")

	// CPU info
	output.WriteString("CPU Information:\n")
	output.WriteString(fmt.Sprintf("  Model: %s\n", info.CPUInfo.Model))
	output.WriteString(fmt.Sprintf("  Physical Cores: %d\n", info.CPUInfo.Cores))
	output.WriteString(fmt.Sprintf("  Logical Cores: %d\n", info.CPUInfo.Threads))
	output.WriteString(fmt.Sprintf("  Load Average: %s (1m), %s (5m), %s (15m)\n", 
		info.CPUInfo.LoadAvg1, info.CPUInfo.LoadAvg5, info.CPUInfo.LoadAvg15))
	if info.CPUInfo.Usage != "" {
		output.WriteString(fmt.Sprintf("  Current Usage: %s\n", info.CPUInfo.Usage))
	}
	output.WriteString("\n")

	// Memory info
	output.WriteString("Memory Information:\n")
	output.WriteString(fmt.Sprintf("  Total: %d MB\n", info.MemoryInfo.TotalMB))
	output.WriteString(fmt.Sprintf("  Used: %d MB (%s)\n", info.MemoryInfo.UsedMB, info.MemoryInfo.UsagePercent))
	output.WriteString(fmt.Sprintf("  Available: %d MB\n", info.MemoryInfo.AvailableMB))
	output.WriteString(fmt.Sprintf("  Free: %d MB\n", info.MemoryInfo.FreeMB))
	output.WriteString(fmt.Sprintf("  Cached: %d MB\n", info.MemoryInfo.CachedMB))
	
	if info.MemoryInfo.SwapTotalMB > 0 {
		output.WriteString("\nSwap Information:\n")
		output.WriteString(fmt.Sprintf("  Total: %d MB\n", info.MemoryInfo.SwapTotalMB))
		output.WriteString(fmt.Sprintf("  Used: %d MB\n", info.MemoryInfo.SwapUsedMB))
		output.WriteString(fmt.Sprintf("  Free: %d MB\n", info.MemoryInfo.SwapFreeMB))
	}

	return output.String()
}

func (s *Tool) SysInfoHandler(_ context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[Input]) (*mcp.CallToolResultFor[SystemInfo], error) {
	input := params.Arguments

	// Determine if this is local or remote execution
	var conn *ssh.Connector
	target := "localhost"

	if input.SSHHost != "" {
		port := 22
		if input.SSHPort != 0 {
			port = input.SSHPort
		}
		conn = ssh.New(input.SSHHost, port, input.SSHUser)
		target = conn.GetTarget()

		s.logger.Info().Msgf("Gathering system information from remote host: %s", target)
		
		// Test connection
		if err := conn.TestConnection(); err != nil {
			return nil, fmt.Errorf("failed to connect to %s: %v", target, err)
		}
	} else {
		s.logger.Info().Msg("Gathering local system information")
	}

	// Gather system information
	info := s.gatherSystemInfo(conn)

	// Format output
	output := s.formatSystemInfo(info, target)

	// Apply pagination
	maxLines := types.MaxDefaultLines
	if input.MaxLines > 0 {
		maxLines = input.MaxLines
	}

	offset := 0
	if input.Offset > 0 {
		offset = input.Offset
	}

	lines := strings.Split(output, "\n")
	totalLines := len(lines)

	if offset >= totalLines {
		lines = []string{}
	} else {
		end := offset + maxLines
		if end > totalLines {
			end = totalLines
		}
		lines = lines[offset:end]
	}

	truncated := totalLines > (offset + maxLines)
	paginatedOutput := strings.Join(lines, "\n")

	resultText := paginatedOutput
	if truncated {
		resultText = fmt.Sprintf("[Showing lines %d-%d of %d total lines. Use offset parameter to view more.]\n\n%s", 
			offset+1, offset+len(lines), totalLines, paginatedOutput)
	}

	return &mcp.CallToolResultFor[SystemInfo]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: resultText,
			},
		},
	}, nil
}

func (s *Tool) Register(server *mcp.Server) {
	sysInfoTool := &mcp.Tool{
		Name:        "sysinfo",
		Description: "Gather system information (CPU and memory) from local or remote host",
	}

	mcp.AddTool(server, sysInfoTool, s.SysInfoHandler)
	s.logger.Debug().Msg("sysinfo tool registered")
}

func New(logger zerolog.Logger) tools.Tool {
	return &Tool{
		logger: logger.With().Str("tool", "sysinfo").Logger(),
	}
}