package die

import (
	"strconv"
	"strings"
	"time"
)

// Version is set at build time via ldflags
var Version = "dev"

// Build info
var (
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// ProcessInfo holds enriched process metadata
type ProcessInfo struct {
	PID       int32
	PPID      int32
	Name      string
	Cmdline   string
	User      string
	CPU       float64
	Mem       float32
	MemRSS    uint64
	Status    string
	Ports     []int
	Cgroup    string
	Threads   int32
	StartTime int64
	Children  []*ProcessInfo
}

// Config holds all kill operation parameters
type Config struct {
	Force       bool
	Timeout     time.Duration
	Verbose     bool
	DryRun      bool
	Interactive bool
	Quiet       bool
	Tree        bool
	All         bool
	Regex       bool
	AuditLog    string
	Parallelism int
}

// TargetMode defines how to interpret the target
type TargetMode int

const (
	ModeAuto TargetMode = iota
	ModePort
	ModeName
	ModePID
	ModeCgroup
	ModeCPUAbove
	ModeMemAbove
)

func (m TargetMode) String() string {
	switch m {
	case ModeAuto:
		return "auto"
	case ModePort:
		return "port"
	case ModeName:
		return "name"
	case ModePID:
		return "pid"
	case ModeCgroup:
		return "cgroup"
	case ModeCPUAbove:
		return "cpu-above"
	case ModeMemAbove:
		return "mem-above"
	default:
		return "unknown"
	}
}

// AuditEntry represents a structured audit log entry
type AuditEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Action     string    `json:"action"`
	Target     string    `json:"target"`
	Mode       string    `json:"mode"`
	PIDs       []int32   `json:"pids"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	User       string    `json:"user"`
	DryRun     bool      `json:"dry_run"`
	Force      bool      `json:"force"`
	Tree       bool      `json:"tree"`
	DurationMs int64     `json:"duration_ms"`
	Version    string    `json:"version"`
}

// KillStats tracks operation metrics
type KillStats struct {
	Attempted int32
	Killed    int32
	Failed    int32
	Skipped   int32
}

// KillResult represents the outcome of a kill operation
type KillResult struct {
	Killed   int
	Failed   int
	Skipped  int
	Duration time.Duration
	PIDs     []int32
	Error    error
}

// PortInfo represents a listening port
type PortInfo struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	PID      int32  `json:"pid"`
	Name     string `json:"name"`
	Cmdline  string `json:"cmdline,omitempty"`
}

func IsNumeric(s string) bool {
	s = strings.TrimSpace(s)
	_, err := strconv.Atoi(s)
	return err == nil
}
