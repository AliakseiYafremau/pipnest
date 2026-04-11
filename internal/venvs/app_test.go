package venvs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPreferredBaseInterpretersDedupesCanonicalDuplicates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink-based canonical duplicate test is Unix-only")
	}

	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	sbinDir := filepath.Join(tempDir, "sbin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	if err := os.MkdirAll(sbinDir, 0o755); err != nil {
		t.Fatalf("create sbin dir: %v", err)
	}

	binPython := filepath.Join(binDir, "python")
	if err := os.WriteFile(binPython, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("create python executable: %v", err)
	}

	sbinPython := filepath.Join(sbinDir, "python")
	if err := os.Symlink(binPython, sbinPython); err != nil {
		t.Fatalf("create python symlink: %v", err)
	}

	t.Setenv("PATH", stringsJoinPathList(binDir, sbinDir))

	options := preferredBaseInterpreters()
	if len(options) != 1 {
		t.Fatalf("expected 1 deduplicated interpreter, got %d (%v)", len(options), options)
	}
	if options[0].Path != binPython && options[0].Path != sbinPython {
		t.Fatalf("unexpected interpreter path %q", options[0].Path)
	}
}

func stringsJoinPathList(dirs ...string) string {
	if len(dirs) == 0 {
		return ""
	}
	path := dirs[0]
	for _, dir := range dirs[1:] {
		path += string(os.PathListSeparator) + dir
	}
	return path
}
