package die

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRenderKillResult_NoProcesses(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI()
	ui.SetOutput(buf)

	result := &KillResult{
		Killed:  0,
		Failed:  0,
		Skipped: 0,
	}

	ui.RenderKillResult(result)

	output := buf.String()
	// RenderKillResult shows nothing for empty results (quiet mode)
	// This is expected behavior - it only shows success/failure messages
	if strings.Contains(output, "Killed") || strings.Contains(output, "Failed") {
		t.Errorf("expected no output for empty result, got: %s", output)
	}
}

func TestRenderKillResult_NoProcesses_WithVerbose(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI(WithVerbose(true))
	ui.SetOutput(buf)

	result := &KillResult{
		Killed:  0,
		Failed:  0,
		Skipped: 0,
	}

	ui.RenderKillResult(result)

	output := buf.String()
	// Verbose mode still doesn't show anything for empty results
	if strings.Contains(output, "Killed") || strings.Contains(output, "Failed") {
		t.Errorf("expected no output for empty result, got: %s", output)
	}
}

func TestRenderKillResult_Success(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI()
	ui.SetOutput(buf)

	result := &KillResult{
		Killed:   2,
		Failed:   0,
		Skipped:  0,
		Duration: 312000000, // 312ms
	}

	ui.RenderKillResult(result)

	output := buf.String()
	if !strings.Contains(output, "Killed 2") {
		t.Errorf("expected success message 'Killed 2', got: %s", output)
	}
}

func TestRenderKillResult_Failure(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI()
	ui.SetOutput(buf)

	result := &KillResult{
		Killed:   0,
		Failed:   3,
		Skipped:  0,
		Duration: 125000000, // 125ms
	}

	ui.RenderKillResult(result)

	output := buf.String()
	if !strings.Contains(output, "Failed to kill 3") {
		t.Errorf("expected failure message 'Failed to kill 3', got: %s", output)
	}
}

//func TestRenderKillResult_Mixed(t *testing.T) {
//	buf := &bytes.Buffer{}
//	ui := NewUI()
//	ui.SetOutput(buf)
//
//	result := &KillResult{
//		Killed:   5,
//		Failed:   2,
//		Skipped:  1,
//		Duration: 500000000, // 500ms
//	}
//
//	ui.RenderKillResult(result)
//
//	output := buf.String()
//	// Should show success for killed processes
//	if !strings.Contains(output, "Killed 5") {
//		t.Errorf("expected success message 'Killed 5', got: %s", output)
//	}
//	// Should also show failure for failed processes
//	if !strings.Contains(output, "Failed to kill 2") {
//		t.Errorf("expected failure message 'Failed to kill 2', got: %s", output)
//	}
//}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"3000", true},
		{"0", true},
		{"-123", true}, // Negative numbers are numeric
		{"abc", false},
		{"12a", false},
		{"", false},
		{"12.34", false}, // Float not considered numeric for port detection
		{"  42  ", true}, // Spaces are trimmed? Check implementation
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if IsNumeric(tt.input) != tt.expected {
				t.Errorf("IsNumeric(%q) = %v, want %v", tt.input, IsNumeric(tt.input), tt.expected)
			}
		})
	}
}

//func TestKill_DryRun_NoMatch(t *testing.T) {
//	logger := ll.New("test")
//
//	k, err := NewKiller(Config{
//		DryRun: true,
//		Quiet:  true, // Suppress output during tests
//	}, logger)
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer k.Close()
//
//	// Add a timeout to prevent hanging
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	// Use a completely unrealistic pattern that shouldn't match any process
//	result, err := k.Kill(ctx, "XYZ123_NONE_EXISTENT_PROCESS_987654321", ModeName)
//	if err != nil {
//		t.Fatalf("Kill returned error: %v", err)
//	}
//
//	if result.Killed != 0 {
//		t.Errorf("expected 0 killed, got %d", result.Killed)
//	}
//
//	if result.Failed != 0 {
//		t.Errorf("expected 0 failed, got %d", result.Failed)
//	}
//
//	if result.Skipped != 0 {
//		t.Errorf("expected 0 skipped, got %d", result.Skipped)
//	}
//}

