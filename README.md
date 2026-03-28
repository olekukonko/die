# die – Process Assassination Tool

`die` is a powerful CLI tool to find and kill processes by port, name, resource usage, or cgroup. It provides a clean interface, safe dry-runs, audit logging, and watch mode.

## Installation

```bash
go install github.com/olekukonko/die/cmd/die@latest
```

## Quick Start

| Task                           | Command          |
|--------------------------------|------------------|
| Kill a `node` on system        | `die node`       |
| Kill `all` node processes      | `die -a node`    |
| Kill process using `port` 3000 | `die 3000`       |
| Kill process by `PID`            | `die -pid 1234`  |
| Dry-run (preview only)         | `die --dry 3000` |
| List listening ports           | `die -l`         |

## Targeting Modes

`die` automatically detects the target type, or you can specify it explicitly.

| Mode | Flag | Example |
|------|------|---------|
| Port | `-p` | `die -p 8080` |
| Name | `-n` | `die -n nginx` |
| PID | `-pid` | `die -pid 1234` |
| Cgroup | `-cgroup` | `die -cgroup /docker/abc` |
| CPU above threshold | `--cpu-above` | `die --cpu-above 90` |
| Memory above threshold | `--mem-above` | `die --mem-above 80` |

**Auto-detection rules:**
- Numeric argument → port mode
- Alphanumeric → name substring search

## Kill Options

| Flag | Description |
|------|-------------|
| `-f, --force` | SIGKILL immediately (no graceful termination) |
| `-t, --timeout` | Grace period before SIGKILL (default: 3s) |
| `-a, --all` | Kill all matching processes (default: first only) |
| `--tree` | Kill entire process tree (children then parent) |
| `-r, --regex` | Use regex for name/cmdline matching |

## Safety & Control

| Flag | Description |
|------|-------------|
| `--dry` | Preview what would be killed – no actual kill |
| `-i, --interactive` | Confirm before killing |
| `-q, --quiet` | Suppress non-essential output |
| `-v, --verbose` | Show detailed debug information |
| `--audit` | Write JSON audit log to file |

## Discovery & Monitoring

| Flag | Description |
|------|-------------|
| `-l, --list` | Show all listening ports with process info |
| `-l --json` | Same as above, JSON output |
| `-w, --watch` | Watch mode – kill matching processes repeatedly |

## Examples

### Kill by port with grace period
```bash
die -t 5s 3000
```

### Force kill entire tree on port 8080
```bash
die -f --tree 8080
```

### Kill all processes matching "chrome" using regex
```bash
die -a -r "chrome.*"
```

### Kill by memory usage (above 50%) matching "node"
```bash
die --mem-above 50 node
```

### Watch mode: every 5s, kill node processes using > 50% CPU
```bash
die -w 5s --cpu-above 50 node
```

### Preview (dry-run) before killing
```bash
die --dry -a python
```

### Kill all processes inside a cgroup (containers)
```bash
die -cgroup /system.slice/apache2
```

### Audit log for forensics
```bash
die --audit /var/log/die.log 3000
```

## Output Examples

### Process table preview
```
🎯 Target: 3000 [mode=port] (2 process(es))
+-------+-------+--------+-------+------+------+---------+--------+-------+
|  PID  | PPID  |  NAME  | USER  | CPU% | MEM% | THREADS | STATUS | PORTS |
+-------+-------+--------+-------+------+------+---------+--------+-------+
|  1234 |   1   | node   | root  | 2.5  | 12.3 |    8    |  S     | 3000  |
|  5678 |  1234 | npm    | root  | 0.0  | 1.2  |    2    |  S     | none  |
+-------+-------+--------+-------+------+------+---------+--------+-------+
```

### Kill result
```
✓ Killed 2 process(es) in 312ms
```

### Listening ports
```
🔌 Listening Ports (5 found):
+----------+------+-------+---------+--------------------------+
| PROTOCOL | PORT |  PID  | PROCESS | COMMAND                  |
+----------+------+-------+---------+--------------------------+
| TCP      | 3000 | 1234  | node    | /usr/bin/node app.js     |
| TCP      | 5432 | 5678  | postgres| postgres -D /var/lib/... |
+----------+------+-------+---------+--------------------------+
```

## Audit Log Format

When `--audit` is enabled, JSON entries are appended:

```json
{
  "timestamp": "2026-03-28T10:30:00Z",
  "action": "kill",
  "target": "3000",
  "mode": "port",
  "pids": [1234, 5678],
  "success": true,
  "user": "oleku",
  "force": false,
  "tree": false,
  "duration_ms": 312,
  "version": "v1.0.0"
}
```

## Build from Source

```bash
git clone https://github.com/olekukonko/die.git
cd die
go build -o die ./cmd/die
```

Set version info:
```bash
go build -ldflags="-X github.com/olekukonko/die.Version=v1.0.0 -X github.com/olekukonko/die.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o die ./cmd/die
```

## License

MIT License – see [LICENSE](LICENSE) for details.
```

The README is organized for quick scanning with:
- **Tables** for flags instead of dense paragraphs
- **Examples** section covering common workflows (including the `--dry` flag placement you discovered)
- **Output examples** to show what users will see
- **Audit log format** for users who need forensics
- **Installation and build** instructions

Let me know if you'd like to add a section on CI integration, shell aliases, or platform-specific notes!