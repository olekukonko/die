package die

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// RenderKillResult

func TestRenderKillResult_DryRunNoMatch(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI()
	ui.SetOutput(buf)

	ui.RenderKillResult(&KillResult{DryRun: true, Killed: 0})

	if !strings.Contains(buf.String(), "No matching") {
		t.Errorf("expected 'No matching processes found', got: %q", buf.String())
	}
}

func TestRenderKillResult_DryRunWithMatches(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI()
	ui.SetOutput(buf)

	ui.RenderKillResult(&KillResult{DryRun: true, Killed: 5})

	out := buf.String()
	if !strings.Contains(out, "dry run") {
		t.Errorf("expected '[dry run]' banner, got: %q", out)
	}
	if !strings.Contains(out, "--kill") {
		t.Errorf("expected '--kill' hint in dry-run output, got: %q", out)
	}
	// Must NOT say "Killed" (that would imply processes were actually killed)
	if strings.Contains(out, "✓ Killed") {
		t.Errorf("dry-run must not print success kill message, got: %q", out)
	}
}

func TestRenderKillResult_NoProcesses(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI()
	ui.SetOutput(buf)

	// Live run with zero results
	ui.RenderKillResult(&KillResult{DryRun: false})

	if !strings.Contains(buf.String(), "No matching") {
		t.Errorf("expected 'No matching processes found', got: %q", buf.String())
	}
}

func TestRenderKillResult_Success(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI()
	ui.SetOutput(buf)

	ui.RenderKillResult(&KillResult{Killed: 2, Duration: 312 * time.Millisecond})

	if !strings.Contains(buf.String(), "Killed 2") {
		t.Errorf("expected 'Killed 2', got: %q", buf.String())
	}
}

func TestRenderKillResult_Failure(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI()
	ui.SetOutput(buf)

	ui.RenderKillResult(&KillResult{Failed: 3, Duration: 125 * time.Millisecond})

	if !strings.Contains(buf.String(), "Failed to kill 3") {
		t.Errorf("expected 'Failed to kill 3', got: %q", buf.String())
	}
}

func TestRenderKillResult_Skipped(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI()
	ui.SetOutput(buf)

	ui.RenderKillResult(&KillResult{Skipped: 1})

	if !strings.Contains(buf.String(), "Skipped 1") {
		t.Errorf("expected 'Skipped 1', got: %q", buf.String())
	}
}

func TestRenderKillResult_Mixed(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI()
	ui.SetOutput(buf)

	ui.RenderKillResult(&KillResult{Killed: 5, Failed: 2, Skipped: 1, Duration: 500 * time.Millisecond})

	out := buf.String()
	if !strings.Contains(out, "Killed 5") {
		t.Errorf("expected 'Killed 5', got: %q", out)
	}
	if !strings.Contains(out, "Failed to kill 2") {
		t.Errorf("expected 'Failed to kill 2', got: %q", out)
	}
	if !strings.Contains(out, "Skipped 1") {
		t.Errorf("expected 'Skipped 1', got: %q", out)
	}
}

// Quiet mode

func TestUI_QuietMode(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI(WithQuiet(true))
	ui.SetOutput(buf)

	ui.RenderKillResult(&KillResult{Killed: 5})

	if buf.String() != "" {
		t.Errorf("quiet mode should produce no output, got: %q", buf.String())
	}
}

// Verbose mode

func TestUI_VerboseMode(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI(WithVerbose(true))
	ui.SetOutput(buf)

	ui.Verbose("Test verbose message\n")

	if !strings.Contains(buf.String(), "Test verbose message") {
		t.Errorf("expected verbose message, got: %q", buf.String())
	}
}

// Self-kill exclusion

