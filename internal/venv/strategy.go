package venv

import (
	"context"
	"os/exec"
	"strings"
)

type VenvCreationStrategy interface {
	handle(ctx context.Context, path string, pythonPath string) error
}

type UvVenvCreationStrategy struct{}

func (s *UvVenvCreationStrategy) handle(ctx context.Context, path string, pythonPath string) error {
	args := []string{"venv", path}
	if strings.TrimSpace(pythonPath) != "" {
		args = append(args, "--python", pythonPath)
	}

	cmd := exec.CommandContext(ctx, "uv", args...)
	return cmd.Run()
}

type PipVenvCreationStrategy struct{}

func (s *PipVenvCreationStrategy) handle(ctx context.Context, path string, pythonPath string) error {
	pythonBin := "python"
	if strings.TrimSpace(pythonPath) != "" {
		pythonBin = pythonPath
	}

	cmd := exec.CommandContext(ctx, pythonBin, "-m", "venv", path)
	return cmd.Run()
}
