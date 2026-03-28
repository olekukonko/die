package die

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

// Theme defines colors for UI elements
type Theme struct {
	Primary   *color.Color
	Success   *color.Color
	Error     *color.Color
	Warning   *color.Color
	Info      *color.Color
	Highlight *color.Color
}

// DefaultTheme returns the default color theme
func DefaultTheme() *Theme {
	return &Theme{
		Primary:   color.New(color.FgCyan),
		Success:   color.New(color.FgGreen),
		Error:     color.New(color.FgRed),
		Warning:   color.New(color.FgYellow),
		Info:      color.New(color.FgBlue),
		Highlight: color.New(color.FgWhite),
	}
}

// NoColorTheme returns a theme with no colors
func NoColorTheme() *Theme {
	return &Theme{
		Primary:   color.New(),
		Success:   color.New(),
		Error:     color.New(),
		Warning:   color.New(),
		Info:      color.New(),
		Highlight: color.New(),
	}
}

// UI handles all user interface rendering
type UI struct {
	out     io.Writer
	theme   *Theme
	quiet   bool
	verbose bool
}

// UIOption configures the UI
type UIOption func(*UI)

// WithTheme sets a custom theme
func WithTheme(theme *Theme) UIOption {
	return func(u *UI) {
		u.theme = theme
	}
}

// WithQuiet suppresses non-essential output
func WithQuiet(quiet bool) UIOption {
	return func(u *UI) {
		u.quiet = quiet
	}
}

// WithVerbose enables verbose output
func WithVerbose(verbose bool) UIOption {
	return func(u *UI) {
		u.verbose = verbose
	}
}

// NewUI creates a new UI instance
func NewUI(opts ...UIOption) *UI {
	ui := &UI{
		out:   os.Stdout,
		theme: DefaultTheme(),
	}

	for _, opt := range opts {
		opt(ui)
	}

	return ui
}

// SetOutput sets the output writer (useful for testing)
func (u *UI) SetOutput(w io.Writer) {
	u.out = w
}

// PrintPrimary prints with primary color
func (u *UI) PrintPrimary(format string, args ...interface{}) {
	u.theme.Primary.Fprintf(u.out, format, args...)
}

// PrintSuccess prints with success color
func (u *UI) PrintSuccess(format string, args ...interface{}) {
	u.theme.Success.Fprintf(u.out, format, args...)
}

// PrintError prints with error color
func (u *UI) PrintError(format string, args ...interface{}) {
	u.theme.Error.Fprintf(u.out, format, args...)
}

// PrintWarning prints with warning color
func (u *UI) PrintWarning(format string, args ...interface{}) {
	u.theme.Warning.Fprintf(u.out, format, args...)
}

// PrintInfo prints with info color
func (u *UI) PrintInfo(format string, args ...interface{}) {
	u.theme.Info.Fprintf(u.out, format, args...)
}

// Println prints a plain line
func (u *UI) Println(args ...interface{}) {
	fmt.Fprintln(u.out, args...)
}

// RenderProcessTable displays processes in a table
func (u *UI) RenderProcessTable(infos []*ProcessInfo, target string, mode TargetMode) {
	if u.quiet {
		return
	}

	u.PrintPrimary("\n🎯 Target: %s [mode=%s] (%d process(es))\n", target, mode, len(infos))

	table := tablewriter.NewWriter(u.out)
	table.Header([]string{"PID", "PPID", "Name", "User", "CPU%", "MEM%", "Threads", "Status", "Ports"})

	for _, info := range infos {
		ports := "none"
		if len(info.Ports) > 0 {
			var portStrs []string
			for _, p := range info.Ports[:min(len(info.Ports), 3)] {
				portStrs = append(portStrs, strconv.Itoa(p))
			}
			ports = strings.Join(portStrs, ", ")
			if len(info.Ports) > 3 {
				ports += fmt.Sprintf(" +%d", len(info.Ports)-3)
			}
		}

		table.Append([]string{
			strconv.Itoa(int(info.PID)),
			strconv.Itoa(int(info.PPID)),
			truncate(info.Name, 20),
			truncate(info.User, 12),
			fmt.Sprintf("%.1f", info.CPU),
			fmt.Sprintf("%.1f", info.Mem),
			strconv.Itoa(int(info.Threads)),
			info.Status,
			ports,
		})
	}
	table.Render()
}

// RenderProcessTree displays hierarchical process structure
func (u *UI) RenderProcessTree(infos []*ProcessInfo) {
	if u.quiet {
		return
	}

	seen := make(map[int32]bool)
	for _, root := range infos {
		u.renderTree(root, 0, seen)
	}
}

func (u *UI) renderTree(info *ProcessInfo, depth int, seen map[int32]bool) {
	if seen[info.PID] {
		return
	}
	seen[info.PID] = true

	prefix := strings.Repeat("  ", depth)
	if depth > 0 {
		prefix += "└── "
	}

	u.theme.Highlight.Fprintf(u.out, "%s%d %s [cpu:%.1f%% mem:%.1f%%]\n", prefix, info.PID, info.Name, info.CPU, info.Mem)

	for _, child := range info.Children {
		u.renderTree(child, depth+1, seen)
	}
}

