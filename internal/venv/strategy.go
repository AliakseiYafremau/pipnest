package venv

import (
	"context"
	"os/exec"
)

type VenvCreationStrategy interface {
	handle(ctx context.Context, path string, pythonPath string) error
}

type UvVenvCreationStrategy struct{}

func (s *UvVenvCreationStrategy) handle(ctx context.Context, path string, pythonPath string) error {
	cmd := exec.CommandContext(ctx, "uv", "venv", path, "--python", pythonPath)
	return cmd.Run()
}

type PipVenvCreationStrategy struct{}

func (s *PipVenvCreationStrategy) handle(ctx context.Context, path string, pythonPath string) error {
	cmd := exec.CommandContext(ctx, "python", "-m", "venv", path)
	return cmd.Run()
}
