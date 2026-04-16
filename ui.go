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

// RenderKillResult displays kill operation results.
//
// When the run was a dry-run (preview), it prints a clear banner showing what
// would have been killed and how to proceed. When live, it prints the outcome.
func (u *UI) RenderKillResult(result *KillResult) {
	if u.quiet {
		return
	}

	if result.DryRun {
		if result.Killed == 0 {
			u.PrintWarning("\nNo matching processes found\n")
			return
		}
		u.PrintWarning("\n[dry run] %d process(es) would be killed — no action taken\n", result.Killed)
		u.PrintInfo("           Re-run with --kill to terminate them\n")
		return
	}

	if result.Killed > 0 {
		u.PrintSuccess("\n✓ Killed %d process(es) in %v\n", result.Killed, result.Duration)
	}
	if result.Failed > 0 {
		u.PrintError("✗ Failed to kill %d process(es)\n", result.Failed)
	}
	if result.Skipped > 0 {
		u.PrintWarning("⚠ Skipped %d process(es)\n", result.Skipped)
	}
	if result.Killed == 0 && result.Failed == 0 && result.Skipped == 0 {
		u.PrintWarning("\nNo matching processes found\n")
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

// flattenInfos returns all ProcessInfo nodes from a forest (roots + children),
// used so ConfirmKill can show accurate totals even in --tree mode.
func flattenInfos(infos []*ProcessInfo) []*ProcessInfo {
	var flat []*ProcessInfo
	var walk func([]*ProcessInfo)
	walk = func(nodes []*ProcessInfo) {
		for _, n := range nodes {
			flat = append(flat, n)
			if len(n.Children) > 0 {
				walk(n.Children)
			}
		}
	}
	walk(infos)
	return flat
}

// ConfirmKill prompts for user confirmation before a destructive operation.
// It accepts the infos slice as returned by Kill (may be a forest in --tree mode)
// and flattens it internally so totals include all descendants.
func (u *UI) ConfirmKill(infos []*ProcessInfo, force bool) bool {
	if u.quiet {
		return true
	}

	flat := flattenInfos(infos)

	var totalCPU, totalMem float64
	for _, info := range flat {
		totalCPU += info.CPU
		totalMem += float64(info.Mem)
	}

	u.PrintWarning("\n⚠ About to kill %d process(es)\n", len(flat))
	fmt.Fprintf(u.out, "   Total CPU usage: %.1f%%\n", totalCPU)
	fmt.Fprintf(u.out, "   Total memory:    %.1f%%\n", totalMem)

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

%s (safe by default — always previews without %s):
  die 3000                    # Preview what listens on port 3000
  die node                    # Preview matching processes by name
  die -p 8080                 # Explicit port mode (preview)
  die -n nginx                # Explicit name mode (preview)
  die -pid 1234               # Preview specific PID
  die -cgroup /docker/abc     # Preview by cgroup (containers)
  die --cpu-above 90          # Preview high CPU processes
  die --mem-above 80          # Preview high memory processes

%s (add %s to actually kill):
  die node --kill             # Kill by name substring
  die 3000 --kill             # Kill by port
  die -f --tree 3000 --kill   # Force kill entire process tree on port
  die -a -r "chrome.*" --kill # Regex kill all chrome instances

%s:
  --kill                      # Perform actual kill (required; default is preview)
  -f, --force                 # SIGKILL immediately (no graceful shutdown)
  -t, --timeout 5s            # Grace period before SIGKILL
  -a, --all                   # Kill all matches (default: first only)
  --tree                      # Kill entire process tree
  -r, --regex                 # Use regex for name matching

%s:
  --dry                       # Explicit preview (same as omitting --kill)
  -i, --interactive           # Confirm before killing
  -q, --quiet                 # Suppress output
  -v, --verbose               # Detailed output
  --audit /var/log/die.log    # JSON audit trail

%s:
  -l, --list                  # List all listening ports
  -l --json                   # JSON output for ports
  -w, --watch 10s             # Watch mode (kill repeatedly)
  -j 8                        # Parallelism (default: CPU count)

%s:
  %s         # Preview what would be killed
  %s  # Actually kill it
  %s      # Preview entire process tree
  %s   # Watch & kill hungry node (live)
`,
		u.theme.Primary.Sprint("die"),
		Version,
		u.theme.Warning.Sprint("TARGETING MODES"),
		u.theme.Error.Sprint("--kill"),
		u.theme.Warning.Sprint("LIVE KILL"),
		u.theme.Error.Sprint("--kill"),
		u.theme.Warning.Sprint("KILL OPTIONS"),
		u.theme.Warning.Sprint("SAFETY & CONTROL"),
		u.theme.Warning.Sprint("DISCOVERY"),
		u.theme.Warning.Sprint("EXAMPLES"),
		u.theme.Success.Sprint("die node"),
		u.theme.Error.Sprint("die node --kill"),
		u.theme.Success.Sprint("die --tree 3000"),
		u.theme.Error.Sprint("die -w 5s --mem-above 50 node --kill"),
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
