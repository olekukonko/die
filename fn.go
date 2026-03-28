package die

import "github.com/shirou/gopsutil/v3/process"

// Helper functions
func isProcessRunning(pid int32) bool {
	p, err := process.NewProcess(pid)
	if err != nil {
		return false
	}
	running, _ := p.IsRunning()
	return running
}

func extractPIDs(infos []*ProcessInfo) []int32 {
	pids := make([]int32, len(infos))
	for i, info := range infos {
		pids[i] = info.PID
	}
	return pids
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