//func TestKill_DryRun_WithMatch(t *testing.T) {
//	logger := ll.New("test")
//
//	// Create a test process or use a known system process that's safe
//	// This test might need to be adjusted based on your environment
//
//	k, err := NewKiller(Config{
//		DryRun: true,
//		Quiet:  true,
//	}, logger)
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer k.Close()
//
//	ctx := context.Background()
//
//	// Use a common system process that should exist
//	result, err := k.Kill(ctx, "init", ModeName)
//	if err != nil {
//		t.Fatalf("Kill returned error: %v", err)
//	}
//
//	// Dry run should have killed nothing
//	if result.Killed != 0 {
//		t.Errorf("dry run: expected 0 killed, got %d", result.Killed)
//	}
//
//	// Should have found some processes (or not, depending on system)
//	if result.Failed != 0 {
//		t.Errorf("dry run: expected 0 failed, got %d", result.Failed)
//	}
//}

//func TestGetProcesses_NoMatch(t *testing.T) {
//	logger := ll.New("test")
//
//	k, err := NewKiller(Config{
//		Quiet: true,
//	}, logger)
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer k.Close()
//
//	ctx := context.Background()
//
//	infos, err := k.GetProcesses(ctx, "XYZ123_NONE_EXISTENT_PROCESS_987654321", ModeName)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	if infos == nil {
//		// Should return empty slice, not nil
//		if infos != nil {
//			t.Errorf("expected empty slice, got nil")
//		}
//	} else if len(infos) != 0 {
//		t.Errorf("expected 0 processes, got %d", len(infos))
//	}
//}

func TestExtractPIDs(t *testing.T) {
	tests := []struct {
		name     string
		infos    []*ProcessInfo
		expected []int32
	}{
		{
			name:     "empty slice",
			infos:    []*ProcessInfo{},
			expected: []int32{},
		},
		{
			name: "single process",
			infos: []*ProcessInfo{
				{PID: 1234},
			},
			expected: []int32{1234},
		},
		{
			name: "multiple processes",
			infos: []*ProcessInfo{
				{PID: 1},
				{PID: 2},
				{PID: 3},
			},
			expected: []int32{1, 2, 3},
		},
		{
			name: "with duplicate PIDs",
			infos: []*ProcessInfo{
				{PID: 1},
				{PID: 1},
				{PID: 2},
			},
			expected: []int32{1, 1, 2}, // extractPIDs doesn't deduplicate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pids := extractPIDs(tt.infos)

			if len(pids) != len(tt.expected) {
				t.Fatalf("expected %d pids, got %d", len(tt.expected), len(pids))
			}

			for i := range tt.expected {
				if pids[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %d, got %d", i, tt.expected[i], pids[i])
				}
			}
		})
	}
}

func TestUI_QuietMode(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI(WithQuiet(true))
	ui.SetOutput(buf)

	result := &KillResult{
		Killed: 5,
		Failed: 0,
	}

	ui.RenderKillResult(result)

	output := buf.String()
	if output != "" {
		t.Errorf("quiet mode should produce no output, got: %q", output)
	}
}

func TestUI_VerboseMode(t *testing.T) {
	buf := &bytes.Buffer{}
	ui := NewUI(WithVerbose(true))
	ui.SetOutput(buf)

	// Verbose mode doesn't affect RenderKillResult directly
	// It's used for debug/verbose logging
	ui.Verbose("Test verbose message\n")

	output := buf.String()
	if !strings.Contains(output, "Test verbose message") {
		t.Errorf("expected verbose message, got: %q", output)
	}
}

// Benchmark tests
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

func withTimeout(t *testing.T, fn func()) {
	done := make(chan bool)
	go func() {
		fn()
		done <- true
	}()

	select {
	case <-done:
		return
	case <-time.After(30 * time.Second):
		t.Fatal("test timed out")
	}
}
