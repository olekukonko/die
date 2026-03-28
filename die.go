package die

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/olekukonko/ll"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// Killer executes process termination operations
type Killer struct {
	config Config
	logger *ll.Logger
	audit  io.WriteCloser
	stats  KillStats
	mu     sync.RWMutex
}

// NewKiller creates a new Killer instance
func NewKiller(config Config, logger *ll.Logger) (*Killer, error) {
	k := &Killer{
		config: config,
		logger: logger,
	}

	if config.AuditLog != "" {
		f, err := os.OpenFile(config.AuditLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
		if err != nil {
			return nil, fmt.Errorf("audit log: %w", err)
		}
		k.audit = f
	}

	return k, nil
}

// Close cleans up resources
func (k *Killer) Close() error {
	if k.audit != nil {
		return k.audit.Close()
	}
	return nil
}

// Kill performs the kill operation and returns structured results
func (k *Killer) Kill(ctx context.Context, target string, mode TargetMode) (*KillResult, error) {
	start := time.Now()
	result := &KillResult{}

	k.mu.Lock()
	k.stats = KillStats{}
	k.mu.Unlock()

	// Discovery phase
	pids, err := k.discover(ctx, target, mode)
	if err != nil {
		result.Error = err
		k.logAudit(AuditEntry{
			Timestamp:  time.Now(),
			Action:     "discover",
			Target:     target,
			Mode:       mode.String(),
			Success:    false,
			Error:      err.Error(),
			DurationMs: time.Since(start).Milliseconds(),
		})
		return result, err
	}

	if len(pids) == 0 {
		result.Duration = time.Since(start)
		return result, nil
	}

	// Enrich process info
	infos := k.enrichProcesses(ctx, pids)
	if len(infos) == 0 {
		result.Duration = time.Since(start)
		return result, nil
	}

	if k.config.Tree {
		infos = k.buildForest(infos)
	}

	// Store PIDs in result
	result.PIDs = extractPIDs(infos)

	if k.config.DryRun {
		k.logAudit(AuditEntry{
			Timestamp: time.Now(),
			Action:    "dry_run",
			Target:    target,
			Mode:      mode.String(),
			PIDs:      result.PIDs,
			Success:   true,
			DryRun:    true,
			Force:     k.config.Force,
			Tree:      k.config.Tree,
		})
		result.Killed = len(infos)
		result.Duration = time.Since(start)
		return result, nil
	}

	// Execute kills
	k.executeKills(infos)

	if !k.config.Force {
		survivors := k.waitAndReap(ctx, infos)
		k.mu.Lock()
		k.stats.Failed += int32(len(survivors))
		k.mu.Unlock()
	}

	k.mu.RLock()
	result.Killed = int(k.stats.Killed)
	result.Failed = int(k.stats.Failed)
	result.Skipped = int(k.stats.Skipped)
	k.mu.RUnlock()
	result.Duration = time.Since(start)

	// Audit log
	k.logAudit(AuditEntry{
		Timestamp:  time.Now(),
		Action:     "kill",
		Target:     target,
		Mode:       mode.String(),
		PIDs:       result.PIDs,
		Success:    result.Failed == 0,
		DryRun:     false,
		Force:      k.config.Force,
		Tree:       k.config.Tree,
		DurationMs: result.Duration.Milliseconds(),
	})

	return result, nil
}

// GetProcesses returns process info without killing (for discovery)
func (k *Killer) GetProcesses(ctx context.Context, target string, mode TargetMode) ([]*ProcessInfo, error) {
	pids, err := k.discover(ctx, target, mode)
	if err != nil {
		return nil, err
	}

	if len(pids) == 0 {
		return nil, nil
	}

	infos := k.enrichProcesses(ctx, pids)
	if k.config.Tree {
		infos = k.buildForest(infos)
	}

	return infos, nil
}

// GetStats returns current statistics
func (k *Killer) GetStats() KillStats {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.stats
}

// ListPorts returns all listening ports
func ListPorts(ctx context.Context) ([]PortInfo, error) {
	protocols := []string{"tcp", "tcp6", "udp", "udp6"}
	var ports []PortInfo
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, proto := range protocols {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()

			conns, err := net.ConnectionsWithContext(ctx, p)
			if err != nil {
				return
			}

			for _, c := range conns {
				if c.Status == "LISTEN" && c.Pid > 0 {
					proc, err := process.NewProcessWithContext(ctx, c.Pid)
					name := "unknown"
					cmdline := ""
					if err == nil {
						name, _ = proc.NameWithContext(ctx)
						cmdline, _ = proc.CmdlineWithContext(ctx)
					}

					mu.Lock()
					ports = append(ports, PortInfo{
						Protocol: strings.ToUpper(p),
						Port:     int(c.Laddr.Port),
						PID:      c.Pid,
						Name:     name,
						Cmdline:  truncate(cmdline, 50),
					})
					mu.Unlock()
				}
			}
		}(proto)
	}

	wg.Wait()

	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Port < ports[j].Port
	})

	return ports, nil
}

