# NegaLog

> Missing Log Detection Tool - Find the logs that *should* exist but don't.

---

**Monday morning.** You walk in to find the payment processing job stopped running Friday at 6pm. No errors. No alerts. It just...stopped. Three days of orders sitting unprocessed. $47,000 in delayed revenue. Angry customers. A manager asking "how did no one notice?"

The job didn't fail - it silently disappeared. Your monitoring watched for errors, but there were none. The absence of expected logs went undetected.

**This is why NegaLog exists.** Instead of watching for what went wrong, it watches for what *should have happened but didn't*.

---

## Table of Contents

- [What is NegaLog?](#what-is-negalog)
- [Features](#features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Configuration](#configuration)
- [Detection Strategies](#detection-strategies)
- [Webhooks](#webhooks)
- [Plugins](#plugins)
- [CLI Reference](#cli-reference)
- [Examples](#examples)
- [Use Cases](#use-cases)
- [Development](#development)

## What is NegaLog?

Traditional log analysis tools search for patterns that exist. NegaLog inverts the paradigm: **define what logs *should* exist, and it reports what's missing.**

| Value | What you get | Why it matters |
|-------|--------------|----------------|
| Detect incomplete transactions | Find requests that started but never completed | Catch hung processes, timeouts, and silent failures before users report them |
| Monitor scheduled jobs | Know when periodic tasks stop running | Prevent cascade failures from missed cron jobs, heartbeats, or health checks |
| Verify error handling | Confirm errors trigger expected responses | Ensure alerts fire, retries happen, and nothing falls through the cracks |

## Features

| Feature | Description |
|---------|-------------|
| **Auto-Detect Timestamps** | Automatically identify timestamp formats in any log file |
| **Sequence Gap Detection** | Find start events without matching end events |
| **Periodic Absence Detection** | Detect missing recurring logs (heartbeats, health checks) |
| **Conditional Absence Detection** | Find triggers without expected consequences |
| **Cross-Service Correlation** | Track sequences across multiple log files via correlation IDs |
| **Flexible Output** | Human-readable text or machine-parseable JSON |
| **Time Range Filtering** | Analyze specific time windows |
| **Rule Selection** | Run specific rules only |
| **Webhook Notifications** | Send analysis results to external endpoints (Slack, PagerDuty, etc.) |
| **Plugin Support** | Extend functionality with standalone plugin binaries (like kubectl/git) |

## Quick Start

```bash
# Install (Linux amd64)
curl -sL https://github.com/ccollicutt/negalog/releases/latest/download/negalog-linux-amd64 -o negalog
chmod +x negalog
sudo mv negalog /usr/local/bin/

# Auto-detect timestamp format in your log file
negalog detect /var/log/syslog

# Generate a starter config from detection
negalog detect --write-config negalog.yaml /var/log/syslog

# Edit the config to add your detection rules, then validate
negalog validate negalog.yaml

# Run analysis
negalog analyze negalog.yaml
```

### Manual Configuration

If you prefer to create the config manually:

```bash
cat > negalog.yaml << 'EOF'
log_sources:
  - /var/log/app/*.log

timestamp_format:
  pattern: '^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]'
  layout: "2006-01-02 15:04:05"

rules:
  - name: request-completion
    type: sequence
    start_pattern: 'REQUEST_START id=(\w+)'
    end_pattern: 'REQUEST_END id=(\w+)'
    correlation_field: 1
    timeout: 60s
EOF
```

## Installation

### Prerequisites

- Go 1.25+ (install with `make install-go`)

### Build from Source

```bash
# Clone and build
git clone <repo>
cd negalog
make build

# Binary is at ./bin/negalog
./bin/negalog version
```

### Install Go (if needed)

```bash
make install-go  # Installs Go 1.25.5
```

## Configuration

NegaLog uses YAML configuration files:

```yaml
# Log files to analyze (supports glob patterns)
log_sources:
  - /var/log/app/*.log
  - /var/log/service-*.log

# How to extract timestamps from log lines
timestamp_format:
  pattern: '^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]'  # Regex with capture group
  layout: "2006-01-02 15:04:05"  # Go time format

# Detection rules
rules:
  - name: my-rule
    type: sequence|periodic|conditional
    # ... type-specific fields
```

### Timestamp Formats

| Log Format | Pattern | Layout |
|------------|---------|--------|
| `[2024-01-15 10:30:00]` | `^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]` | `2006-01-02 15:04:05` |
| `2024-01-15T10:30:00Z` | `^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})` | `2006-01-02T15:04:05` |
| `Jan 15 10:30:00` | `^(\w{3}\s+\d+\s+\d{2}:\d{2}:\d{2})` | `Jan  2 15:04:05` |

## Detection Strategies

### Sequence Rules

Detect when expected log pairs are incomplete:

```yaml
- name: request-completion
  type: sequence
  description: "Every request should complete"
  start_pattern: 'REQUEST_START id=(\w+)'
  end_pattern: 'REQUEST_END id=(\w+)'
  correlation_field: 1  # Capture group for correlation ID
  timeout: 60s          # Max time between start and end
```

### Periodic Rules

Detect when recurring logs don't appear at expected intervals:

```yaml
- name: heartbeat-check
  type: periodic
  description: "Heartbeat should occur every 5 minutes"
  pattern: 'HEARTBEAT service=auth'
  max_gap: 5m              # Maximum allowed gap
  min_occurrences: 10      # Optional: minimum count required
```

### Conditional Rules

Detect when a trigger should produce a log but doesn't:

```yaml
- name: error-alert
  type: conditional
  description: "Errors should trigger alerts"
  trigger_pattern: 'ERROR code=(\d+)'
  expected_pattern: 'ALERT_SENT code=(\d+)'
  correlation_field: 1  # Optional: link trigger to consequence
  timeout: 10s
```

## Webhooks

Send analysis results to external endpoints when issues are detected. Useful for integrating with alerting systems like Slack, PagerDuty, or custom dashboards.

### CLI Flags

```bash
# Send results to a webhook URL
negalog analyze config.yaml --webhook-url https://hooks.slack.com/services/...

# With bearer token authentication
negalog analyze config.yaml \
  --webhook-url https://api.example.com/alerts \
  --webhook-token $TOKEN

# Control when webhooks fire
negalog analyze config.yaml \
  --webhook-url https://example.com/hook \
  --webhook-trigger on_issues  # only when issues detected (default)
```

### Webhook Options

| Flag | Description | Default |
|------|-------------|---------|
| `--webhook-url` | Webhook endpoint URL | none |
| `--webhook-token` | Bearer token for authentication | none |
| `--webhook-trigger` | When to fire: `on_issues`, `always`, `never` | on_issues |

### Config File

Configure multiple webhooks with different triggers:

```yaml
log_sources:
  - /var/log/app/*.log

rules:
  - name: heartbeat-check
    type: periodic
    pattern: 'HEARTBEAT'
    max_gap: 5m

webhooks:
  - name: slack-alerts
    url: "https://hooks.slack.com/services/..."
    trigger: on_issues  # only fire when issues detected
    timeout: 10s

  - name: metrics-collector
    url: "https://metrics.internal/negalog"
    token: "${METRICS_TOKEN}"  # supports env var expansion
    trigger: always  # always send results
```

### Webhook Payload

Webhooks receive the same JSON structure as `negalog analyze -o json`:

```json
{
  "Summary": {
    "RulesChecked": 3,
    "RulesWithIssues": 2,
    "TotalIssues": 5,
    "LinesProcessed": 1000
  },
  "Results": [...],
  "Metadata": {
    "ConfigFile": "config.yaml",
    "Sources": ["app.log"],
    "AnalyzedAt": "2024-01-15T10:00:00Z",
    "Duration": 1500000
  }
}
```

### Trigger Modes

| Trigger | Behavior |
|---------|----------|
| `on_issues` | Fire only when issues are detected (default) |
| `always` | Fire after every analysis run |
| `never` | Disable webhook (useful for temporary disable) |

## Plugins

NegaLog supports plugins for extended functionality. Plugins are standalone binaries that are automatically discovered and executed when you run an unknown command.

This follows the same pattern used by `kubectl` and `git` for plugins.

### How It Works

When you run `negalog <command>`, NegaLog first checks if it's a built-in command. If not, it searches for a plugin binary named `negalog-<command>` in these locations (in order):

1. Same directory as the `negalog` binary
2. `~/.negalog/plugins/`
3. Anywhere in your `PATH`

If found, NegaLog executes the plugin with all remaining arguments passed through. stdin/stdout/stderr are connected, and the plugin's exit code is propagated.

### Installing Plugins

```bash
# Option 1: Place in plugins directory
mkdir -p ~/.negalog/plugins
cp negalog-watch ~/.negalog/plugins/
chmod +x ~/.negalog/plugins/negalog-watch

# Option 2: Place in PATH
sudo cp negalog-watch /usr/local/bin/
chmod +x /usr/local/bin/negalog-watch

# Option 3: Place alongside negalog binary
cp negalog-watch /usr/local/bin/  # if negalog is in /usr/local/bin/
```

### Using Plugins

Once installed, use plugins like built-in commands:

```bash
# If negalog-watch is installed, these are equivalent:
negalog watch config.yaml --interval 30s
negalog-watch config.yaml --interval 30s
```

### Available Plugins

#### Commercial Plugins

| Plugin | Description | License | Info |
|--------|-------------|---------|------|
| `watch` | Continuous log monitoring with real-time alerting | Commercial | [negalog-watch](https://collicutt.net/software/negalog/#negalog-watch) |

#### Open Source Plugins

*None yet. Want to build one? See [Writing Plugins](#writing-plugins) below.*

### Writing Plugins

Plugins are standalone binaries. They can be written in any language and can import NegaLog's Go packages as a library:

```go
import (
    "github.com/ccollicutt/negalog/pkg/config"
    "github.com/ccollicutt/negalog/pkg/parser"
    "github.com/ccollicutt/negalog/pkg/analyzer"
)
```

Requirements:
- Binary name must be `negalog-<command>`
- Must be executable (`chmod +x`)
- Should handle `--help` for documentation
- Should use standard exit codes (0=success, 1=issues found, 2=error)

## CLI Reference

### Commands

```bash
negalog detect <log-file>       # Auto-detect timestamp format
negalog diagnose <config-file>  # Diagnose configuration issues
negalog analyze <config-file>   # Analyze logs for missing entries
negalog validate <config-file>  # Validate configuration file
negalog version                 # Print version information
```

### Detect Options

| Flag | Description | Default |
|------|-------------|---------|
| `-o, --output` | Output format (text\|json) | text |
| `-n, --sample` | Number of lines to sample | 100 |
| `--all` | Show all detected formats | false |
| `--write-config` | Write starter config to file | none |

### Diagnose Options

| Flag | Description | Default |
|------|-------------|---------|
| `-v, --verbose` | Show detailed diagnostic output | false |

### Analyze Options

| Flag | Description | Default |
|------|-------------|---------|
| `-o, --output` | Output format (text\|json) | text |
| `--time-range` | Limit analysis window (e.g., 2h, 24h) | none |
| `--rule` | Run specific rule(s) only | all |
| `-v, --verbose` | Show detailed output | false |
| `-q, --quiet` | Summary only | false |
| `--webhook-url` | Send results to webhook endpoint | none |
| `--webhook-token` | Bearer token for webhook auth | none |
| `--webhook-trigger` | When to fire: on_issues\|always\|never | on_issues |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | No missing logs detected |
| 1 | Missing logs detected |
| 2 | Configuration or runtime error |

## Examples

### Try It Yourself (Using Repo Test Data)

After cloning and building, you can run NegaLog against included sample data:

```bash
# Clone and build
git clone https://github.com/ccollicutt/negalog.git
cd negalog
make build

# Detect timestamp format in sample logs
./bin/negalog detect testdata/logs/heartbeat.log

# Run heartbeat analysis (detects gaps in periodic logs)
./bin/negalog analyze testdata/configs/heartbeat.yaml

# Run cross-service analysis (detects incomplete request flows)
./bin/negalog analyze testdata/configs/cross_service.yaml

# JSON output for scripting
./bin/negalog analyze -o json testdata/configs/heartbeat.yaml
```

### Test Webhooks with webhook.site

Test webhook integration using [webhook.site](https://webhook.site) - a free service that captures HTTP requests:

```bash
# 1. Go to https://webhook.site and copy your unique URL

# 2. Run analysis with webhook
./bin/negalog analyze testdata/configs/heartbeat.yaml \
  --webhook-url https://webhook.site/YOUR-UUID-HERE \
  --webhook-trigger always

# 3. Check webhook.site to see the JSON payload that was sent
```

The repo includes a config for webhook e2e testing:

```bash
# Uses testdata/configs/webhook_e2e.yaml with CLI webhook override
./bin/negalog analyze testdata/configs/webhook_e2e.yaml \
  --webhook-url https://webhook.site/YOUR-UUID-HERE \
  --webhook-trigger always
```

### Detecting Timestamp Formats

```bash
# Detect format in a log file
./bin/negalog detect /var/log/syslog

# Output:
# === Timestamp Format Detection ===
# File: /var/log/syslog
# Lines sampled: 100
# Lines with timestamps: 100
#
# Detected Format: Syslog (BSD)
# Confidence: 100.0% (100/100 lines matched)
# Sample: Jun 14 15:16:01 myhost sshd[1234]: Accepted publickey
#
# --- Configuration snippet ---
# timestamp_format:
#   pattern: '^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})'
#   layout: "Jan 2 15:04:05"

# Show all detected formats (not just the best match)
./bin/negalog detect --all /var/log/app.log

# JSON output for scripting
./bin/negalog detect -o json /var/log/app.log

# Generate a starter config file
./bin/negalog detect --write-config myapp.yaml /var/log/app.log
```

### Diagnosing Configuration Issues

```bash
# Check config for common problems
./bin/negalog diagnose config.yaml

# Output:
# === NegaLog Configuration Diagnostics ===
#
# [PASS] Config File
#     Found: config.yaml (245 bytes)
#
# [PASS] Config Syntax
#     Config file parsed successfully
#
# [PASS] Log Source: /var/log/app.log
#     File exists (1024 bytes)
#
# [PASS] Timestamp Format
#     Timestamp pattern is valid
#
# [FAIL] Pattern Test: app.log
#     Pattern matches no lines in log file
#     Hint: Detected format: ISO 8601
#     Hint: Suggested pattern: ^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})
#
# ---
# Summary: 4 passed, 0 warnings, 1 errors

# Verbose output for more details
./bin/negalog diagnose -v config.yaml
```

### Basic Analysis

```bash
# Text output (default)
./bin/negalog analyze config.yaml

# JSON output
./bin/negalog analyze -o json config.yaml

# Quiet summary
./bin/negalog analyze -q config.yaml
```

### Filtering

```bash
# Last 2 hours only
./bin/negalog analyze --time-range 2h config.yaml

# Specific rules only
./bin/negalog analyze --rule request-completion --rule heartbeat config.yaml
```

### Sample Output

```
=== NegaLog Analysis Report ===

[SEQUENCE] request-completion
  Missing: 2 issue(s)
  - id=abc123: started at 10:05:03, no end (timeout: 1m0s)
  - id=def456: started at 10:07:22, no end (timeout: 1m0s)

[PERIODIC] heartbeat-check
  Missing: 1 issue(s)
  - Gap of 8m30s between 10:15:00 and 10:23:30 (max allowed: 5m0s)

---
Summary: 2 rules checked, 2 rules with issues, 3 total issues
```

## Use Cases

### Detecting Stuck Transactions

**Problem:** Your payment service processes orders but sometimes requests hang indefinitely. No error is logged - the request just disappears.

**Solution:** A sequence rule tracks request start/end pairs:

```yaml
- name: payment-completion
  type: sequence
  start_pattern: 'PAYMENT_START order_id=(\w+)'
  end_pattern: 'PAYMENT_COMPLETE order_id=(\w+)'
  correlation_field: 1
  timeout: 2m
```

**What NegaLog reports:**
```
order_id=ORD-7829: started at 14:32:01, no completion (timeout: 2m)
order_id=ORD-7834: started at 14:33:15, no completion (timeout: 2m)
```

Now you know exactly which orders are stuck, not just that "something is slow."

---

### Monitoring Scheduled Jobs

**Problem:** Your nightly data sync job runs via cron. One night it silently fails to start. Nobody notices until customers complain about stale data three days later.

**Solution:** A periodic rule expects the job to log at regular intervals:

```yaml
- name: nightly-sync
  type: periodic
  pattern: 'DATA_SYNC_COMPLETE'
  max_gap: 25h  # Should run daily, allow some buffer
```

**What NegaLog reports:**
```
Gap of 73h between 2024-01-12 02:00:00 and 2024-01-15 03:00:00 (max: 25h)
```

Three missed runs detected. You can alert on this before customers notice.

---

### Verifying Alert Delivery

**Problem:** Your application logs errors, and your alerting system should send notifications. But sometimes alerts fail silently - the error happened, but no one was notified.

**Solution:** A conditional rule ensures every error triggers an alert:

```yaml
- name: error-alerts
  type: conditional
  trigger_pattern: 'CRITICAL error_id=(\w+)'
  expected_pattern: 'ALERT_SENT error_id=(\w+)'
  correlation_field: 1
  timeout: 60s
```

**What NegaLog reports:**
```
CRITICAL error_id=ERR-5521 at 09:14:33 had no ALERT_SENT within 60s
```

Your alerting system dropped an alert. Now you know to investigate why.

---

### Cross-Service Request Tracking

**Problem:** A user request flows through API gateway, auth service, and backend. Sometimes requests enter the system but never complete - lost somewhere in the chain.

**Solution:** Track requests across multiple log files using a shared correlation ID:

```yaml
log_sources:
  - /var/log/api-gateway/*.log
  - /var/log/auth-service/*.log
  - /var/log/backend/*.log

rules:
  - name: request-flow
    type: sequence
    start_pattern: 'REQUEST_RECEIVED trace_id=(\w+)'
    end_pattern: 'REQUEST_COMPLETED trace_id=(\w+)'
    correlation_field: 1
    timeout: 30s
```

NegaLog merges all logs by timestamp and tracks each trace_id across services. If a request enters but never completes anywhere, you'll know.

## Development

See [DEV.md](DEV.md) for build commands, project structure, and testing.

## License

MIT License
