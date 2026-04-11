package requirements

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetCurrentEnvironment_MatchingProject(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}

	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	configDir := filepath.Join(configRoot, "pipnest")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := "{\n" +
		"  \"projects\": {\n" +
		"    \"" + cwd + "\": {\n" +
		"      \"interpreter_path\": \"/usr/bin/python3.12\",\n" +
		"      \"kind\": \"venv\",\n" +
		"      \"updated_at\": \"2026-04-11T15:13:55Z\"\n" +
		"    }\n" +
		"  }\n" +
		"}\n"

	configPath := filepath.Join(configDir, "interpreter-selections.json")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	env, err := GetCurrentEnvironment()
	if err != nil {
		t.Fatalf("GetCurrentEnvironment returned error: %v", err)
	}

	if env.ProjectPath != cwd {
		t.Fatalf("expected project %q, got %q", cwd, env.ProjectPath)
	}
	if env.InterpreterPath != "/usr/bin/python3.12" {
		t.Fatalf("expected interpreter path, got %q", env.InterpreterPath)
	}
	if env.Kind != "venv" {
		t.Fatalf("expected kind venv, got %q", env.Kind)
	}
}

func TestGetCurrentEnvironment_FallbackToLatest(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	configDir := filepath.Join(configRoot, "pipnest")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := "{\n" +
		"  \"projects\": {\n" +
		"    \"/tmp/project-a\": {\n" +
		"      \"interpreter_path\": \"/usr/bin/python3.10\",\n" +
		"      \"kind\": \"global\",\n" +
		"      \"updated_at\": \"2026-04-10T10:00:00Z\"\n" +
		"    },\n" +
		"    \"/tmp/project-b\": {\n" +
		"      \"interpreter_path\": \"/home/user/.venv/bin/python\",\n" +
		"      \"kind\": \"venv\",\n" +
		"      \"updated_at\": \"2026-04-11T12:00:00Z\"\n" +
		"    }\n" +
		"  }\n" +
		"}\n"

	configPath := filepath.Join(configDir, "interpreter-selections.json")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	env, err := GetCurrentEnvironment()
	if err != nil {
		t.Fatalf("GetCurrentEnvironment returned error: %v", err)
	}

	if env.ProjectPath != "/tmp/project-b" {
		t.Fatalf("expected latest project /tmp/project-b, got %q", env.ProjectPath)
	}
	if env.InterpreterPath != "/home/user/.venv/bin/python" {
		t.Fatalf("unexpected interpreter path %q", env.InterpreterPath)
	}
}

func TestGetCurrentEnvironment_MissingConfig(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	_, err := GetCurrentEnvironment()
	if err == nil {
		t.Fatal("expected error when config is missing")
	}
}

func TestTouchCurrentEnvironment_UpdatesCurrentProject(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}

	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	configDir := filepath.Join(configRoot, "pipnest")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := "{\n" +
		"  \"projects\": {\n" +
		"    \"" + cwd + "\": {\n" +
		"      \"interpreter_path\": \"/usr/bin/python3\",\n" +
		"      \"kind\": \"global\",\n" +
		"      \"updated_at\": \"2026-01-01T00:00:00Z\"\n" +
		"    }\n" +
		"  }\n" +
		"}\n"

	configPath := filepath.Join(configDir, "interpreter-selections.json")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := TouchCurrentEnvironment("", ""); err != nil {
		t.Fatalf("TouchCurrentEnvironment returned error: %v", err)
	}

	payload, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var store interpreterSelectionsFile
	if err := json.Unmarshal(payload, &store); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	rec, ok := store.Projects[cwd]
	if !ok {
		t.Fatalf("expected project record for %q", cwd)
	}

	if rec.InterpreterPath != "/usr/bin/python3" {
		t.Fatalf("expected interpreter unchanged, got %q", rec.InterpreterPath)
	}
	if rec.Kind != "global" {
		t.Fatalf("expected kind unchanged, got %q", rec.Kind)
	}
	if strings.TrimSpace(rec.UpdatedAt) == "" {
		t.Fatal("expected updated_at to be set")
	}
	if _, err := time.Parse(time.RFC3339, rec.UpdatedAt); err != nil {
		t.Fatalf("expected RFC3339 updated_at, got %q", rec.UpdatedAt)
	}
}