// discover finds PIDs based on target and mode
func (k *Killer) discover(ctx context.Context, target string, mode TargetMode) ([]int32, error) {
	switch mode {
	case ModePort:
		port, err := strconv.Atoi(target)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
		return k.findByPort(ctx, port)
	case ModePID:
		pid, err := strconv.Atoi(target)
		if err != nil {
			return nil, fmt.Errorf("invalid PID: %w", err)
		}
		if pid > 0 {
			return []int32{int32(pid)}, nil
		}
		return nil, nil
	case ModeCPUAbove:
		threshold, _ := strconv.ParseFloat(target, 64)
		return k.findByResource(ctx, target, threshold)
	case ModeMemAbove:
		threshold, _ := strconv.ParseFloat(target, 64)
		return k.findByResource(ctx, target, threshold)
	default:
		return k.findByName(ctx, target, mode)
	}
}

// findByPort discovers PIDs listening on a specific port
func (k *Killer) findByPort(ctx context.Context, port int) ([]int32, error) {
	protocols := []string{"tcp", "tcp6", "udp", "udp6"}
	var allPids []int32
	seen := make(map[int32]bool)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, proto := range protocols {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()

			conns, err := net.ConnectionsWithContext(ctx, p)
			if err != nil {
				return
			}

			for _, conn := range conns {
				if conn.Laddr.Port == uint32(port) && conn.Status == "LISTEN" && conn.Pid > 0 {
					mu.Lock()
					if !seen[conn.Pid] {
						seen[conn.Pid] = true
						allPids = append(allPids, conn.Pid)
					}
					mu.Unlock()
				}
			}
		}(proto)
	}

	wg.Wait()
	return allPids, nil
}

// findByName discovers processes by name/cmdline
func (k *Killer) findByName(ctx context.Context, pattern string, mode TargetMode) ([]int32, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var re *regexp.Regexp
	if k.config.Regex {
		re, err = regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
	}

	var pids []int32
	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, k.config.Parallelism)

	for _, p := range procs {
		wg.Add(1)
		go func(proc *process.Process) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			name, err := proc.NameWithContext(ctx)
			if err != nil {
				return
			}

			var match bool
			if k.config.Regex {
				cmdline, _ := proc.CmdlineWithContext(ctx)
				match = re.MatchString(name) || re.MatchString(cmdline)
			} else {
				lowerPattern := strings.ToLower(pattern)
				match = strings.Contains(strings.ToLower(name), lowerPattern)

				if !match && mode == ModeAuto {
					cmdline, _ := proc.CmdlineWithContext(ctx)
					match = strings.Contains(strings.ToLower(cmdline), lowerPattern)
				}
			}

			if match {
				mu.Lock()
				pids = append(pids, proc.Pid)
				mu.Unlock()
			}
		}(p)
	}

	wg.Wait()

	return pids, nil
}

// findByResource discovers processes by resource usage
func (k *Killer) findByResource(ctx context.Context, target string, threshold float64) ([]int32, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var pids []int32
	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, k.config.Parallelism)

	for _, p := range procs {
		wg.Add(1)
		go func(proc *process.Process) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if k.config.Regex && target != "" {
				name, _ := proc.NameWithContext(ctx)
				if !strings.Contains(strings.ToLower(name), strings.ToLower(target)) {
					return
				}
			}

			cpuPercent, _ := proc.CPUPercentWithContext(ctx)
			memPercent, _ := proc.MemoryPercentWithContext(ctx)

			if threshold > 0 && threshold < 100 && (cpuPercent > threshold || float64(memPercent) > threshold) {
				mu.Lock()
				pids = append(pids, proc.Pid)
				mu.Unlock()
			}
		}(p)
	}

	wg.Wait()
	return pids, nil
}

// enrichProcesses gathers detailed info for PIDs concurrently
func (k *Killer) enrichProcesses(ctx context.Context, pids []int32) []*ProcessInfo {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, k.config.Parallelism)
	results := make(chan *ProcessInfo, len(pids))

	for _, pid := range pids {
		wg.Add(1)
		go func(p int32) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			info := k.getProcessInfo(ctx, p)
			if info != nil {
				results <- info
			}
		}(pid)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var infos []*ProcessInfo
	for info := range results {
		infos = append(infos, info)
	}

	return infos
}

