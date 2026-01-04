package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindPlugin_NotFound(t *testing.T) {
	_, err := FindPlugin("nonexistent-plugin-xyz")
	if err != ErrPluginNotFound {
		t.Errorf("expected ErrPluginNotFound, got %v", err)
	}
}

func TestFindPlugin_InPluginsDir(t *testing.T) {
	// Create a temporary plugins directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home directory: %v", err)
	}

	pluginsDir := filepath.Join(homeDir, ".negalog", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}

	// Create a fake plugin
	pluginPath := filepath.Join(pluginsDir, "negalog-testplugin")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("failed to create test plugin: %v", err)
	}
	defer os.Remove(pluginPath)

	// Find the plugin
	found, err := FindPlugin("testplugin")
	if err != nil {
		t.Errorf("expected to find plugin, got error: %v", err)
	}
	if found != pluginPath {
		t.Errorf("expected %s, got %s", pluginPath, found)
	}
}

func TestFormatNotFoundError_KnownPlugin(t *testing.T) {
	err := FormatNotFoundError("watch")

	if !strings.Contains(err, "watch") {
		t.Error("expected error to contain 'watch'")
	}
	if !strings.Contains(err, "available as a plugin") {
		t.Error("expected error to mention plugin availability")
	}
	if !strings.Contains(err, "negalog-watch") {
		t.Error("expected error to mention negalog-watch")
	}
}

func TestFormatNotFoundError_UnknownPlugin(t *testing.T) {
	err := FormatNotFoundError("unknown")

	if !strings.Contains(err, "unknown") {
		t.Error("expected error to contain 'unknown'")
	}
	if !strings.Contains(err, "negalog-unknown") {
		t.Error("expected error to mention negalog-unknown")
	}
	if strings.Contains(err, "available as a plugin") {
		t.Error("should not mention plugin availability for unknown plugins")
	}
}

func TestIsExecutable(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()

	// Non-executable file
	nonExec := filepath.Join(tmpDir, "nonexec")
	if err := os.WriteFile(nonExec, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if isExecutable(nonExec) {
		t.Error("non-executable file should not be detected as executable")
	}

	// Executable file
	exec := filepath.Join(tmpDir, "exec")
	if err := os.WriteFile(exec, []byte("test"), 0755); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if !isExecutable(exec) {
		t.Error("executable file should be detected as executable")
	}

	// Non-existent file
	if isExecutable(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("non-existent file should not be detected as executable")
	}
}

func TestKnownPlugins(t *testing.T) {
	// Verify watch is in known plugins
	if _, ok := KnownPlugins["watch"]; !ok {
		t.Error("expected 'watch' to be in KnownPlugins")
	}
}
