package die

import (
	"runtime"
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

// Default configuration values
const (
	DefaultTimeout     = 5 * time.Second
	DefaultParallelism = 0 // resolved at runtime to GOMAXPROCS
	AutoConfirmLimit   = 5 // prompt user when matching more than this many processes
)

// Safety model
//
// die is dry-run by default. Running `die <target>` always previews what
// would be killed without touching any process. To actually kill, the caller
// must explicitly pass --kill (Config.KillEnabled = true). This prevents
// accidental mass kills from broad patterns like `die version` or `die help`.
//
//	die node          → preview only (safe)
//	die node --kill   → terminate matched processes
//	die node --dry    → preview only (explicit, same as default)

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
	DryRun      bool // explicit --dry flag; redundant when KillEnabled is false
	KillEnabled bool // must be explicitly true to perform actual kills; false = dry-run always
	Interactive bool
	Quiet       bool
	Tree        bool
	All         bool
	Regex       bool
	AuditLog    string
	Parallelism int
}

// IsDryRun reports whether this config will result in a preview-only run.
// Killing is live only when KillEnabled is explicitly true and DryRun is false.
func (c Config) IsDryRun() bool {
	return !c.KillEnabled || c.DryRun
}

// WithDefaults returns a Config with safe zero-value defaults applied.
func (c Config) WithDefaults() Config {
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.Parallelism <= 0 {
		c.Parallelism = runtime.GOMAXPROCS(0)
	}
	return c
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
	DryRun   bool // true when the run was preview-only
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
