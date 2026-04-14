//go:build linux || darwin
// +build linux darwin

package backends

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// DefaultRunner is a shared command runner for backend implementations.
//
// It is declared as a variable so tests can temporarily replace it.
var DefaultRunner = func(ctx context.Context, binary string, args ...string) (string, error) {
	return defaultRunnerImpl(ctx, binary, args...)
}

func defaultRunnerImpl(ctx context.Context, binary string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		if trimmed == "" {
			trimmed = err.Error()
		}
		return "", fmt.Errorf("run %q with args %v: %s", binary, args, trimmed)
	}

	return trimmed, nil
}
