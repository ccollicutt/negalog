package detector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDetector_DetectFromLines_ISO8601(t *testing.T) {
	lines := []string{
		"2024-01-15T10:30:00 Application started",
		"2024-01-15T10:30:05 Processing request",
		"2024-01-15T10:30:10 Request completed",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "ISO 8601" {
		t.Errorf("Expected ISO 8601, got %s", best.Format.Name)
	}

	if best.Confidence != 1.0 {
		t.Errorf("Expected 100%% confidence, got %.1f%%", best.Confidence*100)
	}
}

func TestDetector_DetectFromLines_Syslog(t *testing.T) {
	lines := []string{
		"Jun 14 15:16:01 combo sshd(pam_unix)[19939]: authentication failure",
		"Jun 14 15:16:02 combo sshd[19939]: Failed password for root",
		"Jun 14 15:16:03 combo sshd[19939]: Connection closed",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Syslog (BSD)" {
		t.Errorf("Expected Syslog (BSD), got %s", best.Format.Name)
	}

	if best.MatchCount != 3 {
		t.Errorf("Expected 3 matches, got %d", best.MatchCount)
	}
}

func TestDetector_DetectFromLines_Bracketed(t *testing.T) {
	lines := []string{
		"[2024-01-15 10:30:00] INFO Application started",
		"[2024-01-15 10:30:05] DEBUG Processing request",
		"[2024-01-15 10:30:10] INFO Request completed",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Bracketed datetime" {
		t.Errorf("Expected Bracketed datetime, got %s", best.Format.Name)
	}
}

func TestDetector_DetectFromLines_ApacheCLF(t *testing.T) {
	lines := []string{
		`192.168.1.1 - - [15/Jun/2024:10:30:00 +0000] "GET /index.html HTTP/1.1" 200 1234`,
		`192.168.1.2 - - [15/Jun/2024:10:30:05 +0000] "POST /api/data HTTP/1.1" 201 567`,
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Apache/NGINX CLF" {
		t.Errorf("Expected Apache/NGINX CLF, got %s", best.Format.Name)
	}
}

func TestDetector_DetectFromLines_PythonLogging(t *testing.T) {
	lines := []string{
		"2024-01-15 10:30:00,123 INFO Starting application",
		"2024-01-15 10:30:00,456 DEBUG Initializing modules",
		"2024-01-15 10:30:00,789 INFO Application ready",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Python logging" {
		t.Errorf("Expected Python logging, got %s", best.Format.Name)
	}
}

func TestDetector_DetectFromLines_UnixTimestamp(t *testing.T) {
	lines := []string{
		"1705315800 Application started",
		"1705315805 Processing request",
		"1705315810 Request completed",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Unix timestamp (seconds)" {
		t.Errorf("Expected Unix timestamp (seconds), got %s", best.Format.Name)
	}
}

func TestDetector_DetectFromLines_NoMatch(t *testing.T) {
	lines := []string{
		"No timestamp here",
		"Just some text",
		"More random content",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if result.HasMatch() {
		t.Errorf("Expected no match, got %s", result.BestMatch().Format.Name)
	}
}

func TestDetector_DetectFromLines_EmptyInput(t *testing.T) {
	d := New()
	result := d.DetectFromLines([]string{})

	if result.HasMatch() {
		t.Error("Expected no match for empty input")
	}

	if result.SampledLines != 0 {
		t.Errorf("Expected 0 sampled lines, got %d", result.SampledLines)
	}
}

func TestDetector_DetectFromLines_MixedFormats(t *testing.T) {
	// Most lines are syslog, one is ISO
	lines := []string{
		"Jun 14 15:16:01 server app[1234]: Started",
		"Jun 14 15:16:02 server app[1234]: Processing",
		"Jun 14 15:16:03 server app[1234]: Done",
		"2024-01-15T10:30:00 Outlier line",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	// Syslog should win with 75% confidence
	if best.Format.Name != "Syslog (BSD)" {
		t.Errorf("Expected Syslog (BSD), got %s", best.Format.Name)
	}

	expectedConfidence := 0.75
	if best.Confidence != expectedConfidence {
		t.Errorf("Expected confidence %.2f, got %.2f", expectedConfidence, best.Confidence)
	}
}

func TestDetector_DetectFromLines_SkipsComments(t *testing.T) {
	lines := []string{
		"# This is a comment",
		"2024-01-15T10:30:00 Real log line",
		"# Another comment",
		"2024-01-15T10:30:05 Another real line",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	// Should match 2 out of 4 lines (50%), but we count non-comment lines
	best := result.BestMatch()
	if best.Format.Name != "ISO 8601" {
		t.Errorf("Expected ISO 8601, got %s", best.Format.Name)
	}
}

func TestDetector_DetectFromLines_AmbiguousFormat(t *testing.T) {
	lines := []string{
		"01/05/2024 10:30:00 Some event",
		"01/06/2024 10:30:05 Another event",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if !best.Format.Ambiguous {
		t.Error("Expected format to be marked as ambiguous")
	}

	if result.AmbiguityNote == "" {
		t.Error("Expected ambiguity note to be set")
	}
}

func TestDetector_WithSampleSize(t *testing.T) {
	d := New(WithSampleSize(50))
	if d.sampleSize != 50 {
		t.Errorf("Expected sample size 50, got %d", d.sampleSize)
	}
}

func TestDetector_WithSampleSize_Invalid(t *testing.T) {
	d := New(WithSampleSize(-1))
	if d.sampleSize != 100 {
		t.Errorf("Expected default sample size 100, got %d", d.sampleSize)
	}
}

func TestDetector_DetectFromFile(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.log")

	content := `Jun 14 15:16:01 server app[1234]: Started
Jun 14 15:16:02 server app[1234]: Processing
Jun 14 15:16:03 server app[1234]: Done
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	d := New()
	result, err := d.DetectFromFile(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("DetectFromFile failed: %v", err)
	}

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Syslog (BSD)" {
		t.Errorf("Expected Syslog (BSD), got %s", best.Format.Name)
	}
}

func TestDetector_DetectFromFile_NotFound(t *testing.T) {
	d := New()
	_, err := d.DetectFromFile(context.Background(), "/nonexistent/file.log")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestDetector_ISO8601WithTimezone(t *testing.T) {
	lines := []string{
		"2024-01-15T10:30:00+00:00 Event 1",
		"2024-01-15T10:30:05-05:00 Event 2",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "ISO 8601 with timezone" {
		t.Errorf("Expected ISO 8601 with timezone, got %s", best.Format.Name)
	}
}

func TestDetector_ISO8601WithZ(t *testing.T) {
	lines := []string{
		"2024-01-15T10:30:00Z Event 1",
		"2024-01-15T10:30:05Z Event 2",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "ISO 8601 with Z (UTC)" {
		t.Errorf("Expected ISO 8601 with Z (UTC), got %s", best.Format.Name)
	}
}

func TestDetector_Log4j(t *testing.T) {
	lines := []string{
		"2024-01-15 10:30:00.123 INFO Starting",
		"2024-01-15 10:30:00.456 DEBUG Processing",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Log4j/Java logging" {
		t.Errorf("Expected Log4j/Java logging, got %s", best.Format.Name)
	}
}

func TestDefaultFormats(t *testing.T) {
	formats := DefaultFormats()
	if len(formats) == 0 {
		t.Error("Expected default formats to be non-empty")
	}

	// Verify all formats have compiled patterns
	for _, f := range formats {
		if f.Pattern == nil {
			t.Errorf("Format %s has nil pattern", f.Name)
		}
		if f.PatternStr == "" {
			t.Errorf("Format %s has empty pattern string", f.Name)
		}
		if f.Layout == "" {
			t.Errorf("Format %s has empty layout", f.Name)
		}
		if len(f.Examples) == 0 {
			t.Errorf("Format %s has no examples", f.Name)
		}
	}
}

func TestDetector_HDFSCompact(t *testing.T) {
	lines := []string{
		"081109 203615 148 INFO dfs.DataNode: Started",
		"081109 203807 222 INFO dfs.DataNode: Processing",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "HDFS compact" {
		t.Errorf("Expected HDFS compact, got %s", best.Format.Name)
	}
}

func TestDetector_ApacheErrorLog(t *testing.T) {
	lines := []string{
		"[Sun Dec 04 04:47:44 2005] [notice] workerEnv.init() ok",
		"[Sun Dec 04 04:47:45 2005] [error] mod_jk error",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Apache error log" {
		t.Errorf("Expected Apache error log, got %s", best.Format.Name)
	}
}

func TestDetector_SparkHadoop(t *testing.T) {
	lines := []string{
		"17/06/09 20:10:40 INFO executor.Backend: Registered",
		"17/06/09 20:10:41 INFO spark.SecurityManager: Started",
	}

	d := New()
	result := d.DetectFromLines(lines)

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Spark/Hadoop short date" {
		t.Errorf("Expected Spark/Hadoop short date, got %s", best.Format.Name)
	}
}

func TestDetector_RealSyslogFile(t *testing.T) {
	// Test against the actual syslog file in testdata
	logFile := filepath.Join("..", "..", "testdata", "logs", "linux_syslog.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatalf("Required test file not found: %s", logFile)
	}

	d := New(WithSampleSize(200))
	result, err := d.DetectFromFile(context.Background(), logFile)
	if err != nil {
		t.Fatalf("DetectFromFile failed: %v", err)
	}

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Syslog (BSD)" {
		t.Errorf("Expected Syslog (BSD), got %s", best.Format.Name)
	}

	t.Logf("Detected: %s with %.1f%% confidence", best.Format.Name, best.Confidence*100)
}

func TestDetector_RealHeartbeatFile(t *testing.T) {
	// Test against the heartbeat file in testdata
	logFile := filepath.Join("..", "..", "testdata", "logs", "heartbeat.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatalf("Required test file not found: %s", logFile)
	}

	d := New()
	result, err := d.DetectFromFile(context.Background(), logFile)
	if err != nil {
		t.Fatalf("DetectFromFile failed: %v", err)
	}

	if !result.HasMatch() {
		t.Fatal("Expected to detect a format")
	}

	best := result.BestMatch()
	if best.Format.Name != "Syslog (BSD)" {
		t.Errorf("Expected Syslog (BSD), got %s", best.Format.Name)
	}

	t.Logf("Detected: %s with %.1f%% confidence", best.Format.Name, best.Confidence*100)
}