func TestEnrichProcesses_ExcludesSelf(t *testing.T) {
	cfg := Config{DryRun: true, Quiet: true, KillEnabled: false}.WithDefaults()
	k, err := NewKiller(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()

	selfPID := int32(os.Getpid())
	infos := k.enrichProcesses(t.Context(), []int32{selfPID})

	for _, info := range infos {
		if info.PID == selfPID {
			t.Errorf("enrichProcesses returned self (pid=%d); it must be excluded", selfPID)
		}
	}
}

// collectAllPIDs (tree traversal)

func TestCollectAllPIDs_Flat(t *testing.T) {
	infos := []*ProcessInfo{{PID: 1}, {PID: 2}, {PID: 3}}
	pids := collectAllPIDs(infos)
	if len(pids) != 3 {
		t.Errorf("expected 3 pids, got %d", len(pids))
	}
}

func TestCollectAllPIDs_Tree(t *testing.T) {
	child1 := &ProcessInfo{PID: 20}
	child2 := &ProcessInfo{PID: 21}
	parent := &ProcessInfo{PID: 10, Children: []*ProcessInfo{child1, child2}}
	root := &ProcessInfo{PID: 1, Children: []*ProcessInfo{parent}}

	pids := collectAllPIDs([]*ProcessInfo{root})
	if len(pids) != 4 {
		t.Errorf("expected 4 pids (root + parent + 2 children), got %d: %v", len(pids), pids)
	}

	pidSet := make(map[int32]bool)
	for _, p := range pids {
		pidSet[p] = true
	}
	for _, expected := range []int32{1, 10, 20, 21} {
		if !pidSet[expected] {
			t.Errorf("missing pid %d in collected pids %v", expected, pids)
		}
	}
}

// extractPIDs (top-level only, unchanged contract)

func TestExtractPIDs(t *testing.T) {
	tests := []struct {
		name     string
		infos    []*ProcessInfo
		expected []int32
	}{
		{name: "empty", infos: []*ProcessInfo{}, expected: []int32{}},
		{name: "single", infos: []*ProcessInfo{{PID: 1234}}, expected: []int32{1234}},
		{name: "multiple", infos: []*ProcessInfo{{PID: 1}, {PID: 2}, {PID: 3}}, expected: []int32{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pids := extractPIDs(tt.infos)
			if len(pids) != len(tt.expected) {
				t.Fatalf("expected %d pids, got %d", len(tt.expected), len(pids))
			}
			for i := range tt.expected {
				if pids[i] != tt.expected[i] {
					t.Errorf("index %d: expected %d, got %d", i, tt.expected[i], pids[i])
				}
			}
		})
	}
}

// flattenInfos

func TestFlattenInfos_Deep(t *testing.T) {
	grandchild := &ProcessInfo{PID: 300}
	child := &ProcessInfo{PID: 200, Children: []*ProcessInfo{grandchild}}
	root := &ProcessInfo{PID: 100, Children: []*ProcessInfo{child}}

	flat := flattenInfos([]*ProcessInfo{root})
	if len(flat) != 3 {
		t.Fatalf("expected 3 flattened entries, got %d", len(flat))
	}
}

func TestConfig_IsDryRun_ZeroValue(t *testing.T) {
	// A zero-value Config must always be dry-run — safe by default
	cfg := Config{}
	if !cfg.IsDryRun() {
		t.Error("zero-value Config must be dry-run (KillEnabled defaults to false)")
	}
}

func TestConfig_IsDryRun_KillEnabledOnly(t *testing.T) {
	// KillEnabled=true and DryRun=false → live kill
	cfg := Config{KillEnabled: true, DryRun: false}
	if cfg.IsDryRun() {
		t.Error("KillEnabled=true, DryRun=false must NOT be dry-run")
	}
}

func TestConfig_IsDryRun_KillEnabledWithDryFlag(t *testing.T) {
	// --kill --dry together → --dry wins (explicit preview)
	cfg := Config{KillEnabled: true, DryRun: true}
	if !cfg.IsDryRun() {
		t.Error("explicit --dry must override --kill")
	}
}

func TestConfig_IsDryRun_KillDisabledWithDryFalse(t *testing.T) {
	// KillEnabled=false even when DryRun=false → still dry-run
	cfg := Config{KillEnabled: false, DryRun: false}
	if !cfg.IsDryRun() {
		t.Error("KillEnabled=false must always produce dry-run regardless of DryRun flag")
	}
}

func TestConfig_WithDefaults_ZeroValues(t *testing.T) {
	cfg := Config{}.WithDefaults()
	if cfg.Timeout <= 0 {
		t.Errorf("expected positive Timeout, got %v", cfg.Timeout)
	}
	if cfg.Parallelism <= 0 {
		t.Errorf("expected positive Parallelism, got %d", cfg.Parallelism)
	}
}

func TestConfig_WithDefaults_PreservesExplicit(t *testing.T) {
	cfg := Config{Timeout: 30 * time.Second, Parallelism: 16}.WithDefaults()
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", cfg.Timeout)
	}
	if cfg.Parallelism != 16 {
		t.Errorf("expected parallelism 16, got %d", cfg.Parallelism)
	}
}

// IsNumeric

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"3000", true},
		{"0", true},
		{"-123", true},
		{"abc", false},
		{"12a", false},
		{"", false},
		{"12.34", false},
		{"  42  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if IsNumeric(tt.input) != tt.expected {
				t.Errorf("IsNumeric(%q) = %v, want %v", tt.input, IsNumeric(tt.input), tt.expected)
			}
		})
	}
}

// buildForest

func TestBuildForest_Roots(t *testing.T) {
	infos := []*ProcessInfo{
		{PID: 1, PPID: 0},
		{PID: 2, PPID: 0},
	}
	cfg := Config{KillEnabled: false}.WithDefaults()
	k, _ := NewKiller(cfg, nil)
	roots := k.buildForest(infos)
	if len(roots) != 2 {
		t.Errorf("expected 2 roots, got %d", len(roots))
	}
}

func TestBuildForest_ParentChild(t *testing.T) {
	parent := &ProcessInfo{PID: 10, PPID: 1}
	child := &ProcessInfo{PID: 20, PPID: 10}
	cfg := Config{KillEnabled: false}.WithDefaults()
	k, _ := NewKiller(cfg, nil)
	roots := k.buildForest([]*ProcessInfo{parent, child})

	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if len(roots[0].Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(roots[0].Children))
	}
	if roots[0].Children[0].PID != 20 {
		t.Errorf("expected child PID 20, got %d", roots[0].Children[0].PID)
	}
}

// Benchmarks

func BenchmarkIsNumeric(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IsNumeric("12345")
	}
}

func BenchmarkExtractPIDs(b *testing.B) {
	infos := make([]*ProcessInfo, 1000)
	for i := 0; i < 1000; i++ {
		infos[i] = &ProcessInfo{PID: int32(i)}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractPIDs(infos)
	}
}

func BenchmarkCollectAllPIDs_DeepTree(b *testing.B) {
	// 10-level deep chain of 1000 nodes total
	root := &ProcessInfo{PID: 1}
	cur := root
	for i := 2; i <= 1000; i++ {
		child := &ProcessInfo{PID: int32(i)}
		cur.Children = []*ProcessInfo{child}
		cur = child
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collectAllPIDs([]*ProcessInfo{root})
	}
}

// helpers

func withTimeout(t *testing.T, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("test timed out")
	}
}