// RenderKillResult displays kill operation results
func (u *UI) RenderKillResult(result *KillResult) {
	if u.quiet {
		return
	}

	if result.Killed > 0 {
		u.PrintSuccess("\n✓ Killed %d process(es) in %v\n", result.Killed, result.Duration)
	} else if result.Failed > 0 {
		u.PrintError("\n✗ Failed to kill %d process(es)\n", result.Failed)
	}
}

// RenderPortsTable displays listening ports
func (u *UI) RenderPortsTable(ports []PortInfo) {
	if u.quiet && len(ports) == 0 {
		return
	}

	if len(ports) == 0 {
		u.PrintWarning("No listening ports found\n")
		return
	}

	u.PrintPrimary("\n🔌 Listening Ports (%d found):\n", len(ports))

	table := tablewriter.NewWriter(u.out)
	table.Header([]string{"Protocol", "Port", "PID", "Process", "Command"})

	for _, p := range ports {
		table.Append([]string{
			p.Protocol,
			strconv.Itoa(p.Port),
			strconv.Itoa(int(p.PID)),
			truncate(p.Name, 20),
			p.Cmdline,
		})
	}
	table.Render()
}

// RenderPortsJSON outputs ports as JSON
func (u *UI) RenderPortsJSON(ports []PortInfo) {
	// JSON encoding handled by caller, this is just a placeholder
	// to keep interface consistent
}

// ConfirmKill prompts for user confirmation
func (u *UI) ConfirmKill(infos []*ProcessInfo, force bool) bool {
	if u.quiet {
		return true
	}

	var totalCPU, totalMem float64
	for _, info := range infos {
		totalCPU += info.CPU
		totalMem += float64(info.Mem)
	}

	u.PrintWarning("\n⚠ About to kill %d process(es)\n", len(infos))
	fmt.Fprintf(u.out, "   Total CPU usage: %.1f%%\n", totalCPU)
	fmt.Fprintf(u.out, "   Total memory: %.1f%%\n", totalMem)

	if force {
		u.PrintError("   WARNING: Force kill enabled (no cleanup)\n")
	}

	u.PrintWarning("\nContinue? [y/N] ")

	var response string
	fmt.Fscanln(os.Stdin, &response)
	response = strings.TrimSpace(strings.ToLower(response))

	return response == "y" || response == "yes"
}

// PrintUsage prints help text
func (u *UI) PrintUsage() {
	help := fmt.Sprintf(`
%s - Super Process Assassin v%s

%s:
  die 3000                    # Kill by port (auto-detected)
  die node                    # Kill by name substring
  die -p 8080                 # Explicit port mode
  die -n nginx                # Explicit name mode
  die -pid 1234               # Kill specific PID
  die -cgroup /docker/abc     # Kill by cgroup (containers)
  die --cpu-above 90          # Kill high CPU processes
  die --mem-above 80 nginx    # Kill high memory matching 'nginx'

%s:
  -f, --force                 # SIGKILL immediately (no graceful)
  -t, --timeout 5s            # Grace period before SIGKILL
  -a, --all                   # Kill all matches (default: first only)
  --tree                      # Kill entire process tree
  -r, --regex                 # Use regex for name matching

%s:
  --dry                       # Preview only (no actual kill)
  -i, --interactive           # Confirm before killing
  -q, --quiet                 # Suppress warnings
  -v, --verbose               # Detailed output
  --audit /var/log/die.log    # JSON audit trail

%s:
  -l, --list                  # List all listening ports
  -l --json                   # JSON output for ports
  -w, --watch 10s             # Watch mode (kill repeatedly)
  -j 8                        # Parallelism (default: CPU count)

%s:
  %s              # Force kill tree on port
  %s             # Regex kill all chrome
  %s  # Watch & kill hungry node
  %s             # Preview what would die
  %s # Kill by cgroup
`,
		u.theme.Primary.Sprint("die"),
		Version,
		u.theme.Warning.Sprint("TARGETING MODES"),
		u.theme.Warning.Sprint("KILL OPTIONS"),
		u.theme.Warning.Sprint("SAFETY & CONTROL"),
		u.theme.Warning.Sprint("DISCOVERY"),
		u.theme.Warning.Sprint("EXAMPLES"),
		u.theme.Success.Sprint("die -f --tree 3000"),
		u.theme.Success.Sprint("die -a -r \"chrome.*\""),
		u.theme.Success.Sprint("die -w 5s --mem-above 50 node"),
		u.theme.Success.Sprint("die --dry -a python"),
		u.theme.Success.Sprint("die -cgroup /system.slice/apache2"),
	)

	fmt.Fprint(u.out, help)
}

// Verbose prints verbose output
func (u *UI) Verbose(format string, args ...interface{}) {
	if u.verbose {
		u.theme.Info.Fprintf(u.out, format, args...)
	}
}

// Debug prints debug output
func (u *UI) Debug(format string, args ...interface{}) {
	if u.verbose {
		u.theme.Highlight.Fprintf(u.out, format, args...)
	}
}
