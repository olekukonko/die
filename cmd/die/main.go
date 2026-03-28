package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/olekukonko/die"
	"github.com/olekukonko/ll"
	"github.com/olekukonko/ll/lh"
	"github.com/olekukonko/ll/lx"
)

func main() {
	var (
		portFlag     = flag.Bool("p", false, "Force interpretation as port")
		nameFlag     = flag.Bool("n", false, "Force interpretation as name")
		pidFlag      = flag.Bool("pid", false, "Force interpretation as PID")
		cgroupFlag   = flag.Bool("cgroup", false, "Kill by cgroup name/path")
		cpuAboveFlag = flag.Float64("cpu-above", 0, "Kill processes using more than X% CPU")
		memAboveFlag = flag.Float64("mem-above", 0, "Kill processes using more than X% memory")
		regexFlag    = flag.Bool("r", false, "Treat target as regular expression")
		forceFlag    = flag.Bool("f", false, "Force kill immediately (SIGKILL)")
		timeoutFlag  = flag.Duration("t", 3*time.Second, "Grace period before SIGKILL")
		treeFlag     = flag.Bool("tree", false, "Kill entire process tree")
		allFlag      = flag.Bool("a", false, "Kill all matching processes")
		dryRunFlag   = flag.Bool("dry", false, "Show what would be killed without killing")
		interactive  = flag.Bool("i", false, "Interactive mode (confirm before kill)")
		quietFlag    = flag.Bool("q", false, "Quiet mode")
		verboseFlag  = flag.Bool("v", false, "Verbose output")
		auditFlag    = flag.String("audit", "", "Write audit log to file")
		listFlag     = flag.Bool("l", false, "List listening ports")
		jsonFlag     = flag.Bool("json", false, "JSON output (for -l)")
		watchFlag    = flag.Duration("w", 0, "Watch mode (e.g. 5s)")
		parallelFlag = flag.Int("j", runtime.NumCPU(), "Parallelism")
		helpFlag     = flag.Bool("h", false, "Help")
		versionFlag  = flag.Bool("version", false, "Version")
	)

	flag.Usage = func() {
		die.NewUI().PrintUsage()
	}
	flag.Parse()

	ui := die.NewUI(
		die.WithQuiet(*quietFlag),
		die.WithVerbose(*verboseFlag),
	)

	if *helpFlag {
		ui.PrintUsage()
		return
	}

	if *versionFlag {
		fmt.Printf("die version %s\n", die.Version)
		return
	}

	if *listFlag {
		listPorts(ui, *jsonFlag)
		return
	}

	args := flag.Args()
	if len(args) == 0 && *cpuAboveFlag == 0 && *memAboveFlag == 0 {
		ui.PrintError("✗ No target specified\n")
		os.Exit(1)
	}

	// Determine mode
	mode := die.ModeAuto
	switch {
	case *portFlag:
		mode = die.ModePort
	case *nameFlag:
		mode = die.ModeName
	case *pidFlag:
		mode = die.ModePID
	case *cgroupFlag:
		mode = die.ModeCgroup
	case *cpuAboveFlag > 0:
		mode = die.ModeCPUAbove
	case *memAboveFlag > 0:
		mode = die.ModeMemAbove
	}

	if mode == die.ModeAuto && len(args) > 0 && die.IsNumeric(args[0]) {
		mode = die.ModePort
	}

	config := die.Config{
		Force:       *forceFlag,
		Timeout:     *timeoutFlag,
		Verbose:     *verboseFlag,
		DryRun:      *dryRunFlag,
		Interactive: *interactive,
		Quiet:       *quietFlag,
		Tree:        *treeFlag,
		All:         *allFlag,
		Regex:       *regexFlag,
		AuditLog:    *auditFlag,
		Parallelism: *parallelFlag,
	}

	logger := setupLogger(config.Verbose)

	killer, err := die.NewKiller(config, logger)
	if err != nil {
		ui.PrintError("✗ Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	defer killer.Close()

	target := ""
	if len(args) > 0 {
		target = args[0]
	}

	ctx := context.Background()

	// -----------------------------
	// PREVIEW (always show table)
	// -----------------------------
	if !config.Quiet && mode != die.ModePID {
		infos, err := killer.GetProcesses(ctx, target, mode)
		if err == nil && len(infos) > 0 {
			if config.Tree {
				ui.RenderProcessTree(infos)
			} else {
				ui.RenderProcessTable(infos, target, mode)
			}
		} else if err == nil {
			ui.PrintWarning("⚠ No matching processes found\n")
		}

		// interactive confirmation
		if config.Interactive && len(infos) > 0 {
			if !ui.ConfirmKill(infos, config.Force) {
				ui.PrintWarning("Cancelled\n")
				return
			}
		}
	}

	// -----------------------------
	// DRY RUN (exit early)
	// -----------------------------
	if config.DryRun {
		return
	}

	// -----------------------------
	// WATCH MODE
	// -----------------------------
	if *watchFlag > 0 {
		watcher := die.NewWatcher(killer, die.WatcherConfig{
			Interval: *watchFlag,
			OnKill: func(result *die.KillResult) {
				ui.RenderKillResult(result)
				if !config.Quiet {
					fmt.Println(strings.Repeat("-", 50))
				}
			},
		})

		watcher.Run(ctx, target, mode)
		return
	}

	// -----------------------------
	// EXECUTE KILL
	// -----------------------------
	result, err := killer.Kill(ctx, target, mode)
	if err != nil && !config.Quiet {
		ui.PrintError("✗ Error: %v\n", err)
		os.Exit(1)
	}

	if !config.Quiet {
		ui.RenderKillResult(result)
	}

	if result.Failed > 0 {
		os.Exit(1)
	}
}

func setupLogger(verbose bool) *ll.Logger {
	level := lx.LevelError
	if verbose {
		level = lx.LevelDebug
	}
	return ll.New("die", ll.WithHandler(lh.NewColorizedHandler(os.Stdout)), ll.WithLevel(level))
}

func listPorts(ui *die.UI, jsonOutput bool) {
	ctx := context.Background()
	ports, err := die.ListPorts(ctx)
	if err != nil {
		ui.PrintError("Failed to list ports: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(ports)
	} else {
		ui.RenderPortsTable(ports)
	}
}