// getProcessInfo gathers comprehensive process metadata
func (k *Killer) getProcessInfo(ctx context.Context, pid int32) *ProcessInfo {
	p, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return nil
	}

	info := &ProcessInfo{PID: pid}
	info.Name, _ = p.NameWithContext(ctx)
	info.Cmdline, _ = p.CmdlineWithContext(ctx)
	info.User, _ = p.UsernameWithContext(ctx)
	info.CPU, _ = p.CPUPercentWithContext(ctx)
	info.Mem, _ = p.MemoryPercentWithContext(ctx)
	info.PPID, _ = p.PpidWithContext(ctx)
	info.Threads, _ = p.NumThreadsWithContext(ctx)

	if memInfo, err := p.MemoryInfoWithContext(ctx); err == nil {
		info.MemRSS = memInfo.RSS
	}

	if status, err := p.StatusWithContext(ctx); err == nil && len(status) > 0 {
		info.Status = status[0]
	} else {
		info.Status = "unknown"
	}

	if createTime, err := p.CreateTimeWithContext(ctx); err == nil {
		info.StartTime = createTime
	}

	info.Cgroup = k.getCgroup(pid)

	if conns, err := p.ConnectionsWithContext(ctx); err == nil {
		portMap := make(map[int]bool)
		for _, c := range conns {
			if c.Laddr.Port > 0 {
				portMap[int(c.Laddr.Port)] = true
			}
		}
		for port := range portMap {
			info.Ports = append(info.Ports, port)
		}
		sort.Ints(info.Ports)
	}

	return info
}

// getCgroup reads cgroup for Linux processes
func (k *Killer) getCgroup(pid int32) string {
	if runtime.GOOS != "linux" {
		return ""
	}

	cgroupPath := fmt.Sprintf("/proc/%d/cgroup", pid)
	data, err := os.ReadFile(cgroupPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) >= 3 {
			return strings.TrimSpace(parts[2])
		}
	}
	return ""
}

// buildForest organizes processes into parent-child trees
func (k *Killer) buildForest(infos []*ProcessInfo) []*ProcessInfo {
	pidMap := make(map[int32]*ProcessInfo)
	for _, info := range infos {
		pidMap[info.PID] = info
	}

	var roots []*ProcessInfo
	for _, info := range infos {
		if parent, ok := pidMap[info.PPID]; ok && parent.PID != info.PID {
			parent.Children = append(parent.Children, info)
		} else {
			roots = append(roots, info)
		}
	}

	return roots
}

// executeKills performs the actual termination
func (k *Killer) executeKills(infos []*ProcessInfo) {
	if k.config.Tree {
		k.killForest(infos)
		return
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, min(k.config.Parallelism, 8))

	for _, info := range infos {
		wg.Add(1)
		go func(i *ProcessInfo) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			k.killSingle(i)
		}(info)
	}

	wg.Wait()
}

// killForest kills process trees
func (k *Killer) killForest(roots []*ProcessInfo) {
	for _, root := range roots {
		k.killTreeRecursive(root)
	}
}

// killTreeRecursive kills children before parent
func (k *Killer) killTreeRecursive(info *ProcessInfo) {
	for _, child := range info.Children {
		k.killTreeRecursive(child)
	}
	k.killSingle(info)
}

// killSingle terminates one process
func (k *Killer) killSingle(info *ProcessInfo) bool {
	k.mu.Lock()
	k.stats.Attempted++
	k.mu.Unlock()

	if info.PID == int32(os.Getpid()) {
		k.mu.Lock()
		k.stats.Skipped++
		k.mu.Unlock()
		if k.logger != nil {
			k.logger.Fields("pid", info.PID).Debug("skipping self-kill")
		}
		return false
	}

	proc, err := process.NewProcess(info.PID)
	if err != nil {
		k.mu.Lock()
		k.stats.Skipped++
		k.mu.Unlock()
		return false
	}

	var killErr error
	if k.config.Force {
		killErr = proc.Kill()
	} else {
		killErr = proc.Terminate()
	}

	if killErr != nil {
		k.mu.Lock()
		k.stats.Failed++
		k.mu.Unlock()
		return false
	}

	k.mu.Lock()
	k.stats.Killed++
	k.mu.Unlock()
	return true
}

// waitAndReap waits for grace period and force-kills survivors
func (k *Killer) waitAndReap(ctx context.Context, infos []*ProcessInfo) []int32 {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(k.config.Timeout)

	for {
		select {
		case <-timeout:
			goto forceKill
		case <-ticker.C:
			allDead := true
			for _, info := range infos {
				if isProcessRunning(info.PID) {
					allDead = false
					break
				}
			}
			if allDead {
				return nil
			}
		case <-ctx.Done():
			goto forceKill
		}
	}

forceKill:
	var survivors []int32
	for _, info := range infos {
		if isProcessRunning(info.PID) {
			survivors = append(survivors, info.PID)
		}
	}

	for _, pid := range survivors {
		if proc, err := process.NewProcess(pid); err == nil {
			proc.Kill()
		}
	}

	return survivors
}

// logAudit writes structured audit entry
func (k *Killer) logAudit(entry AuditEntry) {
	if k.audit == nil {
		return
	}

	entry.Version = Version
	entry.User = os.Getenv("USER")
	if entry.User == "" {
		entry.User = os.Getenv("USERNAME")
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	data = append(data, '\n')
	k.audit.Write(data)
}
